package services

import (
	"errors"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fmt"
	"time"

	"gorm.io/gorm"
)

func LoginAttendance(userId string) (*uint, error) {
	// TODO sync in firebase
	// TODO add firebase check for the last synced attendance

	// NOTE this is a local check for previous attendance
	var lastAttendance models.Attendance
	if err := db.DB.Where("user_id = ? AND time_out IS NULL", userId).Last(&lastAttendance).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			lastAttendance = models.Attendance{
				UserID: userId,
				TimeIn: time.Now(),
			}

			if err := db.DB.Create(&lastAttendance).Error; err != nil {
				return nil, err
			}

			fmt.Print("Create new attendance session")
		} else {
			fmt.Print("Error creating new attendance session" + err.Error())
			return nil, err
		}

	}

	return &lastAttendance.ID, nil
}

func LogoutAttendance(attendanceId uint) error {
	if err := db.DB.Model(&models.Attendance{}).Where("attendances.id = ?", attendanceId).Update("time_out", time.Now()).Error; err != nil {
		return err
	}

	// TODO sync in firebase

	return nil
}
