package services

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/utils"
	"time"
)

func GetAttendanceAndRecords(dateStr string) ([]models.ActivityLogRecord, error) {
	loc, usedFallback, loadErr := resolveManilaLocation()
	if loadErr != nil {
		utils.Logger.Printf("activity_log: warning timezone load failed for Asia/Manila, using fallback zone: %v", loadErr)
	}

	// Parse the date string in the GMT+8 timezone
	date, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		utils.Logger.Printf("activity_log: failed parsing date. date=%q location=%q fallback=%t err=%v", dateStr, loc.String(), usedFallback, err)
		return nil, err
	}

	startOfDay := date
	endOfDay := date.Add(24 * time.Hour)
	now := time.Now()

	utils.Logger.Printf(
		"activity_log: query start date=%q location=%q fallback=%t local_now=%s utc_now=%s start=%s end=%s start_utc=%s end_utc=%s",
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
		utils.Logger.Printf("activity_log: attendance query failed start=%s end=%s err=%v", startOfDay.Format(time.RFC3339), endOfDay.Format(time.RFC3339), err)
		return nil, err
	}

	utils.Logger.Printf("activity_log: attendance query matched count=%d", len(foundAttendance))
	if len(foundAttendance) == 0 {
		if err := db.DB.Where("date(time_in) = ? OR date(time_in, 'localtime') = ? OR date(time_in, '+8 hours') = ?", dateStr, dateStr, dateStr).Find(&foundAttendance).Error; err != nil {
			utils.Logger.Printf("activity_log: fallback attendance query failed date=%q err=%v", dateStr, err)
			return nil, err
		}
		utils.Logger.Printf("activity_log: fallback attendance query matched count=%d date=%q", len(foundAttendance), dateStr)
	}

	activityLogRecords := make([]models.ActivityLogRecord, 0, len(foundAttendance))

	for _, attendance := range foundAttendance {
		var records []models.TaskRecord

		if err := db.DB.Where("attendance_id = ?", attendance.ID).Find(&records).Error; err != nil {
			utils.Logger.Printf("activity_log: error fetching records for attendance_id=%d err=%v", attendance.ID, err)
			continue
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

func resolveManilaLocation() (*time.Location, bool, error) {
	loc, err := time.LoadLocation("Asia/Manila")
	if err == nil {
		return loc, false, nil
	}

	// Keep activity logs functional even when IANA tz data is not available on the machine.
	return time.FixedZone("Asia/Manila_Fallback", 8*60*60), true, err
}
