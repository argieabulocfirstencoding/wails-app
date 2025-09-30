package models

import "time"

type Screenshot struct {
	ID           uint       `gorm:"primaryKey"`
	TaskRecordID uint       `gorm:"not null"`
	TaskRecord   TaskRecord `gorm:"foreignKey:TaskRecordID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	FilePath     string     `gorm:"type:text"`
	CloudURL     *string
	TakenAt      time.Time `gorm:"not null"`
}
