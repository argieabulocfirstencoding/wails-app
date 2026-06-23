package db

import (
	"time"

	"gorm.io/gorm"
)

type bootstrapUser struct {
	ID           string `gorm:"primaryKey"`
	Name         string
	FirstName    string
	LastName     string
	MiddleName   string
	Position     string
	Birthdate    string
	Phone        string
	Address      string
	Photo        string
	Email        string
	FESID        string
	SupervisorId string
	EmployeeId   string
	EmploymentId string
}

func (bootstrapUser) TableName() string {
	return "users"
}

type bootstrapTask struct {
	ID          uint   `gorm:"primaryKey"`
	TaskID      string `gorm:"type:char(36);unique"`
	Title       string
	ProjectName string
	ClientID    *string
}

func (bootstrapTask) TableName() string {
	return "tasks"
}

type bootstrapAttendance struct {
	ID            uint `gorm:"primaryKey"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     gorm.DeletedAt `gorm:"index"`
	UserID        string         `gorm:"not null;index:idx_attendances_user_time_in,priority:1"`
	ClientTime    uint32         `gorm:"not null;default:0"`
	TotalGap      uint32         `gorm:"not null;default:0"`
	TimeIn        time.Time      `gorm:"index:idx_attendances_user_time_in,priority:2"`
	TimeOut       *time.Time
	LoginLogoutID *string
	SyncedAt      *time.Time
	SyncStatus    string `gorm:"not null;default:'pending';index:idx_attendances_sync_status"`
	AttemptCount  int32  `gorm:"not null;default:0"`
	LastError     string `gorm:"type:text;not null;default:''"`
	LastAttemptAt *time.Time
}

func (bootstrapAttendance) TableName() string {
	return "attendances"
}

type bootstrapScreenshot struct {
	ID           uint   `gorm:"primaryKey"`
	TaskRecordID uint   `gorm:"not null;index:idx_screenshots_task_record_taken,priority:1"`
	TaskID       string `gorm:"not null;index:idx_screenshots_task_taken,priority:1"`
	Memo         string `gorm:"not null;default:''"`
	Notes        string `gorm:"not null;default:''"`
	FilePath     string `gorm:"type:text"`
	CloudURL     *string
	TakenAt      time.Time `gorm:"not null;index:idx_screenshots_task_record_taken,priority:2;index:idx_screenshots_task_taken,priority:2"`
}

func (bootstrapScreenshot) TableName() string {
	return "screenshots"
}

func bootstrapCoreSchema(database *gorm.DB) error {
	return database.AutoMigrate(
		&bootstrapUser{},
		&bootstrapTask{},
		&bootstrapAttendance{},
	)
}

func bootstrapScreenshotSchema(database *gorm.DB) error {
	return database.AutoMigrate(&bootstrapScreenshot{})
}
