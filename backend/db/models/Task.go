package models

type Task struct {
	ID         uint   `gorm:"primaryKey"`
	Name       string `gorm:"uniqueIndex:idx_name_client" validate:"required"`
	Client     string `gorm:"uniqueIndex:idx_name_client" validate:"required"`
	UserID     uint
	User       User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	TaskRecord []TaskRecord
}
