package models

import "time"

type Screenshot struct {
	ID           uint       `gorm:"primaryKey"`
	TaskRecordID uint       `gorm:"not null"`
	TaskRecord   TaskRecord `gorm:"foreignKey:TaskRecordID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TaskID       string     `gorm:"not null"`
	Task         Task       `gorm:"foreignKey:TaskID;references:TaskID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Memo         string     `gorm:"not null;default:''"`
	Notes        string     `gorm:"not null;default:''"`
	FilePath     string     `gorm:"type:text"`
	CloudURL     *string
	TakenAt      time.Time `gorm:"not null"`
}
