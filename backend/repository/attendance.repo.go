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
	var lastAttendance models.Attendance
	// An unclosed attendance belongs to a prior interrupted shift. Reuse it so
	// the caller can require a review instead of creating overlapping shifts.
	if err := db.DB.Where("user_id = ? AND time_out IS NULL", userId).Order("time_in DESC, id DESC").First(&lastAttendance).Error; err != nil {
		// No currently open session exists, so begin a new attendance record.
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
	return a.LogoutAttendanceAt(attendance, time.Now())
}

func (a *AttendanceRepository) LogoutAttendanceAt(attendance *models.Attendance, endedAt time.Time) error {
	if attendance == nil {
		return errors.New("attendance is nil")
	}
	if err := db.DB.Model(attendance).Updates(map[string]any{
		"time_out":    endedAt,
		"sync_status": "pending",
	}).Error; err != nil {
		return err
	}

	return nil
}

func (a *AttendanceRepository) GetOpenAttendanceFromUserID(userId string) (*models.Attendance, error) {
	var attendance models.Attendance
	if err := db.DB.Where("user_id = ? AND time_out IS NULL", userId).Order("time_in DESC, id DESC").First(&attendance).Error; err != nil {
		return nil, err
	}
	return &attendance, nil
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
