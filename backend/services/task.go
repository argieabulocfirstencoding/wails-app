package services

import (
	"errors"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TODO on firebase task integration, use firebase task id
func CreateTask(userId, title, ClientID string, taskID *string) (*db.Task, error) {
	// NOTE deprecated, desktop users cannot create new tasks, backend function only
	task := db.Task{
		Title:    title,
		ClientID: ClientID,
		// UserID:   userId,
	}

	if taskID != nil {
		task.TaskID = *taskID
	}

	if err := db.DB.Create(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, errors.New("task already exists for this client")
		}
		return nil, err
	}

	return &task, nil
}

func BulkCreateTasks(tasks []models.Task) error {
	if len(tasks) == 0 {
		return nil
	}

	return db.DB.Clauses(clause.OnConflict{
		UpdateAll: true, // updates all fields except primary keys
	}).Create(&tasks).Error
}

func GetTasks() ([]db.Task, error) {
	if db.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var tasks []db.Task

	result := db.DB.Preload("Client").Find(&tasks)
	if result.Error != nil {
		return nil, result.Error
	}

	return tasks, nil
}

func getTaskFromID(taskID string) (*models.Task, error) {
	var task models.Task

	if err := db.DB.Where("tasks.task_id = ?", taskID).First(&task).Error; err != nil {
		return nil, err
	}

	return &task, nil
}
