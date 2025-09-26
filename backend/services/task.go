package services

import (
	"errors"
	"fesapp/backend/db"

	"gorm.io/gorm"
)

func CreateTask(userId string, title, client string) (*db.Task, error) {
	task := db.Task{
		Title:  title,
		Client: client,
		UserID: userId,
	}

	if err := db.DB.Create(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, errors.New("task already exists for this client")
		}
		return nil, err
	}

	return &task, nil
}

