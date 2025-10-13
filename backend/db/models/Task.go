package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Task struct {
	ID           uint       `gorm:"primaryKey"`
	TaskID       string     `gorm:"type:char(36);"`
	Title        string     `validate:"required"`
	Client       string     `validate:"required"`
	UserID       string
	User         User 		`gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TaskRecord   []TaskRecord
}

type ExpTask struct {
	TaskID string `json:"taskId"`
	Title  string `json:"title"`
	Client string `json:"client"`
}

func (t *Task) BeforeCreate(tx *gorm.DB) (err error) {
	if t.TaskID == "" {
		t.TaskID = uuid.New().String()
	}
	return
}

func (t Task) ToExpTask() ExpTask {
	return ExpTask{
		TaskID: t.TaskID,
		Title:  t.Title,
		Client: t.Client,
	}
}

func ToExpTasks(tasks []Task) []ExpTask {
	exp := make([]ExpTask, len(tasks))
	for i, t := range tasks {
		exp[i] = ExpTask{
			TaskID: t.TaskID, Title: t.Title, Client: t.Client}
	}
	return exp
}
