package repository

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fmt"
	"time"

	"gorm.io/gorm/clause"
)

type TaskRepository struct {
}

func NewTaskRepository() *TaskRepository {
	return &TaskRepository{}
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
	if err := db.DB.Where("id = ?", id).First(&task).Error; err != nil {
		return nil, err
	}

	return &task, nil
}

func (t *TaskRepository) GetTasks() ([]models.Task, error) {
	var tasks []models.Task
	if err := db.DB.Find(&tasks).Error; err != nil {
		return nil, err
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
	return db.DB.Create(&record).Error
}

func (t *TaskRepository) UpdateTaskRecord(oldRecord, newRecord *models.TaskRecord) error {
	return db.DB.Model(oldRecord).Updates(newRecord).Error
}

func (t *TaskRepository) CheckTaskRecord(userId, taskId string) (*models.TaskRecord, error) {
	var taskRecord models.TaskRecord
	if err := db.DB.Where("user_id = ? AND ended_at IS NULL", userId).First(&taskRecord).Error; err != nil {
		fmt.Print(err)

		return nil, nil
	}

	return &taskRecord, nil
}

// ==================== TASK RECORD ===========================

func (t *TaskRepository) GetTaskRecord(id string) (*models.TaskRecord, error) {
	var taskRecord models.TaskRecord

	if err := db.DB.Find(&taskRecord, id).Error; err != nil {
		return nil, err
	}
	return &taskRecord, nil
}

func (t *TaskRepository) StopAllActiveRecords(attendance *models.Attendance) error {
	now := time.Now()
	var taskRecords []models.TaskRecord

	fmt.Println("pass here stoppping all active records")

	if err := db.DB.Where("user_id = ? AND attendance_id = ? AND ended_at IS NULL", attendance.UserID, attendance.ID).
		Find(&taskRecords).Error; err != nil {
		return err
	}

	for _, taskRecord := range taskRecords {

		db.DB.Model(&taskRecord).Update("ended_at", now)
	}

	return nil
}
