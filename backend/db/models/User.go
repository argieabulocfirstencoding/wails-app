package models

type User struct {
	ID       string   `gorm:"primaryKey"`
	Name     string `validate:"required,min=2"`
	Email    string `gorm:"uniqueIndex" validate:"required,email"`
	Password string `validate:"required,min=6"`
	Tasks    []Task
}
