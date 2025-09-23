package models

import (
	"time"

	"gorm.io/gorm"
)

type TaskRecord struct {
	gorm.Model
	UserID     uint      `validate:"required"`
	User       User      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TaskID     uint      `validate:"required"`
	Task       Task      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	FolderName string    `validate:"required"`
	StartedAt  time.Time `validate:"required"`
	EndedAt    *time.Time
	Duration   int32
}
