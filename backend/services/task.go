package services

import (
	"errors"
	"fesapp/backend/db"
	"fmt"

	"gorm.io/gorm"
)

// TODO on firebase task integration, use firebase task id
func CreateTask(userId, title, client string, taskID *string) (*db.Task, error) {
	task := db.Task{
		Title:  title,
		Client: client,
		UserID: userId,
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

func GetTasks(userId string) ([]db.Task, error) {
	if db.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var tasks []db.Task

	result := db.DB.Where("user_id = ?", userId).Find(&tasks)
	if result.Error != nil {
		return nil, result.Error
	}

	return tasks, nil
}
