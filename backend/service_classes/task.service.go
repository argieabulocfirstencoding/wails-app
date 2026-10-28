package service_classes

import (
	"context"
	"errors"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/repository"
	"fesapp/backend/types"
	"fesapp/backend/utils"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

type TaskService struct {
	TaskRepository    *repository.TaskRepository
	ScreenshotService *ScreenshotService
	SyncService       *SyncService
	Logger            *utils.LoggerService
	WorkingTaskID     **string
	Attendance        **models.Attendance
	TaskRecord        *models.TaskRecord

	AuthRepository *repository.AuthRepository
	Connectivity   *ConnectivityStateService

	ctx         context.Context
	lifecycleMu sync.Mutex
}

// StopTrackingResult distinguishes a durable local stop from the later remote
// sync outcome. A task must remain stopped locally even if Bubble is offline
// or rejects the current session token.
type StopTrackingResult struct {
	RecordID        uint   `json:"recordId"`
	SyncStatus      string `json:"syncStatus"`
	SyncFailureCode string `json:"syncFailureCode,omitempty"`
}

func NewTaskService(SyncService *SyncService, connectivity ...*ConnectivityStateService) *TaskService {
	var connectivityState *ConnectivityStateService
	if len(connectivity) > 0 {
		connectivityState = connectivity[0]
	}
	if connectivityState == nil {
		connectivityState = NewConnectivityStateService()
	}
	screenshotService := NewScreenshotService(connectivityState)
	screenshotService.SyncService = SyncService

	return &TaskService{
		TaskRepository:    repository.NewTaskRepository(),
		ScreenshotService: screenshotService,
		SyncService:       SyncService,
		AuthRepository:    repository.NewAuthRepository(),
		Connectivity:      connectivityState,
		Logger:            utils.NewLoggerService(),
	}
}

func (t *TaskService) Connect(ctx context.Context) {
	t.ctx = ctx
	t.SyncService.Connect(ctx)
}

func (t *TaskService) CreateTask(title, client string) (*models.ExpTask, error) {
	return nil, nil
}

func (t *TaskService) GetTask(id string) (*models.ExpTask, error) {
	return nil, nil
}

func (t *TaskService) GetTasks(isOnline bool) ([]models.ExpTask, error) {
	if t.Connectivity.IsOnline() && isOnline {
		if err := t.SyncService.FesClientService.SyncClientsAndProjects(); err != nil {
			t.Logger.Printf("error fetching tasks from fes: %v", err)
			if t.ctx != nil {
				runtime.EventsEmit(t.ctx, "tasks-sync-warning", err.Error())
			}
		}
	}

	if t.Attendance == nil {
		return nil, fmt.Errorf("GetTasks error: AuthUser is nil")
	}

	tasks, err := t.TaskRepository.GetTasks()
	if err != nil {
		return nil, err
	}

	// FIX REFACTOR this
	expTasks := models.ToExpTasks(tasks)
	return expTasks, nil
}

func (t *TaskService) UpdateTask(id string, title, client string) (*models.ExpTask, error) {
	return nil, nil
}

func (t *TaskService) DeleteTask(id string) error {
	return nil
}

// ======================== TASK RECORD METHODS ====================================

func (t *TaskService) GetTaskRecord(id uint) (*models.TaskRecord, error) {
	return t.TaskRepository.GetTaskRecord(id)
}

func (t *TaskService) GetLastTaskRecords() ([]models.ExpTaskRecord, error) {
	if t.Attendance == nil {
		return nil, fmt.Errorf("GetLastTaskRecords error: AuthUser is nil")
	}

	task_records, err := t.TaskRepository.GetAllPreviousTasksRecords((*t.Attendance).UserID)
	if err != nil {
		return nil, err
	}

	var exp_task_records []models.ExpTaskRecord

	for _, record := range task_records {
		exp_record := record.ToExp()
		exp_task_records = append(exp_task_records, exp_record)
	}

	return exp_task_records, nil
}

func (t *TaskService) StartTracking(taskId, localStorageValue string, padagan bool) error {
	t.lifecycleMu.Lock()
	defer t.lifecycleMu.Unlock()

	if t.Attendance == nil || *t.Attendance == nil {
		err := fmt.Errorf("StartTracking error: Attendance is nil")
		t.Logger.Println(err)
		return err
	}

	task, err := t.TaskRepository.GetTask(taskId)
	if err != nil {
		return err
	}

	// Stop capture before changing the active database record so screenshots
	// cannot be attached to a record that has just been ended.
	t.ScreenshotService.Stop()
	record, reused, stoppedRecord, err := t.beginTaskRecord(taskId, padagan)
	if err != nil {
		t.Logger.Printf("error starting task record: %v", err)
		return err
	}
	if record == nil {
		err := fmt.Errorf("error starting task record: record is nil")
		t.Logger.Printf("%v", err)
		return err
	}

	t.activateDesktopTaskRecord(record, localStorageValue, task.ProjectName)
	if reused {
		return nil
	}

	if stoppedRecord != nil {
		if err := t.SyncService.SyncTaskRecord(stoppedRecord.ID); err != nil {
			t.Logger.Printlnf("error syncing stopped task record: %v", err)
		}
	}
	if err := t.SyncService.SyncTaskRecord(record.ID); err != nil {
		t.Logger.Printlnf("error syncing task_record: %v", err)
	}
	return nil
}

// beginTaskRecord performs the local transition atomically. The lifecycle
// mutex held by StartTracking prevents another desktop request from observing
// the gap between this transaction and the following cache/sync work.
func (t *TaskService) beginTaskRecord(taskID string, padagan bool) (record *models.TaskRecord, reused bool, stopped *models.TaskRecord, err error) {
	attendance := *t.Attendance
	err = db.DB.Transaction(func(tx *gorm.DB) error {
		var active []models.TaskRecord
		if err := tx.
			Where("user_id = ? AND attendance_id = ? AND ended_at IS NULL", attendance.UserID, attendance.ID).
			Order("started_at ASC, id ASC").
			Find(&active).Error; err != nil {
			return err
		}
		if len(active) > 1 {
			return fmt.Errorf("start tracking: found %d active task records for the current attendance", len(active))
		}
		if len(active) == 1 && active[0].TaskID == taskID {
			record = &active[0]
			reused = true
			return nil
		}

		now := time.Now()
		if len(active) == 1 {
			previous := active[0]
			duration := int32(now.Sub(previous.StartedAt).Seconds())
			if duration < 0 {
				duration = 0
			}
			if err := tx.Model(&models.TaskRecord{}).
				Where("id = ? AND ended_at IS NULL", previous.ID).
				Updates(map[string]any{
					"ended_at":    now,
					"duration":    duration,
					"sync_status": gorm.Expr("CASE WHEN sync_status IN ('identity_conflict', 'remote_missing') THEN sync_status ELSE 'pending' END"),
					"last_error":  "",
					"synced_at":   nil,
				}).Error; err != nil {
				return err
			}
			previous.EndedAt = &now
			previous.Duration = duration
			previous.SyncStatus = syncStatePending
			previous.LastError = ""
			previous.SyncedAt = nil
			stopped = &previous
		}

		gap := int32(0)
		var previous models.TaskRecord
		if err := tx.
			Where("user_id = ? AND attendance_id = ? AND ended_at IS NOT NULL", attendance.UserID, attendance.ID).
			Order("ended_at DESC, id DESC").
			First(&previous).Error; err == nil && previous.EndedAt != nil {
			gap = int32(now.Sub(*previous.EndedAt).Seconds())
			if gap < 0 {
				gap = 0
			}
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		record = &models.TaskRecord{
			ClientSessionID: models.StringPtr(uuid.NewString()),
			UserID:          attendance.UserID,
			AttendanceID:    attendance.ID,
			TaskID:          taskID,
			Padagan:         padagan,
			StartedAt:       now,
			Gap:             gap,
		}
		return tx.Create(record).Error
	})
	return record, reused, stopped, err
}

func (t *TaskService) activateDesktopTaskRecord(record *models.TaskRecord, folderName, projectName string) {
	if record == nil {
		return
	}
	taskID := record.TaskID
	if folderName == "" {
		folderName = fmt.Sprintf("%d", record.StartedAt.UnixMilli())
	}
	t.ScreenshotService.FolderName = folderName
	t.ScreenshotService.TaskID = &taskID
	t.ScreenshotService.TaskRecordID = &record.ID
	t.TaskRecord = record
	if t.WorkingTaskID != nil {
		*t.WorkingTaskID = &taskID
	}
	if projectName != "Break" {
		t.ScreenshotService.Start(t.ctx)
	}
}

func (t *TaskService) clearActiveTaskCacheForRecord(taskRecordID uint, taskID string) {
	if taskRecordID == 0 {
		return
	}
	if t.ScreenshotService.TaskRecordID != nil && *t.ScreenshotService.TaskRecordID == taskRecordID {
		t.clearActiveTaskCacheForTask(taskID)
		return
	}
	if t.TaskRecord != nil && t.TaskRecord.ID == taskRecordID {
		t.clearActiveTaskCacheForTask(taskID)
		return
	}
	if t.WorkingTaskID == nil || *t.WorkingTaskID == nil || **t.WorkingTaskID != taskID {
		return
	}
	t.clearActiveTaskCacheForTask(taskID)
}

func (t *TaskService) clearActiveTaskCacheForTask(taskID string) {
	if taskID == "" {
		return
	}
	matchesTask := false
	if t.WorkingTaskID != nil && *t.WorkingTaskID != nil && **t.WorkingTaskID == taskID {
		matchesTask = true
	}
	if t.ScreenshotService.TaskID != nil && *t.ScreenshotService.TaskID == taskID {
		matchesTask = true
	}
	if t.TaskRecord != nil && t.TaskRecord.TaskID == taskID {
		matchesTask = true
	}
	if !matchesTask {
		return
	}
	t.ScreenshotService.Stop()
	if t.WorkingTaskID != nil {
		*t.WorkingTaskID = nil
	}
	t.ScreenshotService.TaskID = nil
	t.ScreenshotService.TaskRecordID = nil
	t.TaskRecord = nil
}

// ActivateBrowserTaskRecord adopts an already-imported Bubble productivity
// record as the desktop's active task. It deliberately does not call
// StartTaskRecord or SyncTaskRecord for the adopted record: Bubble has already
// created its remote productivity ID, so creating another task-in would
// duplicate time tracking remotely.
func (t *TaskService) ActivateBrowserTaskRecord(taskRecordID uint) error {
	t.lifecycleMu.Lock()
	defer t.lifecycleMu.Unlock()
	return t.activateBrowserTaskRecordLocked(taskRecordID)
}

func (t *TaskService) activateBrowserTaskRecordLocked(taskRecordID uint) error {
	if t == nil || t.Attendance == nil || *t.Attendance == nil {
		return fmt.Errorf("activate browser task: attendance is nil")
	}
	if t.TaskRepository == nil || t.ScreenshotService == nil {
		return fmt.Errorf("activate browser task: task service is unavailable")
	}

	record, err := t.TaskRepository.GetTaskRecord(taskRecordID)
	if err != nil {
		return fmt.Errorf("activate browser task: load task record: %w", err)
	}
	attendance := *t.Attendance
	if record.UserID != attendance.UserID || record.AttendanceID != attendance.ID {
		return fmt.Errorf("activate browser task: task record does not belong to the current attendance")
	}
	if record.EndedAt != nil {
		return nil
	}
	if !taskRecordHasRemoteProductivity(record) {
		return fmt.Errorf("activate browser task: task record has no productivity ID")
	}

	task, err := t.TaskRepository.GetTask(record.TaskID)
	if err != nil {
		return fmt.Errorf("activate browser task: load task: %w", err)
	}

	// A browser task-in takes over from any other local active task. End only
	// the conflicting records, never the imported target record.
	var conflicting []models.TaskRecord
	if err := db.DB.
		Where("user_id = ? AND attendance_id = ? AND ended_at IS NULL AND id <> ?", attendance.UserID, attendance.ID, record.ID).
		Find(&conflicting).Error; err != nil {
		return fmt.Errorf("activate browser task: find conflicting active records: %w", err)
	}

	if len(conflicting) > 0 {
		if t.ScreenshotService.TaskRecordID != nil && *t.ScreenshotService.TaskRecordID != record.ID {
			t.ScreenshotService.Stop()
		}

		if err := db.DB.Transaction(func(tx *gorm.DB) error {
			now := time.Now()
			for i := range conflicting {
				endAt := now
				if record.StartedAt.After(conflicting[i].StartedAt) && record.StartedAt.Before(now) {
					endAt = record.StartedAt
				}
				conflicting[i].EndedAt = &endAt
				conflicting[i].Duration = int32(endAt.Sub(conflicting[i].StartedAt).Seconds())
				if conflicting[i].Duration < 0 {
					conflicting[i].Duration = 0
				}
				conflicting[i].SyncStatus = syncStatePending
				conflicting[i].LastError = ""
				conflicting[i].SyncedAt = nil
				if err := tx.Save(&conflicting[i]).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return fmt.Errorf("activate browser task: stop conflicting active records: %w", err)
		}
	}

	taskID := record.TaskID
	t.ScreenshotService.FolderName = fmt.Sprintf("%d", record.StartedAt.UnixMilli())
	t.ScreenshotService.TaskID = &taskID
	t.ScreenshotService.TaskRecordID = &record.ID
	t.TaskRecord = record
	if t.WorkingTaskID != nil {
		*t.WorkingTaskID = &taskID
	}

	if task.ProjectName != "Break" {
		t.ScreenshotService.Start(t.ctx)
	}
	return nil
}

func (t *TaskService) StopTracking(taskId string) (*StopTrackingResult, error) {
	t.lifecycleMu.Lock()
	defer t.lifecycleMu.Unlock()

	if t.Attendance == nil || *t.Attendance == nil {
		return nil, fmt.Errorf("StopTracking error: Attendance is nil")
	}
	attendance := *t.Attendance
	var record models.TaskRecord
	err := db.DB.
		Where("user_id = ? AND attendance_id = ? AND task_id = ?", attendance.UserID, attendance.ID, taskId).
		Order("started_at DESC, id DESC").
		First(&record).Error
	if err != nil {
		t.clearActiveTaskCacheForTask(taskId)
		return nil, err
	}
	return t.stopTrackingRecordLocked(record.ID, time.Time{})
}

// StopTrackingRecord stops one known session. Calling it again for a session
// that is already stopped returns its saved state without changing its end time
// or sending another task-out request.
func (t *TaskService) StopTrackingRecord(taskRecordID uint) (*StopTrackingResult, error) {
	t.lifecycleMu.Lock()
	defer t.lifecycleMu.Unlock()
	return t.stopTrackingRecordLocked(taskRecordID, time.Time{})
}

// StopTrackingRecordAt closes a task at an employee-confirmed time after an
// interrupted session. The time is validated locally before any durable write.
func (t *TaskService) StopTrackingRecordAt(taskRecordID uint, endedAt time.Time) (*StopTrackingResult, error) {
	t.lifecycleMu.Lock()
	defer t.lifecycleMu.Unlock()
	return t.stopTrackingRecordLocked(taskRecordID, endedAt)
}

// ResolveStaleTaskRecordAt closes a record from an older attendance after an
// employee confirms its task-out time. Unlike normal StopTracking, it does not
// affect the current attendance or attempt an immediate remote sync.
func (t *TaskService) ResolveStaleTaskRecordAt(taskRecordID uint, endedAt time.Time) (*StopTrackingResult, error) {
	t.lifecycleMu.Lock()
	defer t.lifecycleMu.Unlock()

	if t.Attendance == nil || *t.Attendance == nil {
		return nil, fmt.Errorf("resolve stale task record: Attendance is nil")
	}
	attendance := *t.Attendance
	taskRecord, err := t.TaskRepository.GetTaskRecord(taskRecordID)
	if err != nil {
		return nil, err
	}
	if taskRecord.UserID != attendance.UserID {
		return nil, fmt.Errorf("resolve stale task record: task record does not belong to the current user")
	}
	if taskRecord.AttendanceID == attendance.ID {
		return nil, fmt.Errorf("resolve stale task record: task record belongs to the current attendance")
	}

	return t.closeTaskRecordLocked(taskRecord, endedAt, false)
}

func (t *TaskService) stopTrackingRecordLocked(taskRecordID uint, requestedEnd time.Time) (*StopTrackingResult, error) {
	if t.Attendance == nil || *t.Attendance == nil {
		return nil, fmt.Errorf("StopTracking error: Attendance is nil")
	}
	attendance := *t.Attendance
	taskRecord, err := t.TaskRepository.GetTaskRecord(taskRecordID)
	if err != nil {
		return nil, err
	}
	if taskRecord.UserID != attendance.UserID || taskRecord.AttendanceID != attendance.ID {
		return nil, fmt.Errorf("stop tracking: task record does not belong to the current attendance")
	}
	if taskRecord.EndedAt != nil {
		return &StopTrackingResult{RecordID: taskRecord.ID, SyncStatus: taskRecord.SyncStatus}, nil
	}

	return t.closeTaskRecordLocked(taskRecord, requestedEnd, true)
}

func (t *TaskService) closeTaskRecordLocked(taskRecord *models.TaskRecord, requestedEnd time.Time, syncImmediately bool) (*StopTrackingResult, error) {
	if taskRecord == nil {
		return nil, fmt.Errorf("stop tracking: task record is nil")
	}
	if taskRecord.EndedAt != nil {
		return &StopTrackingResult{RecordID: taskRecord.ID, SyncStatus: taskRecord.SyncStatus}, nil
	}

	if t.ScreenshotService != nil && t.ScreenshotService.TaskRecordID != nil && *t.ScreenshotService.TaskRecordID == taskRecord.ID {
		t.ScreenshotService.Stop()
	}

	now := requestedEnd
	if now.IsZero() {
		now = time.Now()
	}
	if now.Before(taskRecord.StartedAt) || now.After(time.Now().Add(time.Minute)) {
		return nil, fmt.Errorf("stop tracking: end time must be between task start and now")
	}
	duration := int32(now.Sub(taskRecord.StartedAt).Seconds())
	if duration < 0 {
		duration = 0
	}
	result := db.DB.Model(&models.TaskRecord{}).
		Where("id = ? AND ended_at IS NULL", taskRecord.ID).
		Updates(map[string]any{
			"ended_at":    now,
			"duration":    duration,
			"sync_status": gorm.Expr("CASE WHEN sync_status IN ('identity_conflict', 'remote_missing') THEN sync_status ELSE 'pending' END"),
			"last_error":  "",
			"synced_at":   nil,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		refreshed, err := t.TaskRepository.GetTaskRecord(taskRecord.ID)
		if err != nil {
			return nil, err
		}
		if refreshed.EndedAt != nil {
			return &StopTrackingResult{RecordID: refreshed.ID, SyncStatus: refreshed.SyncStatus}, nil
		}
		return nil, fmt.Errorf("stop tracking: active record changed before it could be stopped")
	}

	taskRecord.EndedAt = &now
	taskRecord.Duration = duration
	taskRecord.SyncStatus = syncStatePending
	taskRecord.LastError = ""
	taskRecord.SyncedAt = nil
	stopResult := &StopTrackingResult{RecordID: taskRecord.ID, SyncStatus: syncStatePending}

	if syncImmediately && t.Connectivity.IsOnline() {
		if err := t.SyncService.SyncTaskRecord(taskRecord.ID); err != nil {
			t.Logger.Printf("error syncing task_record: %v", err)
			stopResult.SyncFailureCode = taskSyncFailureCode(err)
		}
	} else if syncImmediately {
		stopResult.SyncFailureCode = "offline"
	}

	if refreshed, err := t.TaskRepository.GetTaskRecord(taskRecord.ID); err != nil {
		t.Logger.Printf("error refreshing stopped task record: %v", err)
	} else if refreshed != nil {
		stopResult.SyncStatus = refreshed.SyncStatus
	}
	if syncImmediately {
		t.clearActiveTaskCacheForRecord(taskRecord.ID, taskRecord.TaskID)
	}
	return stopResult, nil
}

func taskSyncFailureCode(err error) string {
	var stableErr *types.StableError
	if errors.As(err, &stableErr) && stableErr.Code == "SESSION_REAUTH_REQUIRED" {
		return "session_expired"
	}
	var remoteErr *types.RemoteAPIError
	if errors.As(err, &remoteErr) && remoteErr.StatusCode == 401 {
		return "session_expired"
	}
	return "sync_failed"
}

func (t *TaskService) StopAllActiveRecords() error {
	t.lifecycleMu.Lock()
	defer t.lifecycleMu.Unlock()
	if t.Attendance == nil || *t.Attendance == nil {
		return fmt.Errorf("stop active records: attendance is nil")
	}
	attendance := (*t.Attendance)

	return t.TaskRepository.StopAllActiveRecords(attendance)
}

func (t *TaskService) GetLastTaskRecord(userId, taskId string) (*models.TaskRecord, error) {
	return nil, nil
}

func (t *TaskService) SetTaskRecordNotes(taskRecordId uint, notes string) {
	if taskRecord, err := t.GetTaskRecord(taskRecordId); err != nil {
		t.Logger.Println(err)
	} else {
		taskRecord.Notes = notes
		if err := t.TaskRepository.UpdateTaskRecord(taskRecord, taskRecord); err != nil {
			t.Logger.Printf("error updating task record notes: %v", types.WrapOperation("task.set_notes", taskRecordId, taskRecord.UserID, err))
			return
		}

		if t.TaskRecord == nil {
			t.TaskRecord = taskRecord
		} else {
			(*t.TaskRecord).Notes = notes
		}
	}

}

func (t *TaskService) GetActiveTaskRecord(userId string) (*models.ExpTaskRecord, error) {
	attendance, err := t.AuthRepository.GetUserAttendance(userId)
	if err != nil {
		return nil, err
	}

	taskRecord, err := t.TaskRepository.GetActiveTaskRecord(userId, attendance.ID)
	if err != nil {
		return nil, err
	}

	expTaskRecord := taskRecord.ToExp()
	return &expTaskRecord, nil
}

//  ============= PRIVATE METHODS =========================

func (t *TaskService) newTaskRecord(userId, taskId string, attendanceId uint, padagan bool) (*models.TaskRecord, error) {
	now := time.Now()

	record := models.TaskRecord{
		ClientSessionID: models.StringPtr(uuid.NewString()),
		UserID:          userId,
		AttendanceID:    attendanceId,
		TaskID:          taskId,
		Padagan:         padagan,
		StartedAt:       now,
	}

	if err := t.calculateGap(&record, now); err != nil {
		return nil, err
	}

	if err := t.TaskRepository.CreateTaskRecord(&record); err != nil {
		return nil, err
	}

	return &record, nil
}

func (t *TaskService) updateRecord(tr *models.TaskRecord) error {

	now := time.Now()
	tr.EndedAt = &now

	if !tr.StartedAt.IsZero() {
		duration := int32(now.Sub(tr.StartedAt).Seconds())
		tr.Duration = duration
	}

	taskRecord := models.TaskRecord{
		EndedAt:  tr.EndedAt,
		Duration: tr.Duration,
	}

	// This part updates the database
	return t.TaskRepository.UpdateTaskRecord(tr, &taskRecord)
}

func (t *TaskService) calculateGap(tr *models.TaskRecord, now time.Time) error {
	taskRecord, err := t.TaskRepository.GetLastTaskRecordFromAttendance(tr.UserID, tr.AttendanceID)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tr.Gap = 0
			return nil
		}
		return err
	}

	// Defensive checks (VERY important)
	if taskRecord == nil || taskRecord.EndedAt == nil {
		tr.Gap = 0
		return nil
	}

	gap := now.Sub(*taskRecord.EndedAt)
	tr.Gap = int32(gap / time.Second)

	return nil
}
