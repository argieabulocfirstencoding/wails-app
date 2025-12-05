package services

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/utils"
	"fmt"
	"time"
)

func GetAttendanceAndRecords(dateStr string) ([]models.ActivityLogRecord, error) {
	loc, err := time.LoadLocation("Asia/Manila") // GMT+8
	if err != nil {
		return nil, err
	}

	// Parse the date string in the GMT+8 timezone
	date, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		return nil, err
	}

	startOfDay := date
	fmt.Printf("Start of Day: %v", startOfDay)
	endOfDay := date.Add(24 * time.Hour)
	fmt.Printf("End of Day: %v", endOfDay)

	var foundAttendance []models.Attendance
	if err := db.DB.Where("time_in >= ? AND time_in < ?", startOfDay, endOfDay).Find(&foundAttendance).Error; err != nil {
		return nil, err
	}

	activityLogRecords := make([]models.ActivityLogRecord, 0, len(foundAttendance))

	for _, attendance := range foundAttendance {
		var records []models.TaskRecord

		if err := db.DB.Where("attendance_id = ?", attendance.ID).Find(&records).Error; err != nil {
			utils.Logger.Println("error fetching attendance records for activity log")
		}

		activityLogRecords = append(activityLogRecords, models.ActivityLogRecord{
			Attendance: attendance,
			Records:    records,
		})
	}

	return activityLogRecords, nil
}
