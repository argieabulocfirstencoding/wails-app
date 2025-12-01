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
	cutoff := time.Now().Add(-20 * time.Hour)

	var lastAttendance models.Attendance
	// Find the LAST (most recent) attendance record for the user
	// that is NOT timed out (time_out IS NULL)
	// AND started WITHIN the last 20 hours (time_in >= cutoff)
	if err := db.DB.Where("user_id = ? AND time_out IS NULL AND time_in >= ?", userId, cutoff).Last(&lastAttendance).Error; err != nil {
		// If the record is NOT found (ErrRecordNotFound):
		// 1. There is no currently open session.
		// 2. OR, the last open session started more than 20 hours ago (and was therefore skipped by the query).
		// In either case, we create a new attendance record.
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

func GetAttendanceFromID(attendanceID uint) (*models.Attendance, error) {
	var attendance models.Attendance

	if err := db.DB.Where("attendances.id = ?", attendanceID).First(&attendance).Error; err != nil {
		return nil, err
	}

	return &attendance, nil
}

func GetUserAttendance(userId string) (*models.Attendance, error) {
	var attendance models.Attendance

	if err := db.DB.Where("attendances.user_id = ?", userId).Last(&attendance).Error; err != nil {
		return nil, err
	}

	return &attendance, nil
}
