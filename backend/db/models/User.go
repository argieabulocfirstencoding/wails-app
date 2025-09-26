package models

type User struct {
	ID string `gorm:"primaryKey"`
	// Name     string `validate:"required,min=2"` // TODO add this when profile is supported
	Email string `gorm:"uniqueIndex" validate:"required,email"`
	Tasks []Task
}
