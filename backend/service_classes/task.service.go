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

func NewTaskService(SyncService *SyncService, connectivity ...*ConnectivityStateService) *TaskService {
	var connectivityState *ConnectivityStateService
	if len(connectivity) > 0 {
		connectivityState = connectivity[0]
	}
	if connectivityState == nil {
		connectivityState = NewConnectivityStateService()
	}
	screenshotService := NewScreenshotService(connectivityState)

	return &TaskService{
		TaskRepository:    repository.NewTaskRepository(),
		ScreenshotService: screenshotService,
		SyncService:       SyncService,
		Connectivity:      connectivityState,
		AuthRepository:    repository.NewAuthRepository(),
		Logger:            utils.NewLoggerService(),
	}
}

func (t *TaskService) Connect(ctx context.Context) {
	t.ctx = ctx
	if t.SyncService != nil {
		t.SyncService.Connect(ctx)
	}
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
		}
	}

	if t.Attendance == nil || *t.Attendance == nil {
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
	if t.Attendance == nil || *t.Attendance == nil {
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
	t.Logger.Infof("op=task_start_record_local_start user_id=%s attendance_id=%d task_id=%s padagan=%t", userId, attendanceId, taskId, padagan)

	if taskRecord, err := t.TaskRepository.CheckTaskRecordForAttendance(userId, attendanceId, taskId); err == nil && taskRecord != nil {
		t.Logger.Infof("op=task_start_record_existing_active user_id=%s attendance_id=%d task_id=%s record_id=%d", userId, attendanceId, taskId, taskRecord.ID)
		if err := t.updateRecord(taskRecord); err != nil {
			t.Logger.Warnf("op=task_start_record_existing_stop_failed user_id=%s attendance_id=%d task_id=%s record_id=%d err=%v", userId, attendanceId, taskId, taskRecord.ID, err)
			return nil, types.WrapOperation("task.start.update_existing_record", taskRecord.ID, userId, err)
		}
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Logger.Warnf("op=task_start_record_check_failed user_id=%s attendance_id=%d task_id=%s err=%v", userId, attendanceId, taskId, err)
		return nil, types.WrapOperation("task.start.check_existing_record", 0, userId, err)
	}

	record, err := t.newTaskRecord(userId, taskId, attendanceId, padagan)
	if err != nil {
		t.Logger.Warnf("op=task_start_record_local_failed user_id=%s attendance_id=%d task_id=%s err=%v", userId, attendanceId, taskId, err)
		return nil, types.WrapOperation("task.start.create_record", 0, userId, err)
	}

	record.Attendance = *(*t.Attendance)
	record.User = (*t.Attendance).User
	t.Logger.Infof("op=task_start_record_local_success user_id=%s attendance_id=%d task_id=%s record_id=%d started_at=%s sync_status=%s", userId, attendanceId, taskId, record.ID, record.StartedAt.Format(time.RFC3339), record.SyncStatus)

	t.syncTaskRecordAsync(record.ID)

	return record, nil
}

func (t *TaskService) StopTaskRecord(userId, taskId string) error {
	var taskRecord models.TaskRecord
	if t.Attendance == nil || *t.Attendance == nil {
		return fmt.Errorf("stop task record failed: attendance is nil")
	}

	err := db.DB.Where("user_id = ? AND attendance_id = ? AND task_id = ? AND ended_at IS NULL", userId, (*t.Attendance).ID, taskId).
		First(&taskRecord).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Stop is idempotent: no active local record is not a hard failure.
		t.Logger.Printf("StopTaskRecord: no active task found for user %s attendance %d task %s", userId, (*t.Attendance).ID, taskId)
		return nil
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
		err := fmt.Errorf("start tracking failed: attendance is nil")
		t.Logger.Println(err.Error())
		return err
	}
	_ = localStorageValue
	t.Logger.Infof("op=task_tracking_start_requested user_id=%s attendance_id=%d task_id=%s padagan=%t", (*t.Attendance).UserID, (*t.Attendance).ID, taskId, padagan)

	task, err := t.TaskRepository.GetTask(taskId)
	if err != nil {
		wrapped := types.WrapOperation("task.start.get_task", 0, (*t.Attendance).UserID, err)
		t.Logger.Warnf("op=task_tracking_start_failed user_id=%s attendance_id=%d task_id=%s err=%v", (*t.Attendance).UserID, (*t.Attendance).ID, taskId, wrapped)
		return wrapped
	}

	if err := t.TaskRepository.StopAllActiveRecords(*t.Attendance); err != nil {
		wrapped := types.WrapOperation("task.start.stop_all_active_records", 0, (*t.Attendance).UserID, err)
		t.Logger.Printf("error stopping active task record: %v", wrapped)
		return wrapped
	}
	t.Logger.Infof("op=task_tracking_start_previous_records_closed user_id=%s attendance_id=%d task_id=%s", (*t.Attendance).UserID, (*t.Attendance).ID, taskId)

	record, err := t.StartTaskRecord(taskId, padagan)
	if err != nil {
		t.Logger.Printf("error starting task record: %v", err)
		return err
	}
	if record == nil {
		err := fmt.Errorf("start tracking failed: task record is nil")
		t.Logger.Println(err.Error())
		return err
	}

	t.TaskRecord = record

	if t.WorkingTaskID != nil {
		if *t.WorkingTaskID != nil {
			**t.WorkingTaskID = taskId
		} else {
			*t.WorkingTaskID = &taskId
		}
	}

	if task.ProjectName == "Break" {
		return nil
	}

	t.Logger.Infof("op=task_tracking_start_success user_id=%s attendance_id=%d task_id=%s record_id=%d", (*t.Attendance).UserID, (*t.Attendance).ID, taskId, record.ID)
	return nil
}

func (t *TaskService) StopTracking(taskId string) error {
	if t.Attendance == nil || *t.Attendance == nil {
		t.Logger.Println("StopTracking error: Attendance is nil")
		return fmt.Errorf("stop tracking failed: attendance is nil")
	}
	userId := (*t.Attendance).UserID
	t.Logger.Infof("op=task_tracking_stop_requested user_id=%s attendance_id=%d task_id=%s", userId, (*t.Attendance).ID, taskId)

	err := t.stopCurrentTaskRecord(userId, taskId)
	if err != nil {
		t.Logger.Println("StopTaskRecord error:", err)
		t.Logger.Warnf("op=task_tracking_stop_failed user_id=%s attendance_id=%d task_id=%s err=%v", userId, (*t.Attendance).ID, taskId, err)
		return err
	}
	t.Logger.Infof("op=task_tracking_stop_local_success user_id=%s attendance_id=%d task_id=%s", userId, (*t.Attendance).ID, taskId)

	if t.WorkingTaskID != nil {
		*t.WorkingTaskID = nil
	}

	// Syncing TaskRecord to Firebase
	if t.Connectivity.IsOnline() {
		// Retrieving TaskRecord from TaskRecordID
		t.Logger.Print("Attempting to sync record")
		taskRecord, err := t.TaskRepository.GetLastTaskRecordForAttendance(userId, (*t.Attendance).ID, taskId)
		t.TaskRecord = taskRecord

		if err != nil {
			t.Logger.Printf("error retrieving task record: %v\n", err)
		}
		if taskRecord == nil {
			t.Logger.Printf("error retrieving task record: no record found for user %s task %s\n", userId, taskId)
		}

		if err != nil || taskRecord == nil {
			t.TaskRecord = nil
			return nil
		}
		t.Logger.Printf("Syncing record: %v\n", taskRecord)
		if err := db.DB.Model(taskRecord).Update("sync_status", "pending").Error; err != nil {
			t.Logger.Printf("error updating sync status: %v", types.WrapOperation("task.stop.mark_pending", taskRecord.ID, userId, err))
			t.TaskRecord = nil
			return nil
		}

		t.syncTaskRecordAsync(taskRecord.ID)

		t.TaskRecord = nil
	}

	t.Logger.Infof("op=task_tracking_stop_success user_id=%s attendance_id=%d task_id=%s", userId, (*t.Attendance).ID, taskId)
	return nil
}

func (t *TaskService) StopAllActiveRecords() error {
	if t.Attendance == nil || *t.Attendance == nil {
		return fmt.Errorf("stop all active records failed: attendance is nil")
	}
	attendance := (*t.Attendance)

	return t.TaskRepository.StopAllActiveRecords(attendance)
}

func (t *TaskService) GetLastTaskRecord(userId, taskId string) (*models.TaskRecord, error) {
	if t.Attendance != nil && *t.Attendance != nil {
		return t.TaskRepository.GetLastTaskRecordForAttendance(userId, (*t.Attendance).ID, taskId)
	}

	return t.TaskRepository.GetLastTaskRecord(userId, taskId)
}

func (t *TaskService) SetTaskRecordNotes(taskRecordId uint, notes string) {
	if taskRecord, err := t.GetTaskRecord(taskRecordId); err != nil {
		t.Logger.Println(err)
	} else {
		if t.Attendance == nil || *t.Attendance == nil {
			t.Logger.Printf("skipping task record notes update without active attendance: record_id=%d", taskRecordId)
			return
		}
		if taskRecord.UserID != (*t.Attendance).UserID || taskRecord.AttendanceID != (*t.Attendance).ID {
			t.Logger.Printf("skipping task record notes update outside current attendance: record_id=%d user_id=%s attendance_id=%d", taskRecordId, taskRecord.UserID, taskRecord.AttendanceID)
			return
		}
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
	var attendance *models.Attendance
	if t.Attendance != nil && *t.Attendance != nil {
		attendance = *t.Attendance
	} else {
		var err error
		attendance, err = t.AuthRepository.GetUserAttendance(userId)
		if err != nil {
			return nil, err
		}
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

	if err := t.TaskRepository.UpdateTaskRecord(tr, &taskRecord); err != nil {
		return err
	}

	return db.DB.Model(tr).Updates(map[string]any{
		"sync_status": "pending",
		"synced_at":   nil,
		"last_error":  "",
	}).Error
}

func (t *TaskService) stopCurrentTaskRecord(userId, taskId string) error {
	return t.StopTaskRecord(userId, taskId)
}

func (t *TaskService) syncTaskRecordAsync(recordID uint) {
	if recordID == 0 || t.SyncService == nil {
		return
	}

	utils.GoSafe("task.sync_task_record_async", func() {
		if err := t.SyncService.SyncTaskRecord(recordID); err != nil {
			t.Logger.Printlnf("error syncing task_record: %v", err)
		}
	})
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
