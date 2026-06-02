package models

import "time"

type Screenshot struct {
	ID           uint       `gorm:"primaryKey"`
	TaskRecordID uint       `gorm:"not null;index:idx_screenshots_task_record_taken,priority:1"`
	TaskRecord   TaskRecord `gorm:"foreignKey:TaskRecordID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TaskID       string     `gorm:"not null;index:idx_screenshots_task_taken,priority:1"`
	Task         Task       `gorm:"-:all"`
	Memo         string     `gorm:"not null;default:''"`
	Notes        string     `gorm:"not null;default:''"`
	FilePath     string     `gorm:"type:text"`
	CloudURL     *string
	TakenAt      time.Time `gorm:"not null;index:idx_screenshots_task_record_taken,priority:2;index:idx_screenshots_task_taken,priority:2"`
}
