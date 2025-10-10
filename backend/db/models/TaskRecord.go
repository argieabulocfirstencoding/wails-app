package models

import (
	"time"
)

type TaskRecord struct {
	ID           uint       `gorm:"primaryKey"`
	UserID       string     `gorm:"not null"`
	User         User       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	AttendanceID uint       `gorm:"not null"`
	Attendance   Attendance `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TaskID       string     `gorm:"not null"` // foreign key
	Task         Task       `gorm:"foreignKey:TaskID;references:TaskID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	StartedAt    time.Time  `gorm:"autoCreateTime"`
	EndedAt      *time.Time
	Duration     *int32
	SyncedAt     *time.Time
	SyncStatus   string 	`gorm:"not null;default:'pending'"`
	Screenshots  []Screenshot
}

type ExpTaskRecord struct {
	ID        uint       `json:"id"`
	TaskID    string     `json:"task_id"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
	Duration  *int32     `json:"duration"`
}

type TaskRecordWithScreenshots struct {
	Record      *ExpTaskRecord `json:"record"`
	Date        time.Time      `json:"takenAt"`
	Screenshots []string       `json:"screenshots"`
}

func (tr TaskRecord) ToExp() ExpTaskRecord {
	return ExpTaskRecord{
		ID:        tr.ID,
		TaskID:    tr.TaskID,
		StartedAt: tr.StartedAt,
		EndedAt:   tr.EndedAt,
		Duration:  tr.Duration,
	}
}
