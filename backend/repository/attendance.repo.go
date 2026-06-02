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
	a.logger.Infof("op=attendance_local_login_start user_id=%s", userId)
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
				a.logger.Warnf("op=attendance_local_login_failed user_id=%s err=%v", userId, err)
				return nil, err
			}

			a.logger.Infof("op=attendance_local_login_created user_id=%s attendance_id=%d time_in=%s", userId, lastAttendance.ID, lastAttendance.TimeIn.Format(time.RFC3339))
		} else {
			a.logger.Warnf("op=attendance_local_login_failed user_id=%s err=%v", userId, err)
			return nil, err
		}

	} else {
		a.logger.Infof("op=attendance_local_login_reused user_id=%s attendance_id=%d time_in=%s", userId, lastAttendance.ID, lastAttendance.TimeIn.Format(time.RFC3339))
	}

	return &lastAttendance, nil
}

func (a *AttendanceRepository) LogoutAttendance(attendance *models.Attendance) error {
	if attendance == nil {
		a.logger.Warn("op=attendance_local_logout_skipped reason=nil_attendance")
		return nil
	}

	now := time.Now()
	timeOut := now
	if attendance.TimeOut != nil {
		timeOut = *attendance.TimeOut
	}
	a.logger.Infof("op=attendance_local_logout_start user_id=%s attendance_id=%d time_out=%s already_timed_out=%t", attendance.UserID, attendance.ID, timeOut.Format(time.RFC3339), attendance.TimeOut != nil)
	attendance.TimeOut = &timeOut
	attendance.SyncStatus = "pending"
	attendance.SyncedAt = nil
	attendance.LastError = ""

	if err := db.DB.Model(attendance).Updates(map[string]any{
		"time_out":    timeOut,
		"sync_status": "pending",
		"synced_at":   nil,
		"last_error":  "",
	}).Error; err != nil {
		a.logger.Warnf("op=attendance_local_logout_failed user_id=%s attendance_id=%d err=%v", attendance.UserID, attendance.ID, err)
		return err
	}

	a.logger.Infof("op=attendance_local_logout_saved user_id=%s attendance_id=%d time_out=%s sync_status=%s", attendance.UserID, attendance.ID, timeOut.Format(time.RFC3339), attendance.SyncStatus)
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
