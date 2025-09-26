package models

type Task struct {
	ID         uint   `gorm:"primaryKey"`
	Title      string `gorm:"uniqueIndex:idx_name_client" validate:"required"`
	Client     string `gorm:"uniqueIndex:idx_name_client" validate:"required"`
	UserID     string
	User       User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TaskRecord []TaskRecord
}

type ExpTask struct {
	Title  string `json:"title"`
	Client string `json:"client"`
}

func (t Task) ToExpTask() ExpTask {
	return ExpTask{
		Title:  t.Title,
		Client: t.Client,
	}
}

func ToExpTasks(tasks []Task) []ExpTask {
	exp := make([]ExpTask, len(tasks))
	for i, t := range tasks {
		exp[i] = ExpTask{Title: t.Title, Client: t.Client}
	}
	return exp
}
