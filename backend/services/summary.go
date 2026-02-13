package services

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/types"
	"fmt"
	"time"
)

// GetTodaySummary retrieves a summarized view of tasks for a specific attendance record. It aggregates task durations by TaskID and Padagan status, while also including the task’s descriptive name in the format "Client Fullname - Task Title".
func GetTodaySummary(dateStr string) ([]types.TaskSummary, error) {
	// TODO refactor this with activity_log.go since they share this feature
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

	activitySummary := make([]types.TaskSummary, 0, len(foundAttendance))

	for _, attendance := range foundAttendance {
		var results []types.TaskSummary

		err := db.DB.Model(&models.TaskRecord{}).
			Select(`
			task_records.task_id,
			task_records.padagan,
			SUM(task_records.duration) as total_duration,
			CONCAT(clients.fullname, ' - ', tasks.title) as task_name
		`).
			Joins("JOIN tasks ON tasks.task_id = task_records.task_id").
			Joins("JOIN clients ON clients.id = tasks.client_id").
			Where("task_records.duration IS NOT NULL AND task_records.attendance_id = ?", attendance.ID).
			Group(`
			task_records.task_id,
			task_records.padagan,
			clients.fullname,
			tasks.title
		`).
			Scan(&results).Error

		if err != nil {
			return nil, err
		}

		activitySummary = append(activitySummary, results...)
	}

	return activitySummary, nil
}
