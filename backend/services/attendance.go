package services

import (
	"errors"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/utils"
	"time"

	"gorm.io/gorm"
)

var LoginAttendance = func(userId string) (*models.Attendance, error) {
	logger := utils.NewLoggerService()
	logger.Infof("op=attendance_local_login_start user_id=%s", userId)
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
				logger.Warnf("op=attendance_local_login_failed user_id=%s err=%v", userId, err)
				return nil, err
			}

			logger.Infof("op=attendance_local_login_created user_id=%s attendance_id=%d time_in=%s", userId, lastAttendance.ID, lastAttendance.TimeIn.Format(time.RFC3339))
		} else {
			logger.Warnf("op=attendance_local_login_failed user_id=%s err=%v", userId, err)
			return nil, err
		}

	} else {
		logger.Infof("op=attendance_local_login_reused user_id=%s attendance_id=%d time_in=%s", userId, lastAttendance.ID, lastAttendance.TimeIn.Format(time.RFC3339))
	}

	return &lastAttendance, nil
}

var LogoutAttendance = func(attendance *models.Attendance) error {
	logger := utils.NewLoggerService()
	if attendance == nil {
		logger.Warn("op=attendance_local_logout_skipped reason=nil_attendance")
		return nil
	}

	now := time.Now()
	logger.Infof("op=attendance_local_logout_start user_id=%s attendance_id=%d time_out=%s", attendance.UserID, attendance.ID, now.Format(time.RFC3339))
	attendance.TimeOut = &now
	attendance.SyncStatus = "pending"
	attendance.SyncedAt = nil
	attendance.LastError = ""

	if err := db.DB.Model(attendance).Updates(map[string]any{
		"time_out":    now,
		"sync_status": "pending",
		"synced_at":   nil,
		"last_error":  "",
	}).Error; err != nil {
		logger.Warnf("op=attendance_local_logout_failed user_id=%s attendance_id=%d err=%v", attendance.UserID, attendance.ID, err)
		return err
	}

	logger.Infof("op=attendance_local_logout_saved user_id=%s attendance_id=%d time_out=%s sync_status=%s", attendance.UserID, attendance.ID, now.Format(time.RFC3339), attendance.SyncStatus)
	return nil
}

var GetAttendanceFromID = func(attendanceID uint) (*models.Attendance, error) {
	var attendance models.Attendance

	if err := db.DB.Where("attendances.id = ?", attendanceID).First(&attendance).Error; err != nil {
		return nil, err
	}

	return &attendance, nil
}

var GetUserAttendance = func(userId string) (*models.Attendance, error) {
	var attendance models.Attendance

	if err := db.DB.Where("attendances.user_id = ?", userId).Last(&attendance).Error; err != nil {
		return nil, err
	}

	return &attendance, nil
}

var LoginUser = func(email string) (*models.OfflineLoginProps, *models.Attendance, *models.AuthUser, error) {
	user, err := OfflineLogin(email)
	if err != nil {
		return nil, nil, nil, err
	}

	authUser := &models.AuthUser{
		LocalID: user.ID,
		Email:   user.Email,
	}

	attendance, err := LoginAttendance(authUser.LocalID)
	if attendance == nil {
		return nil, nil, nil, err
	}

	offlineData := &models.OfflineLoginProps{
		AuthUser:   *authUser,
		Attendance: *attendance,
	}

	return offlineData, attendance, authUser, nil
}
