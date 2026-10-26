package services

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/types"
	"fesapp/backend/utils"
	"fmt"
	"strings"
	"time"
)

// GetTodaySummary returns the authenticated employee's daily duration totals,
// grouped by the exact task/project ID.
func GetTodaySummary(dateStr, userID string) ([]types.TaskSummary, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("summary requires an authenticated user")
	}

	startOfDay, endOfDay, loc, usedFallback, loadErr, parseErr := parseAppDateRange(dateStr)
	if loadErr != nil {
		utils.Logger.Printf("summary: warning timezone load failed for Asia/Manila, using fallback zone: %v", loadErr)
	}
	if parseErr != nil {
		utils.Logger.Printf("summary: failed parsing date. date=%q location=%q fallback=%t err=%v", dateStr, loc.String(), usedFallback, parseErr)
		return nil, parseErr
	}
	now := utcNow()

	utils.Logger.Printf(
		"summary: query start date=%q user_id=%q location=%q fallback=%t local_now=%s utc_now=%s start=%s end=%s start_utc=%s end_utc=%s",
		dateStr,
		userID,
		loc.String(),
		usedFallback,
		now.In(loc).Format(time.RFC3339),
		now.UTC().Format(time.RFC3339),
		startOfDay.Format(time.RFC3339),
		endOfDay.Format(time.RFC3339),
		startOfDay.UTC().Format(time.RFC3339),
		endOfDay.UTC().Format(time.RFC3339),
	)

	activitySummary := make([]types.TaskSummary, 0)
	err := db.DB.Model(&models.TaskRecord{}).
		Select(`
			task_records.task_id,
			MAX(task_records.padagan) AS padagan,
			SUM(task_records.duration) AS total_duration,
			CASE
				WHEN clients.fullname IS NOT NULL AND TRIM(clients.fullname) <> ''
					THEN clients.fullname || ' - ' || tasks.title
				ELSE tasks.title
			END AS task_name
		`).
		Joins("JOIN attendances ON attendances.id = task_records.attendance_id").
		Joins("JOIN tasks ON tasks.task_id = task_records.task_id").
		Joins("LEFT JOIN clients ON clients.id = tasks.client_id").
		Where(
			"task_records.user_id = ? AND attendances.user_id = ? AND attendances.time_in >= ? AND attendances.time_in < ?",
			userID,
			userID,
			startOfDay,
			endOfDay,
		).
		Group("task_records.task_id, clients.fullname, tasks.title").
		Scan(&activitySummary).Error
	if err != nil {
		utils.Logger.Printf("summary: aggregate query failed user_id=%q date=%q err=%v", userID, dateStr, err)
		return nil, err
	}

	utils.Logger.Printf("summary: completed user_id=%q result_count=%d date=%q", userID, len(activitySummary), dateStr)
	return activitySummary, nil
}
