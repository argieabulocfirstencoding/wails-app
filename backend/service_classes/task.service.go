package service_classes

import (
	"context"
	config "fesapp/backend"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/repository"
	"fesapp/backend/services"
	"fesapp/backend/utils"
	"fmt"
	"time"
)

type TaskService struct {
	TaskRepository    *repository.TaskRepository
	ScreenshotService *ScreenshotService
	SyncService       *SyncService
	Logger            *utils.LoggerService
	WorkingTaskID     *string
	Attendance        *models.Attendance
	ctx               context.Context
}

func NewTaskService() *TaskService {
	return &TaskService{
		TaskRepository:    repository.NewTaskRepository(),
		ScreenshotService: NewScreenshotService(),
		SyncService:       NewSyncService(),
	}
}

func (t *TaskService) AddContext(ctx context.Context) {
	t.ctx = ctx
}

func (t *TaskService) SetAttendance(attendance *models.Attendance) {
	t.Attendance = attendance
}

func (t *TaskService) CreateTask(title, client string) (*models.ExpTask, error) {
	return nil, nil
}

func (t *TaskService) GetTask(id string) (*models.ExpTask, error) {
	return nil, nil
}

func (t *TaskService) GetTasks(isOnline bool) ([]models.ExpTask, error) {
	if config.OnlineMode && isOnline {
		if err := t.SyncService.fesClient.SyncClientsAndProjects(); err != nil {
			utils.Logger.Printf("error fetching tasks from fes: %v", err)
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

func (t *TaskService) GetTaskRecord(id string) (*models.TaskRecord, error) {
	return t.TaskRepository.GetTaskRecord(id)
}

// ======================== TASK RECORD ACTIONS ====================================

func (t *TaskService) StartTaskRecord(taskId string, padagan bool) (*db.TaskRecord, error) {
	userId := t.Attendance.UserID
	attendanceId := t.Attendance.ID

	if taskRecord, err := t.TaskRepository.CheckTaskRecord(userId, taskId); err == nil {
		return nil, err
	} else {
		if err := t.updateRecord(taskRecord); err != nil {
			return nil, err
		}
	}

	record, err := t.newTaskRecord(userId, taskId, attendanceId, padagan)

	return record, err
}

func (t *TaskService) StopTaskRecord(userId, taskId string) error {
	return nil
}

func (t *TaskService) StartTracking(taskId, localStorageValue string, padagan bool) {
	userId := t.Attendance.UserID
	attendanceId := t.Attendance.ID

	if t.ScreenshotService.Running {
		t.ScreenshotService.Stop()
	}

	if err := services.StopAllActiveRecords(userId); err != nil {
		fmt.Printf("Error stoping active task record: %s", err)
	}

	if t.ScreenshotService.TaskRecordID != nil {
		record, err := services.GetTaskRecordFromID(*t.ScreenshotService.TaskRecordID)

		if err != nil {
			fmt.Printf("Error on syncing record: %s", err)
		} else {
			t.SyncService.SyncTaskRecord(record)
		}

	}

	t.ScreenshotService.FolderName = localStorageValue
	t.WorkingTaskID = &taskId
	record, err := services.StartTaskRecord(userId, taskId, attendanceId, padagan)

	if err != nil {
		t.Logger.Printf("error starting task record: %v", err)
	}
	t.ScreenshotService.TaskID = t.WorkingTaskID
	t.ScreenshotService.TaskRecordID = &record.ID

	task, err := services.GetTaskFromID(taskId)
	if err != nil {
		return
	}

	if task.ProjectName == "Break" {
		t.SyncService.SyncTaskRecord(record)
		return
	}

	t.ScreenshotService.Start(t.ctx)
}

func (t *TaskService) StopTracking(taskid string) {

}

func (t *TaskService) StopAllActiveRecords() error {
	attendance := t.Attendance

	return t.TaskRepository.StopAllActiveRecords(attendance)
}

func (t *TaskService) GetLastTaskRecord(userId, taskId string) (*models.TaskRecord, error) {
	return nil, nil
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

	if err := services.CalculateGap(&record, now); err != nil {
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
