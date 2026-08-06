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
	"time"

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

	ctx context.Context
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

// ======================== TASK RECORD ACTIONS ====================================

func (t *TaskService) StartTaskRecord(taskId string, padagan bool) (*db.TaskRecord, error) {
	if t.Attendance == nil || *t.Attendance == nil {
		return nil, fmt.Errorf("StartTaskRecord error: Attendance is nil")
	}

	userId := (*t.Attendance).UserID
	attendanceId := (*t.Attendance).ID

	if taskRecord, err := t.TaskRepository.CheckTaskRecord(userId, taskId); err == nil && taskRecord != nil {
		if err := t.updateRecord(taskRecord); err != nil {
			return nil, types.WrapOperation("task.start.update_existing_record", taskRecord.ID, userId, err)
		}
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, types.WrapOperation("task.start.check_existing_record", 0, userId, err)
	}

	record, err := t.newTaskRecord(userId, taskId, attendanceId, padagan)
	if err != nil {
		return nil, types.WrapOperation("task.start.create_record", 0, userId, err)
	}

	record.Attendance = *(*t.Attendance)
	record.User = (*t.Attendance).User

	if err := t.SyncService.SyncTaskRecord(record.ID); err != nil {
		t.Logger.Printlnf("error syncing task_record: %v", err)
	}

	return record, nil
}

func (t *TaskService) StopTaskRecord(userId, taskId string) error {
	var taskRecord models.TaskRecord

	err := db.DB.Where("user_id = ? AND task_id = ? AND ended_at IS NULL", userId, taskId).
		First(&taskRecord).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		fmt.Printf("no active task found for user %s task %s", userId, taskId)
	}
	if err != nil {
		return err
	}

	// ✅ only call UpdateRecord if we really have a record
	if taskRecord.ID == 0 {
		return fmt.Errorf("invalid taskRecord, no ID")
	}

	if err := t.TaskRepository.UpdateRecord(&taskRecord); err != nil {
		return fmt.Errorf("failed to update record: %w", err)
	}

	fmt.Println("STOPPED TASK RECORD")
	return nil
}

func (t *TaskService) StartTracking(taskId, localStorageValue string, padagan bool) error {
	if t.Attendance == nil || *t.Attendance == nil {
		err := fmt.Errorf("StartTracking error: Attendance is nil")
		t.Logger.Println(err)
		return err
	}

	// if t.ScreenshotService.Running {
	// 	t.ScreenshotService.Stop()
	// }
	if err := t.TaskRepository.StopAllActiveRecords(*t.Attendance); err != nil {
		t.Logger.Printf("error stopping active task record: %v", types.WrapOperation("task.start.stop_all_active_records", 0, (*t.Attendance).UserID, err))
	}

	if t.ScreenshotService.TaskRecordID != nil && t.WorkingTaskID != nil {
		record, err := t.GetTaskRecord(*t.ScreenshotService.TaskRecordID)

		if err != nil {
			fmt.Printf("Error on syncing record: %s", err)
		} else {
			if err := t.SyncService.SyncTaskRecord(record.ID); err != nil {
				t.Logger.Printlnf("error syncing task_record: %v", err)
			}
		}

	}

	record, err := t.StartTaskRecord(taskId, padagan)
	if err != nil {
		t.Logger.Printf("error starting task record: %v", err)
		return err
	}
	if record == nil {
		err := fmt.Errorf("error starting task record: record is nil")
		t.Logger.Printf("%v", err)
		return err
	}

	t.ScreenshotService.FolderName = localStorageValue
	t.ScreenshotService.TaskID = &taskId

	if t.WorkingTaskID != nil {
		if *t.WorkingTaskID != nil {
			// Update the value at the existing address
			**t.WorkingTaskID = taskId
		} else {
			// The outer pointer exists, but it points to nil
			// Assign a new pointer to the inner location
			*t.WorkingTaskID = &taskId
		}
	}

	t.TaskRecord = record

	if t.WorkingTaskID != nil && *t.WorkingTaskID != nil {
		t.ScreenshotService.TaskID = *t.WorkingTaskID
	}
	t.ScreenshotService.TaskRecordID = &record.ID

	task, err := t.TaskRepository.GetTask(taskId)
	if err != nil {
		return err
	}

	if task.ProjectName == "Break" {
		t.SyncService.SyncTaskRecord(record.ID)
		return nil
	}

	t.ScreenshotService.Start(t.ctx)
	return nil
}

// ActivateBrowserTaskRecord adopts an already-imported Bubble productivity
// record as the desktop's active task. It deliberately does not call
// StartTaskRecord or SyncTaskRecord for the adopted record: Bubble has already
// created its remote productivity ID, so creating another task-in would
// duplicate time tracking remotely.
func (t *TaskService) ActivateBrowserTaskRecord(taskRecordID uint) error {
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
	if t.Attendance == nil || *t.Attendance == nil {
		return nil, fmt.Errorf("StopTracking error: Attendance is nil")
	}
	userId := (*t.Attendance).UserID

	clearActiveTaskCache := func() {
		if t.WorkingTaskID != nil {
			*t.WorkingTaskID = nil
		}
		t.ScreenshotService.TaskID = nil
		t.ScreenshotService.TaskRecordID = nil
		t.TaskRecord = nil
	}

	t.ScreenshotService.Stop()
	err := t.StopTaskRecord(
		userId,
		taskId,
	)
	if err != nil {
		t.Logger.Println("StopTaskRecord error:", err)
		clearActiveTaskCache()
		return nil, err
	}

	taskRecord, err := t.TaskRepository.GetLastTaskRecord(userId, taskId)
	t.TaskRecord = taskRecord
	if err != nil {
		t.Logger.Printf("error retrieving task record: %v\n", err)
		clearActiveTaskCache()
		return nil, err
	}
	if taskRecord == nil {
		t.Logger.Printf("error retrieving task record: no record found for user %s task %s\n", userId, taskId)
		clearActiveTaskCache()
		return nil, fmt.Errorf("no stopped task record found for user %s task %s", userId, taskId)
	}

	if err := db.DB.Model(taskRecord).Updates(map[string]any{
		"sync_status": syncStatePending,
		"last_error":  "",
		"synced_at":   nil,
	}).Error; err != nil {
		t.Logger.Printf("error updating sync status: %v", types.WrapOperation("task.stop.mark_pending", taskRecord.ID, userId, err))
		clearActiveTaskCache()
		return nil, err
	}

	result := &StopTrackingResult{
		RecordID:   taskRecord.ID,
		SyncStatus: syncStatePending,
	}

	// Syncing TaskRecord to the active production sync service.
	if t.Connectivity.IsOnline() {
		t.Logger.Print("Attempting to sync record")
		t.Logger.Printf("Syncing record: %v\n", taskRecord)
		if err := t.SyncService.SyncTaskRecord(taskRecord.ID); err != nil {
			t.Logger.Printf("error syncing task_record: %v", err)
			result.SyncFailureCode = taskSyncFailureCode(err)
		}
	} else {
		result.SyncFailureCode = "offline"
	}

	if refreshed, err := t.TaskRepository.GetTaskRecord(taskRecord.ID); err != nil {
		t.Logger.Printf("error refreshing stopped task record: %v", err)
	} else if refreshed != nil {
		result.SyncStatus = refreshed.SyncStatus
	}

	t.TaskRecord = nil
	clearActiveTaskCache()
	return result, nil
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
		UserID:       userId,
		AttendanceID: attendanceId,
		TaskID:       taskId,
		Padagan:      padagan,
		StartedAt:    now,
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
