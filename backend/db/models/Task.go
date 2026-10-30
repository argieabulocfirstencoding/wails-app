package models

import (
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Task struct {
	ID          uint   `gorm:"primaryKey"`
	TaskID      string `gorm:"type:char(36);unique"`
	Title       string `validate:"required"`
	ProjectName string `validate:"required"`
	ClientID    *string
	Client      Client `gorm:"foreignKey:ClientID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
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
	title := t.DisplayLabel()
	return ExpTask{
		TaskID: t.TaskID,
		Title:  title,
		Client: strings.TrimSpace(t.Client.Fullname),
	}
}

func ToExpTasks(tasks []Task) []ExpTask {
	exp := make([]ExpTask, len(tasks))
	for i, t := range tasks {
		title := t.DisplayLabel()
		exp[i] = ExpTask{
			TaskID: t.TaskID,
			Title:  title,
			Client: strings.TrimSpace(t.Client.Fullname),
		}
	}
	return exp
}

// DisplayLabel is the consistent user-facing task identity used across task,
// activity, and conflict views.
func (t Task) DisplayLabel() string {
	return taskDisplayTitle(t.Client.Fullname, t.Title, t.ProjectName)
}

func taskDisplayTitle(clientName, title, projectName string) string {
	parts := make([]string, 0, 3)
	for _, part := range []string{clientName, title, projectName} {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}

	return strings.Join(parts, " - ")
}
