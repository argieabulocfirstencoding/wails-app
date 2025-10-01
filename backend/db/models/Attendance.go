package models

import (
	"time"

	"gorm.io/gorm"
)

type Attendance struct {
	gorm.Model
	UserID  string `gorm:"not null"`
	User    User   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TimeIn  time.Time
	TimeOut *time.Time
	SyncID  *string
}
