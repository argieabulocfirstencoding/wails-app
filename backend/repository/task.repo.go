package repository

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fmt"
)

type TaskRepository struct {
}

func NewTaskRepository() *TaskRepository {
	return &TaskRepository{}
}

func (t *TaskRepository) CreateTask(title, client string) (*models.Task, error) {
	return nil, nil
}

func (t *TaskRepository) GetTask(id string) (*models.Task, error) {
	return nil, nil
}

func (t *TaskRepository) GetTasks() ([]*models.Task, error) {
	return nil, nil
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
