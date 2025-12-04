package services

import (
	"errors"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fmt"
	"time"

	"gorm.io/gorm"
)

func GetActiveTaskRecord(userId string) (*models.ExpTaskRecord, error) {
	// TODO firebase fetch
	var taskRecord models.TaskRecord

	attendance, err := GetUserAttendance(userId)
	if err != nil {
		return nil, err
	}

	// Only get the active task record from the current attendance
	if err := db.DB.Where("user_id = ? AND attendance_id = ? AND ended_at IS NULL", userId, attendance.ID).First(&taskRecord).Error; err != nil {
		return nil, err
	}

	expTaskRecord := taskRecord.ToExp()
	return &expTaskRecord, nil
}

func GetLastTaskRecord(userId, taskId string) (*models.ExpTaskRecord, error) {
	// TODO add fetch on firebase
	var taskRecord models.TaskRecord
	if err := db.DB.Where("user_id = ? AND task_id = ?", userId, taskId).Last(&taskRecord).Error; err != nil {
		fmt.Println("DB error:", err)
		return nil, err
	}

	taskRecordExp := taskRecord.ToExp()
	return &taskRecordExp, nil
}

func GetTaskRecordFromID(taskRecordId uint) (*models.TaskRecord, error) {
	var taskRecord models.TaskRecord
	if err := db.DB.Where("task_records.id = ?", taskRecordId).First(&taskRecord).Error; err != nil {
		fmt.Println("DB error:", err)
		return nil, err
	}

	return &taskRecord, nil
}

func GetAllPreviousTasksRecords(userID string) ([]models.TaskRecord, error) {
	var taskRecords []models.TaskRecord

	err := db.DB.
		Where("user_id = ?", userID). // filter by user
		Order("started_at DESC").     // order by recent tasks
		Group("task_id").             // make unique based on task_id
		Find(&taskRecords).Error      // fetch all matching records

	if err != nil {
		fmt.Println("DB error:", err)
		return nil, err
	}

	return taskRecords, nil
}

func StartTaskRecord(userId, taskId string, attendanceId uint) (*db.TaskRecord, error) {
	var taskRecord models.TaskRecord
	if err := db.DB.Where("user_id = ? AND ended_at IS NULL", userId).First(&taskRecord).Error; err != nil {
		fmt.Print(err)
	} else {
		if err := UpdateRecord(&taskRecord); err != nil {
			return nil, err
		}

		// TODO add edge cases where forgot to logout due to power problem
	}

	record, err := NewTaskRecord(userId, taskId, attendanceId)
	return record, err
}
func StopTaskRecord(userId, taskId string) error {
	var taskRecord models.TaskRecord

	err := db.DB.Where("user_id = ? AND task_id = ? AND ended_at IS NULL", userId, taskId).
		First(&taskRecord).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		fmt.Printf("no active task found for user %s task %s", userId, taskId)
	}
	if err != nil {
		return err
	}

	// ✅ only call UpdateRecord if we really have a record
	if taskRecord.ID == 0 {
		return fmt.Errorf("invalid taskRecord, no ID")
	}

	if err := UpdateRecord(&taskRecord); err != nil {
		return fmt.Errorf("failed to update record: %w", err)
	}

	fmt.Println("STOPPED TASK RECORD")
	return nil
}

func NewTaskRecord(userId, taskId string, attendanceId uint) (*models.TaskRecord, error) {
	record := models.TaskRecord{
		UserID:       userId,
		AttendanceID: attendanceId,
		TaskID:       taskId,
	}

	err := db.DB.Create(&record).Error
	if err != nil {
		return nil, err
	}

	return &record, nil
}

func UpdateRecord(tr *models.TaskRecord) error {

	now := time.Now()
	tr.EndedAt = &now

	if !tr.StartedAt.IsZero() {
		duration := int32(now.Sub(tr.StartedAt).Seconds())
		tr.Duration = &duration
	}

	// This part updates the database
	return db.DB.Model(tr).Updates(models.TaskRecord{
		EndedAt:  tr.EndedAt,
		Duration: tr.Duration,
	}).Error
}

// Fetch total hours worked on a specific task in a day
func GetAttendanceTaskTotalHours(userId, taskId string, attendanceId uint) (*int32, error) {
	var totalDuration int32
	if err := db.DB.Model(&models.TaskRecord{}).Select("SUM(duration)").Where("user_id = ? AND task_id = ? AND attendance_id = ?", userId, taskId, attendanceId).Scan(&totalDuration).Error; err != nil {
		return nil, err
	}

	return &totalDuration, nil
}

// Fetch total hours worked on a specific task in a week
func GetAttendanceTaskTotalHoursWeek(userId, taskId string) (*int32, error) {
	var totalDuration int32

	monday, sunday := weekRange(time.Now())

	if err := db.DB.Model(&models.TaskRecord{}).Select("SUM(duration)").Where("user_id = ? AND task_id = ? AND started_at >= ? AND started_at <= ?", userId, taskId, monday, sunday).Scan(&totalDuration).Error; err != nil {
		return nil, err
	}

	return &totalDuration, nil
}

func weekRange(now time.Time) (monday, sunday time.Time) {
	// Ensure we’re working with the same location
	weekday := int(now.Weekday())
	if weekday == 0 {
		// Go treats Sunday as 0, so adjust to 7
		weekday = 7
	}
	// Find Monday (start of week)
	monday = now.AddDate(0, 0, -(weekday - 1))
	monday = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, monday.Location())
	// Find Sunday (end of week)
	sunday = now.AddDate(0, 0, 6)
	sunday = time.Date(sunday.Year(), sunday.Month(), sunday.Day(), 23, 59, 59, 0, sunday.Location())

	return monday, sunday
}
