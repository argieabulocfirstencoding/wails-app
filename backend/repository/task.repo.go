package repository

import (
	"errors"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/utils"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TaskRepository struct {
	AttendanceRepository *AttendanceRepository
	logger               *utils.LoggerService
}

func NewTaskRepository() *TaskRepository {
	return &TaskRepository{
		AttendanceRepository: NewAttendanceRepository(),
		logger:               utils.NewLoggerService(),
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

func (t *TaskRepository) GetTask(id string) (*models.Task, error) {
	var task models.Task
	if err := db.DB.Where("task_id = ?", id).First(&task).Error; err != nil {
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
	return db.DB.Model(&models.Task{}).Where("task_id = ?", id).Delete(&models.Task{}).Error
}

func (t *TaskRepository) CreateTaskRecord(record *models.TaskRecord) error {
	if record == nil {
		return fmt.Errorf("create task record: record is nil")
	}
	t.logger.Infof("op=task_local_in_start user_id=%s attendance_id=%d task_id=%s started_at=%s", record.UserID, record.AttendanceID, record.TaskID, record.StartedAt.Format(time.RFC3339))
	if err := db.DB.Create(record).Error; err != nil {
		t.logger.Warnf("op=task_local_in_failed user_id=%s attendance_id=%d task_id=%s err=%v", record.UserID, record.AttendanceID, record.TaskID, err)
		return err
	}
	t.logger.Infof("op=task_local_in_saved user_id=%s attendance_id=%d task_id=%s record_id=%d started_at=%s sync_status=%s", record.UserID, record.AttendanceID, record.TaskID, record.ID, record.StartedAt.Format(time.RFC3339), record.SyncStatus)
	return nil
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

	if err := db.DB.First(&taskRecord, id).Error; err != nil {
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
			t.logger.Infof("op=task_local_auto_out_start user_id=%s attendance_id=%d task_id=%s record_id=%d ended_at=%s duration=%d", taskRecord.UserID, taskRecord.AttendanceID, taskRecord.TaskID, taskRecord.ID, now.Format(time.RFC3339), taskRecord.Duration)

			if err := tx.Save(&taskRecord).Error; err != nil {
				t.logger.Warnf("op=task_local_auto_out_failed user_id=%s attendance_id=%d task_id=%s record_id=%d err=%v", taskRecord.UserID, taskRecord.AttendanceID, taskRecord.TaskID, taskRecord.ID, err)
				return err
			}
			t.logger.Infof("op=task_local_auto_out_saved user_id=%s attendance_id=%d task_id=%s record_id=%d ended_at=%s duration=%d sync_status=%s", taskRecord.UserID, taskRecord.AttendanceID, taskRecord.TaskID, taskRecord.ID, now.Format(time.RFC3339), taskRecord.Duration, taskRecord.SyncStatus)
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
	t.logger.Infof("op=task_local_out_start user_id=%s attendance_id=%d task_id=%s record_id=%d ended_at=%s duration=%d", tr.UserID, tr.AttendanceID, tr.TaskID, tr.ID, now.Format(time.RFC3339), tr.Duration)

	// This part updates the database
	if err := db.DB.Model(tr).Updates(map[string]any{
		"ended_at":    tr.EndedAt,
		"duration":    tr.Duration,
		"sync_status": "pending",
		"synced_at":   nil,
		"last_error":  "",
	}).Error; err != nil {
		t.logger.Warnf("op=task_local_out_failed user_id=%s attendance_id=%d task_id=%s record_id=%d err=%v", tr.UserID, tr.AttendanceID, tr.TaskID, tr.ID, err)
		return err
	}
	t.logger.Infof("op=task_local_out_saved user_id=%s attendance_id=%d task_id=%s record_id=%d ended_at=%s duration=%d sync_status=pending", tr.UserID, tr.AttendanceID, tr.TaskID, tr.ID, now.Format(time.RFC3339), tr.Duration)
	return nil
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
