package services

import (
	"errors"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"time"

	"gorm.io/gorm"
)

func CalculateGap(tr *models.TaskRecord, now time.Time) error {
	taskRecord, err := GetLastTaskRecordFromAttendance(tr.UserID, tr.AttendanceID)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tr.Gap = 0
			return nil
		}
		return err
	}

	// Defensive checks (VERY important)
	if taskRecord == nil || taskRecord.EndedAt == nil {
		tr.Gap = 0
		return nil
	}

	gap := now.Sub(*taskRecord.EndedAt)
	tr.Gap = int32(gap / time.Second)

	return nil
}

type AttendanceSummary struct {
	TotalClientTime uint32
	TotalGap        uint32
	TotalDuration   uint32
}

func CalculateAttendanceSummary(attendance *models.Attendance) error {
	var total AttendanceSummary

	if err := db.DB.Model(&models.TaskRecord{}).
		Joins("JOIN tasks ON task_records.task_id = tasks.id").
		Select(`SUM(CASE 
		WHEN (tasks.project_name = ? OR tasks.project_name = ?)
		THEN 0 ELSE task_records.duration END) as total_client_time,
		SUM(gap) AS total_gap, SUM(duration) as total_duration`, "Break", "Training").
		Where("attendance_id = ?", attendance.ID).
		Scan(&total).Error; err != nil {
		return err
	}

	//TODO attendance.TotalDuration = total.TotalDuration
	attendance.ClientTime = total.TotalClientTime
	attendance.TotalGap = total.TotalGap // FIX not showing right
	return nil
}
