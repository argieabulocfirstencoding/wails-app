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

const resumableAttendanceWindow = 16 * time.Hour

func NewAttendanceRepository() *AttendanceRepository {
	return &AttendanceRepository{
		logger: utils.NewLoggerService(),
	}
}

func (a *AttendanceRepository) CreateAttendance(userId string) (*models.Attendance, error) {
	var lastAttendance models.Attendance
	// Only a recent unclosed attendance can be resumed. Older records are
	// historical incomplete shifts and must not take over a new login.
	cutoff := time.Now().Add(-resumableAttendanceWindow)
	if err := db.DB.Where("user_id = ? AND time_out IS NULL AND time_in >= ?", userId, cutoff).Order("time_in DESC, id DESC").First(&lastAttendance).Error; err != nil {
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

// GetStaleOpenAttendances returns historical incomplete shifts for a user.
// These records are intentionally excluded from CreateAttendance recovery.
func (a *AttendanceRepository) GetStaleOpenAttendances(userID string, cutoff time.Time) ([]models.Attendance, error) {
	attendances := make([]models.Attendance, 0)
	err := db.DB.
		Where("user_id = ? AND time_out IS NULL AND time_in < ?", userID, cutoff).
		Order("time_in DESC, id DESC").
		Find(&attendances).Error
	return attendances, err
}

// CloseStaleAttendanceAt durably closes every unfinished task in a historical
// attendance and then closes the attendance itself. Callers validate employee
// ownership and the selected end time before invoking this method.
func (a *AttendanceRepository) CloseStaleAttendanceAt(userID string, attendanceID uint, endedAt time.Time) ([]uint, *models.Attendance, error) {
	updatedRecordIDs := make([]uint, 0)
	var closed models.Attendance

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var attendance models.Attendance
		if err := tx.Where("id = ? AND user_id = ? AND time_out IS NULL", attendanceID, userID).First(&attendance).Error; err != nil {
			return err
		}

		var records []models.TaskRecord
		if err := tx.Where("attendance_id = ? AND user_id = ? AND ended_at IS NULL", attendanceID, userID).Order("started_at ASC, id ASC").Find(&records).Error; err != nil {
			return err
		}
		for _, record := range records {
			if endedAt.Before(record.StartedAt) {
				return errors.New("shift end time must be after every unfinished task started")
			}
			duration := int32(endedAt.Sub(record.StartedAt).Seconds())
			if duration < 0 {
				duration = 0
			}
			if err := tx.Model(&models.TaskRecord{}).Where("id = ? AND ended_at IS NULL", record.ID).Updates(map[string]any{
				"ended_at":    endedAt,
				"duration":    duration,
				"sync_status": gorm.Expr("CASE WHEN sync_status IN ('identity_conflict', 'remote_missing') THEN sync_status ELSE 'pending' END"),
				"last_error":  "",
				"synced_at":   nil,
			}).Error; err != nil {
				return err
			}
			updatedRecordIDs = append(updatedRecordIDs, record.ID)
		}

		if err := tx.Model(&attendance).Updates(map[string]any{
			"time_out":    endedAt,
			"sync_status": "pending",
			"last_error":  "",
			"synced_at":   nil,
		}).Error; err != nil {
			return err
		}
		attendance.TimeOut = &endedAt
		attendance.SyncStatus = "pending"
		attendance.LastError = ""
		attendance.SyncedAt = nil
		closed = attendance
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return updatedRecordIDs, &closed, nil
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
