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

func CalculateAttendanceSummary(attendance *models.Attendance) error {

	var total struct {
		TotalClientTime uint32 `gorm:"column:total_client_time"`
		TotalGap        uint32 `gorm:"column:total_gap"`
		TotalDuration   uint32 `gorm:"column:total_duration"`
	}

	if err := db.DB.Model(&models.TaskRecord{}).
		Joins("JOIN tasks ON task_records.task_id = tasks.task_id").
		Select(`
			COALESCE(SUM(CASE 
				WHEN tasks.project_name IN (?, ?)
				THEN 0 ELSE task_records.duration 
			END),0) AS total_client_time,
			
			COALESCE(SUM(CASE 
				WHEN tasks.project_name IN (?, ?)
				THEN 0 ELSE task_records.gap
			END),0) AS total_gap,
			
			COALESCE(SUM(task_records.duration),0) AS total_duration
		`, "Break", "Training", "Break", "Training").
		Where("attendance_id = ?", attendance.ID).
		Scan(&total).Error; err != nil {
		return err
	}

	//TODO attendance.TotalDuration = total.TotalDuration
	attendance.ClientTime = total.TotalClientTime
	attendance.TotalGap = total.TotalGap

	err := db.DB.Save(attendance).Error
	if err != nil {
		return err
	}

	return nil
}
