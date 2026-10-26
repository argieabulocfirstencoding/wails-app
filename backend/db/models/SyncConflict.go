package models

import "time"

// SyncConflict preserves both sides of a Bubble/local mismatch until the user
// explicitly selects the source for every mutable field.
type SyncConflict struct {
	ID                       uint `gorm:"primaryKey"`
	CreatedAt                time.Time
	UpdatedAt                time.Time
	UserID                   string    `gorm:"not null;index:idx_sync_conflicts_user_status,priority:1"`
	RecordType               string    `gorm:"not null;index:idx_sync_conflicts_user_status,priority:2"`
	AttendanceID             *uint     `gorm:"index"`
	TaskRecordID             *uint     `gorm:"index"`
	RemoteID                 string    `gorm:"not null;index"`
	ExpectedRemoteModifiedAt time.Time `gorm:"not null"`
	LocalValues              string    `gorm:"type:text;not null"`
	BubbleValues             string    `gorm:"type:text;not null"`
	Differences              string    `gorm:"type:text;not null"`
	Status                   string    `gorm:"not null;default:'open';index:idx_sync_conflicts_user_status,priority:3"`
	Resolution               string    `gorm:"type:text;not null;default:''"`
	ResolvedAt               *time.Time
}

type SyncConflictField struct {
	Field       string `json:"field"`
	LocalValue  any    `json:"localValue"`
	BubbleValue any    `json:"bubbleValue"`
}
