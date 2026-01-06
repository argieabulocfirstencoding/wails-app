package services

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fmt"

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

	if err := db.DB.
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "task_id"}},
			DoNothing: true,
		}).Create(&task).Error; err != nil {
		return nil, err
	}

	return &task, nil
}

func BulkCreateTasks(tasks []models.Task) error {
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

func GetTaskFromID(taskID string) (*models.Task, error) {
	var task models.Task

	if err := db.DB.Where("task_id = ?", taskID).
		Preload("Client").
		First(&task).
		Error; err != nil {
		return nil, err
	}

	return &task, nil
}
