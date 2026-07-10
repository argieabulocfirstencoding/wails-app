package models

type Client struct {
	ID          string `gorm:"primaryKey"`
	Fullname    string
	CompanyName string `validate:"required"`
	Tasks       []Task
}
