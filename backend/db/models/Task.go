package models

import (
	"gorm.io/gorm"
)

type Task struct {
	gorm.Model
	Name       string `gorm:"uniqueIndex:idx_name_client" validate:"required"`
	Client     string `gorm:"uniqueIndex:idx_name_client" validate:"required"`
	UserID     uint
	User       User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TaskRecord []TaskRecord
}
