package repository

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"
)

type AuthRepository struct {
}

func NewAuthRepository() *AuthRepository {
	return &AuthRepository{}
}

func (a *AuthRepository) GetUserAttendance(userId string) (*models.Attendance, error) {
	var attendance models.Attendance

	if err := db.DB.Where("attendances.user_id = ?", userId).Last(&attendance).Error; err != nil {
		return nil, err
	}

	return &attendance, nil
}

func (a *AuthRepository) GetAttendanceFromID(attendanceID uint) (*models.Attendance, error) {
	var attendance models.Attendance

	if err := db.DB.Where("attendances.id = ?", attendanceID).First(&attendance).Error; err != nil {
		return nil, err
	}

	return &attendance, nil
}
