package service_classes

import (
	"sync"
	"testing"
	"time"

	"fesapp/backend/db"
	"fesapp/backend/db/models"
)

func setupTaskLifecycleService(t *testing.T) (*TaskService, *models.Attendance, *models.Task, *models.Task) {
	t.Helper()

	testDB, err := db.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}
	originalDB := db.DB
	db.DB = testDB
	t.Cleanup(func() { db.DB = originalDB })

	user := &models.User{ID: "user-1", Email: "user@example.com"}
	if err := testDB.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	client := &models.Client{ID: "client-1", Fullname: "Test Client", CompanyName: "Test Company"}
	if err := testDB.Create(client).Error; err != nil {
		t.Fatal(err)
	}
	attendance := &models.Attendance{UserID: user.ID, TimeIn: time.Now().Add(-time.Hour), SyncStatus: syncStatePending}
	if err := testDB.Create(attendance).Error; err != nil {
		t.Fatal(err)
	}
	taskOne := &models.Task{TaskID: "task-1", Title: "Task One", ProjectName: "Project", ClientID: models.StringPtr(client.ID)}
	taskTwo := &models.Task{TaskID: "task-2", Title: "Task Two", ProjectName: "Project", ClientID: models.StringPtr(client.ID)}
	if err := testDB.Create(taskOne).Error; err != nil {
		t.Fatal(err)
	}
	if err := testDB.Create(taskTwo).Error; err != nil {
		t.Fatal(err)
	}

	connectivity := NewConnectivityStateService()
	connectivity.SetOnline(false)
	syncService := NewSyncService(connectivity)
	service := NewTaskService(syncService, connectivity)
	attendancePointer := attendance
	service.Attendance = &attendancePointer
	var workingTaskID *string
	service.WorkingTaskID = &workingTaskID
	return service, attendance, taskOne, taskTwo
}

func TestStartTrackingReusesCurrentTaskRecord(t *testing.T) {
	service, attendance, task, _ := setupTaskLifecycleService(t)

	if err := service.StartTracking(task.TaskID, "first", true); err != nil {
		t.Fatalf("first start: %v", err)
	}
	var first models.TaskRecord
	if err := db.DB.Where("user_id = ? AND attendance_id = ? AND task_id = ?", attendance.UserID, attendance.ID, task.TaskID).First(&first).Error; err != nil {
		t.Fatal(err)
	}

	if err := service.StartTracking(task.TaskID, "second", false); err != nil {
		t.Fatalf("repeated start: %v", err)
	}
	var records []models.TaskRecord
	if err := db.DB.Where("user_id = ? AND attendance_id = ? AND task_id = ?", attendance.UserID, attendance.ID, task.TaskID).Find(&records).Error; err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one record after repeated start, got %d", len(records))
	}
	if records[0].ID != first.ID || !records[0].StartedAt.Equal(first.StartedAt) || !records[0].Padagan || records[0].EndedAt != nil {
		t.Fatalf("repeated start changed the active record: %#v", records[0])
	}
}

func TestConcurrentStartsCreateOneRecord(t *testing.T) {
	service, attendance, task, _ := setupTaskLifecycleService(t)

	var group sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			errs <- service.StartTracking(task.TaskID, "start", false)
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent start: %v", err)
		}
	}

	var records []models.TaskRecord
	if err := db.DB.Where("user_id = ? AND attendance_id = ? AND task_id = ?", attendance.UserID, attendance.ID, task.TaskID).Find(&records).Error; err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].EndedAt != nil {
		t.Fatalf("expected one active record, got %#v", records)
	}
}

func TestStartTrackingSwitchesTasksWithoutLeavingTwoActiveRecords(t *testing.T) {
	service, attendance, taskOne, taskTwo := setupTaskLifecycleService(t)
	if err := service.StartTracking(taskOne.TaskID, "one", false); err != nil {
		t.Fatal(err)
	}
	if err := service.StartTracking(taskTwo.TaskID, "two", false); err != nil {
		t.Fatal(err)
	}

	var records []models.TaskRecord
	if err := db.DB.Where("user_id = ? AND attendance_id = ?", attendance.UserID, attendance.ID).Order("id ASC").Find(&records).Error; err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[0].TaskID != taskOne.TaskID || records[0].EndedAt == nil || records[1].TaskID != taskTwo.TaskID || records[1].EndedAt != nil {
		t.Fatalf("unexpected records after task switch: %#v", records)
	}
}

func TestStartTrackingRefusesToModifyExistingDuplicateActiveRecords(t *testing.T) {
	service, attendance, taskOne, taskTwo := setupTaskLifecycleService(t)
	now := time.Now().Add(-time.Minute)
	for _, taskID := range []string{taskOne.TaskID, taskTwo.TaskID} {
		record := &models.TaskRecord{
			UserID:       attendance.UserID,
			AttendanceID: attendance.ID,
			TaskID:       taskID,
			StartedAt:    now,
			SyncStatus:   syncStatePending,
		}
		if err := db.DB.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := service.StartTracking(taskOne.TaskID, "start", false); err == nil {
		t.Fatal("expected start to reject multiple active records")
	}
	var activeCount int64
	if err := db.DB.Model(&models.TaskRecord{}).
		Where("user_id = ? AND attendance_id = ? AND ended_at IS NULL", attendance.UserID, attendance.ID).
		Count(&activeCount).Error; err != nil {
		t.Fatal(err)
	}
	if activeCount != 2 {
		t.Fatalf("start modified pre-existing duplicate records; active count = %d", activeCount)
	}
}

func TestStopTrackingRecordIsIdempotent(t *testing.T) {
	service, attendance, task, _ := setupTaskLifecycleService(t)
	if err := service.StartTracking(task.TaskID, "start", false); err != nil {
		t.Fatal(err)
	}
	var record models.TaskRecord
	if err := db.DB.Where("user_id = ? AND attendance_id = ? AND task_id = ?", attendance.UserID, attendance.ID, task.TaskID).First(&record).Error; err != nil {
		t.Fatal(err)
	}

	first, err := service.StopTrackingRecord(record.ID)
	if err != nil {
		t.Fatalf("first stop: %v", err)
	}
	var stopped models.TaskRecord
	if err := db.DB.First(&stopped, record.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stopped.EndedAt == nil {
		t.Fatal("expected first stop to end the record")
	}
	firstEnd := *stopped.EndedAt

	second, err := service.StopTrackingRecord(record.ID)
	if err != nil {
		t.Fatalf("repeated stop: %v", err)
	}
	if first.RecordID != second.RecordID || first.SyncStatus != second.SyncStatus {
		t.Fatalf("repeated stop returned a different result: first=%#v second=%#v", first, second)
	}
	if err := db.DB.First(&stopped, record.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stopped.EndedAt == nil || !stopped.EndedAt.Equal(firstEnd) {
		t.Fatalf("repeated stop changed end time: got %v want %v", stopped.EndedAt, firstEnd)
	}
}
