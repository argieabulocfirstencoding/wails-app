package serviceclasses

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/repository"
	"fesapp/backend/services"
	"time"
)

type TaskService struct {
	TaskRepository *repository.TaskRepository
}

func NewTaskService() *TaskService {
	return &TaskService{
		TaskRepository: repository.NewTaskRepository(),
	}
}

func (t *TaskService) CreateTask(title, client string) (*models.ExpTask, error) {
	return nil, nil
}

func (t *TaskService) GetTask(id string) (*models.ExpTask, error) {
	return nil, nil
}

func (t *TaskService) GetTasks() ([]*models.ExpTask, error) {
	return nil, nil
}

func (t *TaskService) UpdateTask(id string, title, client string) (*models.ExpTask, error) {
	return nil, nil
}

func (t *TaskService) DeleteTask(id string) error {
	return nil
}

func (t *TaskService) StartTaskRecord(userId, taskId string, attendanceId uint, padagan bool) (*db.TaskRecord, error) {

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
