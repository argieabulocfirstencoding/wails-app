package models

import (
	"time"
)

type TaskRecord struct {
	ID                     uint       `gorm:"primaryKey"`
	UserID                 string     `gorm:"not null;index:idx_task_records_user_attendance_ended,priority:1"`
	User                   User       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	AttendanceID           uint       `gorm:"not null;index:idx_task_records_user_attendance_ended,priority:2;index:idx_task_records_attendance_sync_started,priority:1"`
	Attendance             Attendance `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TaskID                 string     `gorm:"not null;index:idx_task_records_task_started,priority:1"` // foreign key
	Task                   Task       `gorm:"foreignKey:TaskID;references:TaskID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	StartedAt              time.Time  `gorm:"not null;autoCreateTime;index:idx_task_records_task_started,priority:2;index:idx_task_records_attendance_sync_started,priority:3"`
	Padagan                bool       `gorm:"not null;default:false"`
	Notes                  string     `gorm:"not null;default:''"`
	EndedAt                *time.Time `gorm:"index:idx_task_records_user_attendance_ended,priority:3"`
	Duration               int32      `gorm:"not null;default:0"`
	SyncedAt               *time.Time
	Productivity           *string
	ClientSessionID        *string
	TaskInAttemptedAt      *time.Time
	TaskInIdempotent       bool `gorm:"not null;default:false"`
	RemoteModifiedAt       *time.Time
	SyncStatus             string `gorm:"not null;default:'pending';index:idx_task_records_attendance_sync_started,priority:2"`
	AttemptCount           int32  `gorm:"not null;default:0"`
	LastError              string `gorm:"type:text;not null;default:''"`
	LastAttemptAt          *time.Time
	Gap                    int32        `gorm:"not null;default:0"`
	PendingScreenshotCount int          `gorm:"-"`
	Screenshots            []Screenshot `gorm:"-"`
}

type ExpTaskRecord struct {
	ID         uint       `json:"id"`
	TaskID     string     `json:"task_id"`
	StartedAt  time.Time  `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at"`
	Duration   int32      `json:"duration"`
	Padagan    bool       `json:"padagan"`
	Notes      string     `json:"notes"`
	SyncStatus string     `json:"sync_status"`
}

type TaskRecordWithScreenshots struct {
	Record      *ExpTaskRecord `json:"record"`
	Date        time.Time      `json:"takenAt"`
	Screenshots []string       `json:"screenshots"`
}

// StaleOpenTaskRecord identifies an unclosed task from a previous attendance.
// It is intentionally a read model: employees must explicitly choose a task-out
// time before the persisted record is changed.
type StaleOpenTaskRecord struct {
	ID               uint       `json:"id"`
	TaskID           string     `json:"taskId"`
	TaskTitle        string     `json:"taskTitle"`
	AttendanceID     uint       `json:"attendanceId"`
	StartedAt        time.Time  `json:"startedAt"`
	SuggestedEndedAt *time.Time `json:"suggestedEndedAt"`
	SyncStatus       string     `json:"syncStatus"`
}

// TaskShiftTime is the timer state for one task inside the current
// login/logout attendance session. CompletedShiftSeconds excludes a running
// record; clients add elapsed time from CurrentSessionStartedAt while it runs.
type TaskShiftTime struct {
	CurrentSessionStartedAt *time.Time `json:"current_session_started_at"`
	CompletedShiftSeconds   int64      `json:"completed_shift_seconds"`
}

// ShiftTime is the aggregate timer state for every task in one login/logout
// attendance session. CompletedShiftSeconds excludes a running record; clients
// add elapsed time from CurrentSessionStartedAt while it runs.
type ShiftTime struct {
	CurrentSessionStartedAt *time.Time `json:"current_session_started_at"`
	CompletedShiftSeconds   int64      `json:"completed_shift_seconds"`
}

func (tr TaskRecord) ToExp() ExpTaskRecord {
	return ExpTaskRecord{
		ID:         tr.ID,
		TaskID:     tr.TaskID,
		StartedAt:  tr.StartedAt,
		EndedAt:    tr.EndedAt,
		Duration:   tr.Duration,
		Padagan:    tr.Padagan,
		Notes:      tr.Notes,
		SyncStatus: tr.SyncStatus,
	}
}
