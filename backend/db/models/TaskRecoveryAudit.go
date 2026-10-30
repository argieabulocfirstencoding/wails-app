package models

import "time"

// TaskRecoveryAudit records an explicit user decision made while repairing a
// legacy task row that had no durable Bubble identity.
type TaskRecoveryAudit struct {
	ID           uint `gorm:"primaryKey"`
	CreatedAt    time.Time
	UserID       string `gorm:"not null;index:idx_task_recovery_audits_user_record,priority:1"`
	TaskRecordID uint   `gorm:"not null;index:idx_task_recovery_audits_user_record,priority:2"`
	Action       string `gorm:"not null"`
	RemoteID     string `gorm:"not null;default:''"`
	Details      string `gorm:"type:text;not null;default:''"`
}
