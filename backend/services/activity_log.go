package services

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/utils"
	"strings"
	"time"
)

func GetAttendanceAndRecords(dateStr, userID string) ([]models.ActivityLogRecord, error) {
	startOfDay, endOfDay, loc, usedFallback, loadErr, parseErr := parseAppDateRange(dateStr)
	if loadErr != nil {
		utils.Logger.Printf("activity_log: warning timezone load failed for Asia/Manila, using fallback zone: %v", loadErr)
	}

	now := utcNow()
	if parseErr != nil {
		utils.Logger.Printf("activity_log: failed parsing date. date=%q location=%q fallback=%t err=%v", dateStr, loc.String(), usedFallback, parseErr)
		return nil, parseErr
	}

	utils.Logger.Printf(
		"activity_log: query start date=%q location=%q fallback=%t local_now=%s utc_now=%s start=%s end=%s start_utc=%s end_utc=%s",
		dateStr,
		loc.String(),
		usedFallback,
		now.In(loc).Format(time.RFC3339),
		now.UTC().Format(time.RFC3339),
		startOfDay.Format(time.RFC3339),
		endOfDay.Format(time.RFC3339),
		startOfDay.UTC().Format(time.RFC3339),
		endOfDay.UTC().Format(time.RFC3339),
	)

	var foundAttendance []models.Attendance
	attendanceQuery := db.DB
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID != "" {
		attendanceQuery = attendanceQuery.Where("user_id = ?", trimmedUserID)
	}

	if err := attendanceQuery.Where("time_in >= ? AND time_in < ?", startOfDay, endOfDay).Find(&foundAttendance).Error; err != nil {
		utils.Logger.Printf("activity_log: attendance query failed start=%s end=%s err=%v", startOfDay.Format(time.RFC3339), endOfDay.Format(time.RFC3339), err)
		return nil, err
	}

	utils.Logger.Printf("activity_log: attendance query matched count=%d", len(foundAttendance))
	if len(foundAttendance) == 0 {
		fallbackQuery := db.DB
		if trimmedUserID != "" {
			fallbackQuery = fallbackQuery.Where("user_id = ?", trimmedUserID)
		}

		if err := fallbackQuery.Where("date(time_in) = ? OR date(time_in, '+8 hours') = ?", dateStr, dateStr).Find(&foundAttendance).Error; err != nil {
			utils.Logger.Printf("activity_log: fallback attendance query failed date=%q err=%v", dateStr, err)
			return nil, err
		}
		utils.Logger.Printf("activity_log: fallback attendance query matched count=%d date=%q", len(foundAttendance), dateStr)
	}

	activityLogRecords := make([]models.ActivityLogRecord, 0, len(foundAttendance))

	for _, attendance := range foundAttendance {
		var records []models.TaskRecord

		if err := db.DB.Preload("Task.Client").Where("attendance_id = ?", attendance.ID).Order("started_at ASC").Find(&records).Error; err != nil {
			utils.Logger.Printf("activity_log: error fetching records for attendance_id=%d err=%v", attendance.ID, err)
			continue
		}
		if err := decorateTaskRecordsWithPendingScreenshotCounts(records); err != nil {
			utils.Logger.Printf("activity_log: failed counting pending screenshots for attendance_id=%d err=%v", attendance.ID, err)
		}
		for i := range records {
			records[i].SetTaskLabel()
		}

		timeOutStr := "<nil>"
		if attendance.TimeOut != nil {
			timeOutStr = attendance.TimeOut.Format(time.RFC3339)
		}

		utils.Logger.Printf(
			"activity_log: attendance_id=%d user_id=%s time_in=%s time_out=%s records=%d",
			attendance.ID,
			attendance.UserID,
			attendance.TimeIn.Format(time.RFC3339),
			timeOutStr,
			len(records),
		)

		activityLogRecords = append(activityLogRecords, models.ActivityLogRecord{
			Attendance: attendance,
			Records:    records,
		})
	}

	utils.Logger.Printf("activity_log: completed result_count=%d date=%q", len(activityLogRecords), dateStr)

	return activityLogRecords, nil
}

type pendingScreenshotCountRow struct {
	TaskRecordID uint
	Total        int64
}

func decorateTaskRecordsWithPendingScreenshotCounts(records []models.TaskRecord) error {
	if !backblazeEnabled || len(records) == 0 {
		return nil
	}

	recordIDs := make([]uint, 0, len(records))
	for i := range records {
		recordIDs = append(recordIDs, records[i].ID)
	}

	var rows []pendingScreenshotCountRow
	if err := db.DB.
		Model(&models.Screenshot{}).
		Select("task_record_id, COUNT(*) AS total").
		Where("task_record_id IN ? AND (cloud_url IS NULL OR TRIM(cloud_url) = '')", recordIDs).
		Group("task_record_id").
		Scan(&rows).Error; err != nil {
		return err
	}

	counts := make(map[uint]int, len(rows))
	for _, row := range rows {
		counts[row.TaskRecordID] = int(row.Total)
	}

	for i := range records {
		records[i].PendingScreenshotCount = counts[records[i].ID]
	}

	return nil
}
