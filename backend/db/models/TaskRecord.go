package models

import (
	"time"
)

type TaskRecord struct {
	ID         uint      `gorm:"primaryKey"`
	UserID     string    `validate:"required"`
	User       User      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TaskID     string    `gorm:"not null"` // foreign key
	Task       Task      `gorm:"foreignKey:TaskID;references:TaskID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	FolderName string    `validate:"required"`
	StartedAt  time.Time `gorm:"autoCreateTime"`
	EndedAt    *time.Time
	Duration   *int32
}

type ExpTaskRecord struct {
	TaskID     string     `json:"task_id"`
	FolderName string     `json:"folder_name"`
	StartedAt  time.Time  `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at"`
	Duration   *int32     `json:"duration"`
}

func (tr TaskRecord) ToExp() ExpTaskRecord {
	return ExpTaskRecord{
		TaskID:     tr.TaskID,
		FolderName: tr.FolderName,
		StartedAt:  tr.StartedAt,
		EndedAt:    tr.EndedAt,
		Duration:   tr.Duration,
	}
}
