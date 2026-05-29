package models

import (
	"time"

	"gorm.io/gorm"
)

type Attendance struct {
	gorm.Model
	UserID        string    `gorm:"not null;index:idx_attendances_user_time_in,priority:1"`
	User          User      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	ClientTime    uint32    `gorm:"not null;default:0"`
	TotalGap      uint32    `gorm:"not null;default:0"`
	TimeIn        time.Time `gorm:"index:idx_attendances_user_time_in,priority:2"`
	TimeOut       *time.Time
	LoginLogoutID *string
	SyncedAt      *time.Time
	SyncStatus    string `gorm:"not null;default:'pending';index:idx_attendances_sync_status"`
	AttemptCount  int32  `gorm:"not null;default:0"`
	LastError     string `gorm:"type:text;not null;default:''"`
	LastAttemptAt *time.Time
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

type OfflineLoginProps struct {
	AuthUser   AuthUser
	Attendance Attendance
}
