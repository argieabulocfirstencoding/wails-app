package models

import (
	"time"

	"gorm.io/gorm"
)

type Attendance struct {
	gorm.Model
	UserID     string `gorm:"not null"`
	User       User   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TimeIn     time.Time
	TimeOut    *time.Time
	SyncedAt   *time.Time
	SyncStatus string `gorm:"not null;default:'pending'"`
}

type ActivityLogRecord struct {
	Attendance Attendance   `json:"attendance"`
	Records    []TaskRecord `json:"records"`
}

type ExpAttendance struct {
	ID         uint   `json:"id"`
	SyncStatus string `json:"sync_status"`
}

func (a Attendance) ToExp() ExpAttendance {
	return ExpAttendance{
		ID:         a.ID,
		SyncStatus: a.SyncStatus,
	}
}
