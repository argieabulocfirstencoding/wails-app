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
	if err := db.DB.Where("user_id = ? AND ended_at IS NULL", userId).First(&taskRecord).Error; err != nil {
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
		return fmt.Errorf("db error: %w", err)
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
