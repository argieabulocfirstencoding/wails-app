package service_classes

import (
	"context"
	"errors"
	config "fesapp/backend"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/repository"
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

	ctx context.Context
}

func NewTaskService(SyncService *SyncService) *TaskService {
	screenshotService := NewScreenshotService()
	screenshotService.SyncService = SyncService

	return &TaskService{
		TaskRepository:    repository.NewTaskRepository(),
		ScreenshotService: screenshotService,
		SyncService:       SyncService,
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
	if config.OnlineMode && isOnline {
		if err := t.SyncService.FesClientService.SyncClientsAndProjects(); err != nil {
			t.Logger.Printf("error fetching tasks from fes: %v", err)
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
			return nil, err
		}
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	record, err := t.newTaskRecord(userId, taskId, attendanceId, padagan)
	if err != nil {
		return nil, err
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

func (t *TaskService) StartTracking(taskId, localStorageValue string, padagan bool) {
	if t.Attendance == nil || *t.Attendance == nil {
		t.Logger.Println("StartTracking error: Attendance is nil")
		return
	}

	// if t.ScreenshotService.Running {
	// 	t.ScreenshotService.Stop()
	// }
	if err := t.TaskRepository.StopAllActiveRecords(*t.Attendance); err != nil {
		fmt.Printf("Error stoping active task record: %s", err)
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

	record, err := t.StartTaskRecord(taskId, padagan)
	if err != nil {
		t.Logger.Printf("error starting task record: %v", err)
		return
	}
	if record == nil {
		t.Logger.Printf("error starting task record: record is nil")
		return
	}

	t.TaskRecord = record

	if t.WorkingTaskID != nil && *t.WorkingTaskID != nil {
		t.ScreenshotService.TaskID = *t.WorkingTaskID
	}
	t.ScreenshotService.TaskRecordID = &record.ID

	task, err := t.TaskRepository.GetTask(taskId)
	if err != nil {
		return
	}

	if task.ProjectName == "Break" {
		t.SyncService.SyncTaskRecord(record.ID)
		return
	}

	t.ScreenshotService.Start(t.ctx)
}

func (t *TaskService) StopTracking(taskId string) {
	if t.Attendance == nil || *t.Attendance == nil {
		t.Logger.Println("StopTracking error: Attendance is nil")
		return
	}
	userId := (*t.Attendance).UserID

	t.ScreenshotService.Stop()
	err := t.StopTaskRecord(
		userId,
		taskId,
	)
	if err != nil {
		t.Logger.Println("StopTaskRecord error:", err)
		return
	}

	// Syncing TaskRecord to Firebase
	if config.OnlineMode {
		// Retrieving TaskRecord from TaskRecordID
		t.Logger.Print("Attempting to sync record")
		taskRecord, err := t.TaskRepository.GetLastTaskRecord(userId, taskId)
		t.TaskRecord = taskRecord

		if err != nil {
			t.Logger.Printf("error retrieving task record: %v\n", err)
		}
		if taskRecord == nil {
			t.Logger.Printf("error retrieving task record: no record found for user %s task %s\n", userId, taskId)
		}

		if err != nil || taskRecord == nil {
			t.TaskRecord = nil
			t.ScreenshotService.TaskRecordID = nil
			return
		}
		t.Logger.Printf("Syncing record: %v\n", taskRecord)
		db.DB.Model(taskRecord).Update("sync_status", "pending")

		if err := t.SyncService.SyncTaskRecord(taskRecord.ID); err != nil {
			t.Logger.Printf("error syncing task_record: %v", err)
		}

		t.TaskRecord = nil
	}

	t.ScreenshotService.TaskRecordID = nil
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
		t.TaskRepository.UpdateTaskRecord(taskRecord, taskRecord)

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
