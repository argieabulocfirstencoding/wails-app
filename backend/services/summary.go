package services

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/types"
	"fesapp/backend/utils"
	"time"
)

// GetTodaySummary retrieves a summarized view of tasks for a specific attendance record. It aggregates task durations by TaskID and Padagan status, while also including the task’s descriptive name in the format "Client Fullname - Task Title".
func GetTodaySummary(dateStr string) ([]types.TaskSummary, error) {
	loc, usedFallback, loadErr := resolveManilaLocation()
	if loadErr != nil {
		utils.Logger.Printf("summary: warning timezone load failed for Asia/Manila, using fallback zone: %v", loadErr)
	}

	date, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		utils.Logger.Printf("summary: failed parsing date. date=%q location=%q fallback=%t err=%v", dateStr, loc.String(), usedFallback, err)
		return nil, err
	}

	startOfDay := date
	endOfDay := date.Add(24 * time.Hour)
	now := time.Now()

	utils.Logger.Printf(
		"summary: query start date=%q location=%q fallback=%t local_now=%s utc_now=%s start=%s end=%s start_utc=%s end_utc=%s",
		dateStr,
		loc.String(),
		usedFallback,
		now.Format(time.RFC3339),
		now.UTC().Format(time.RFC3339),
		startOfDay.Format(time.RFC3339),
		endOfDay.Format(time.RFC3339),
		startOfDay.UTC().Format(time.RFC3339),
		endOfDay.UTC().Format(time.RFC3339),
	)

	var foundAttendance []models.Attendance
	if err := db.DB.Where("time_in >= ? AND time_in < ?", startOfDay, endOfDay).Find(&foundAttendance).Error; err != nil {
		utils.Logger.Printf("summary: attendance query failed start=%s end=%s err=%v", startOfDay.Format(time.RFC3339), endOfDay.Format(time.RFC3339), err)
		return nil, err
	}

	utils.Logger.Printf("summary: attendance query matched count=%d", len(foundAttendance))

	if len(foundAttendance) == 0 {
		if err := db.DB.Where("date(time_in) = ? OR date(time_in, 'localtime') = ? OR date(time_in, '+8 hours') = ?", dateStr, dateStr, dateStr).Find(&foundAttendance).Error; err != nil {
			utils.Logger.Printf("summary: fallback attendance query failed date=%q err=%v", dateStr, err)
			return nil, err
		}
		utils.Logger.Printf("summary: fallback attendance query matched count=%d date=%q", len(foundAttendance), dateStr)
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
			utils.Logger.Printf("summary: aggregate query failed attendance_id=%d err=%v", attendance.ID, err)
			return nil, err
		}

		utils.Logger.Printf("summary: attendance_id=%d task_summary_count=%d", attendance.ID, len(results))
		activitySummary = append(activitySummary, results...)
	}

	utils.Logger.Printf("summary: completed result_count=%d date=%q", len(activitySummary), dateStr)
	return activitySummary, nil
}
