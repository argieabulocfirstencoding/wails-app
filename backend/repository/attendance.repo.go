package repository

import (
	"errors"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/utils"
	"time"

	"gorm.io/gorm"
)

type AttendanceRepository struct {
	logger *utils.LoggerService
}

func NewAttendanceRepository() *AttendanceRepository {
	return &AttendanceRepository{
		logger: utils.NewLoggerService(),
	}
}

func (a *AttendanceRepository) CreateAttendance(userId string) (*models.Attendance, error) {

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

			a.logger.Print("Create new attendance session")
		} else {
			a.logger.Print("Error creating new attendance session" + err.Error())
			return nil, err
		}

	}

	return &lastAttendance, nil
}

func (a *AttendanceRepository) LogoutAttendance(attendance *models.Attendance) error {
	now := time.Now()
	if err := db.DB.Model(attendance).Updates(map[string]any{
		"time_out":    now,
		"sync_status": "pending",
	}).Error; err != nil {
		return err
	}

	return nil
}

func (a *AttendanceRepository) GetAttendanceFromID(attendanceID uint) (*models.Attendance, error) {
	var attendance models.Attendance

	if err := db.DB.Where("attendances.id = ?", attendanceID).First(&attendance).Error; err != nil {
		return nil, err
	}

	return &attendance, nil
}

func (a *AttendanceRepository) GetAttendanceFromUserID(userId string) (*models.Attendance, error) {
	var attendance models.Attendance

	if err := db.DB.Where("attendances.user_id = ?", userId).Last(&attendance).Error; err != nil {
		return nil, err
	}

	return &attendance, nil
}
