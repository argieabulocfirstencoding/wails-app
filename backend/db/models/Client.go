package models

type Client struct {
	ID          string `gorm:"primaryKey"`
	Fullname    string `gorm:"type:char(36);"`
	CompanyName string `validate:"required"`
	Tasks       []Task
}
