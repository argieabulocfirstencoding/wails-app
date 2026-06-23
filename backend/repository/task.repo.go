package repository

import (
	"errors"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TaskRepository struct {
	AttendanceRepository *AttendanceRepository
}

func NewTaskRepository() *TaskRepository {
	return &TaskRepository{
		AttendanceRepository: NewAttendanceRepository(),
	}
}

func (t *TaskRepository) CreateTask(title, client string) (*models.Task, error) {
	return nil, nil
}

func (t *TaskRepository) BulkCreateTasks(tasks []models.Task) error {
	if len(tasks) == 0 {
		return nil
	}

	return db.DB.
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "task_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"title",
				"project_name",
				"client_id",
			}),
		}).
		Create(&tasks).
		Error
}

func (t *TaskRepository) BulkCreateClients(clients []models.Client) error {
	if len(clients) == 0 {
		return nil
	}

	return db.DB.Clauses(clause.OnConflict{
		UpdateAll: true, // updates all fields except primary keys
	}).Create(&clients).Error
}

func (t *TaskRepository) ClientExists(id string) (bool, error) {
	if id == "" {
		return false, nil
	}

	var count int64
	if err := db.DB.Model(&models.Client{}).
		Where("id = ?", id).
		Limit(1).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

func (t *TaskRepository) GetTask(id string) (*models.Task, error) {
	var task models.Task
	if err := db.DB.Where("id = ?", id).First(&task).Error; err != nil {
		return nil, err
	}

	return &task, nil
}

func (t *TaskRepository) GetTaskFromTaskID(id string) (*models.Task, error) {
	var task models.Task
	if err := db.DB.Where("task_id = ?", id).First(&task).Error; err != nil {
		return nil, err
	}

	return &task, nil
}

func (t *TaskRepository) GetTasks() ([]models.Task, error) {
	var tasks []models.Task

	result := db.DB.Preload("Client").Find(&tasks)
	if result.Error != nil {
		return nil, result.Error
	}

	return tasks, nil
}

func (t *TaskRepository) UpdateTask(oldTask, newTask *models.Task) error {
	return db.DB.Model(oldTask).Updates(newTask).Error
}

func (t *TaskRepository) DeleteTask(id string) error {
	return db.DB.Model(&models.Task{}).Where("id = ?", id).Delete(&models.Task{}).Error
}

func (t *TaskRepository) CreateTaskRecord(record *models.TaskRecord) error {
	return db.DB.Create(record).Error
}

func (t *TaskRepository) UpdateTaskRecord(oldRecord, newRecord *models.TaskRecord) error {
	return db.DB.Model(oldRecord).Updates(newRecord).Error
}

func (t *TaskRepository) CheckTaskRecord(userId, taskId string) (*models.TaskRecord, error) {
	var taskRecord models.TaskRecord
	if err := db.DB.Where("user_id = ? AND task_id = ? AND ended_at IS NULL", userId, taskId).First(&taskRecord).Error; err != nil {
		return nil, err
	}

	return &taskRecord, nil
}

// ==================== TASK RECORD ===========================

func (t *TaskRepository) GetTaskRecord(id uint) (*models.TaskRecord, error) {
	var taskRecord models.TaskRecord

	if err := db.DB.Find(&taskRecord, id).Error; err != nil {
		return nil, err
	}
	return &taskRecord, nil
}

func (t *TaskRepository) StopAllActiveRecords(attendance *models.Attendance) error {
	if attendance == nil {
		return fmt.Errorf("stop active records: attendance is nil")
	}

	now := time.Now()
	var taskRecords []models.TaskRecord

	if err := db.DB.Where("user_id = ? AND attendance_id = ? AND ended_at IS NULL", attendance.UserID, attendance.ID).
		Find(&taskRecords).Error; err != nil {
		return err
	}

	return db.DB.Transaction(func(tx *gorm.DB) error {
		for _, taskRecord := range taskRecords {
			taskRecord.SyncStatus = "pending"
			taskRecord.EndedAt = &now
			taskRecord.Duration = int32(now.Sub(taskRecord.StartedAt).Seconds())

			if err := tx.Save(&taskRecord).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (t *TaskRepository) GetLastTaskRecord(userId, taskId string) (*models.TaskRecord, error) {
	// TODO add fetch on firebase
	var taskRecord models.TaskRecord
	if err := db.DB.Where("user_id = ? AND task_id = ?", userId, taskId).Last(&taskRecord).Error; err != nil {
		fmt.Println("DB error:", err)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}

	// taskRecordExp := taskRecord.ToExp()
	return &taskRecord, nil
}

func (t *TaskRepository) GetLastTaskRecordFromAttendance(userId string, attendanceId uint) (*models.TaskRecord, error) {
	var taskRecord models.TaskRecord

	if err := db.DB.Where("user_id = ? AND attendance_id = ?", userId, attendanceId).
		Order("started_at DESC").
		First(&taskRecord).Error; err != nil {
		return nil, err
	}

	return &taskRecord, nil
}

func (t *TaskRepository) UpdateRecord(tr *models.TaskRecord) error {

	now := time.Now()
	tr.EndedAt = &now

	if !tr.StartedAt.IsZero() {
		duration := int32(now.Sub(tr.StartedAt).Seconds())
		tr.Duration = duration
	}

	// This part updates the database
	return db.DB.Model(tr).Updates(models.TaskRecord{
		EndedAt:    tr.EndedAt,
		Duration:   tr.Duration,
		SyncStatus: "pending",
	}).Error
}

func (t *TaskRepository) GetAllPreviousTasksRecords(userID string) ([]models.TaskRecord, error) {
	sub := db.DB.
		Model(&models.TaskRecord{}).
		Select("task_id, MAX(started_at) as started_at").
		Where("user_id = ?", userID).
		Group("task_id")

	var taskRecords []models.TaskRecord
	err := db.DB.
		Joins("JOIN (?) AS t ON t.task_id = task_records.task_id AND t.started_at = task_records.started_at", sub).
		Find(&taskRecords).Error

	if err != nil {
		fmt.Println("DB error:", err)
		return nil, err
	}

	return taskRecords, nil
}

func (t *TaskRepository) GetActiveTaskRecord(userId string, attendanceId uint) (*models.TaskRecord, error) {
	var taskRecord models.TaskRecord

	// Only get the active task record from the current attendance
	if err := db.DB.Where("user_id = ? AND attendance_id = ? AND ended_at IS NULL", userId, attendanceId).First(&taskRecord).Error; err != nil {
		return nil, err
	}

	return &taskRecord, nil
}
