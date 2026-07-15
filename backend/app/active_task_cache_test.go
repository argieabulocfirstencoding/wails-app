package app

import (
	"context"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/repository"
	"fesapp/backend/service_classes"
	"testing"
	"time"

	"gorm.io/gorm"
)

func setupActiveTaskCacheTestDB(t *testing.T) (*gorm.DB, *models.User, *models.Client, *models.Task, *models.Attendance, *models.TaskRecord) {
	t.Helper()

	testDB, err := db.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}

	originalDB := db.DB
	db.DB = testDB
	t.Cleanup(func() {
		db.DB = originalDB
	})

	user := &models.User{ID: "user-1", Email: "test@example.com"}
	if err := testDB.Create(user).Error; err != nil {
		t.Fatal(err)
	}

	client := &models.Client{ID: "client-1", Fullname: "Test Client", CompanyName: "Test Company"}
	if err := testDB.Create(client).Error; err != nil {
		t.Fatal(err)
	}

	task := &models.Task{
		TaskID:      "task-1",
		Title:       "Test Task",
		ProjectName: "Test Project",
		ClientID:    models.StringPtr(client.ID),
	}
	if err := testDB.Create(task).Error; err != nil {
		t.Fatal(err)
	}

	now := time.Now().Add(-5 * time.Minute)
	attendance := &models.Attendance{
		UserID:     user.ID,
		TimeIn:     now,
		SyncStatus: "pending",
	}
	if err := testDB.Create(attendance).Error; err != nil {
		t.Fatal(err)
	}

	taskRecord := &models.TaskRecord{
		UserID:       user.ID,
		AttendanceID: attendance.ID,
		TaskID:       task.TaskID,
		StartedAt:    now,
		Padagan:      false,
		Notes:        "",
		SyncStatus:   "pending",
	}
	if err := testDB.Create(taskRecord).Error; err != nil {
		t.Fatal(err)
	}

	return testDB, user, client, task, attendance, taskRecord
}

func TestGetActiveTaskRecordClearsStaleWorkingTaskCache(t *testing.T) {
	_, user, _, task, attendance, _ := setupActiveTaskCacheTestDB(t)

	a := NewApp()
	a.ctx = context.Background()
	a.AuthUser = &models.AuthUser{LocalID: user.ID}
	a.Attendance = attendance
	a.AuthService.TaskService.Connect(a.ctx)
	a.AuthService.TaskService.Attendance = &a.Attendance
	a.AuthService.TaskService.WorkingTaskID = &a.WorkingTaskID
	a.AuthService.TaskService.ScreenshotService = a.ScreenshotService
	a.ScreenshotService.TaskID = nil
	a.ScreenshotService.TaskRecordID = nil

	activeTaskRecord, err := a.AuthService.TaskService.GetActiveTaskRecord(user.ID)
	if err != nil {
		t.Fatalf("expected active task record, got error: %v", err)
	}
	if activeTaskRecord == nil {
		t.Fatalf("expected active task record, got nil")
	}
	if got := activeTaskRecord.TaskID; got != task.TaskID {
		t.Fatalf("expected active task id %q, got %q", task.TaskID, got)
	}

	a.WorkingTaskID = &activeTaskRecord.TaskID
	a.StopTracking(activeTaskRecord.TaskID)

	if a.WorkingTaskID != nil {
		t.Fatalf("expected WorkingTaskID to be cleared after stop, got %v", *a.WorkingTaskID)
	}

	if got := a.GetActiveTaskRecord(); got != nil {
		t.Fatalf("expected no active task after stop, got %q", *got)
	}

	if record, err := a.GetLastTaskRecord(task.TaskID); err != nil {
		t.Fatalf("GetLastTaskRecord returned error: %v", err)
	} else if record == nil || record.Record == nil {
		t.Fatalf("expected historical task record to still be readable")
	}

	if a.WorkingTaskID != nil {
		t.Fatalf("expected historical record lookup not to repopulate WorkingTaskID, got %v", *a.WorkingTaskID)
	}
}

func TestGetActiveTaskRecordReturnsNilWithoutActiveRow(t *testing.T) {
	_, user, _, _, attendance, taskRecord := setupActiveTaskCacheTestDB(t)

	if err := db.DB.Delete(taskRecord).Error; err != nil {
		t.Fatal(err)
	}

	a := NewApp()
	a.ctx = context.Background()
	a.AuthUser = &models.AuthUser{LocalID: user.ID}
	a.Attendance = attendance
	a.AuthService.TaskService.Connect(a.ctx)
	a.AuthService.TaskService.Attendance = &a.Attendance
	a.AuthService.TaskService.WorkingTaskID = &a.WorkingTaskID
	a.AuthService.TaskService.ScreenshotService = a.ScreenshotService

	if got := a.GetActiveTaskRecord(); got != nil {
		t.Fatalf("expected nil active task when no active row exists, got %q", *got)
	}

	if a.WorkingTaskID != nil {
		t.Fatalf("expected WorkingTaskID to remain nil when no active row exists, got %v", *a.WorkingTaskID)
	}
}

func TestStopTrackingClearsCacheOnMissingActiveRow(t *testing.T) {
	_, user, _, task, attendance, taskRecord := setupActiveTaskCacheTestDB(t)

	a := NewApp()
	a.ctx = context.Background()
	a.AuthUser = &models.AuthUser{LocalID: user.ID}
	a.Attendance = attendance
	a.AuthService.TaskService.Connect(a.ctx)
	a.AuthService.TaskService.Attendance = &a.Attendance
	a.AuthService.TaskService.WorkingTaskID = &a.WorkingTaskID
	a.AuthService.TaskService.ScreenshotService = a.ScreenshotService
	a.AuthService.TaskService.TaskRepository = repository.NewTaskRepository()
	a.AuthService.TaskService.SyncService = service_classes.NewSyncService(a.Connectivity)
	a.ScreenshotService.TaskID = &task.TaskID
	a.ScreenshotService.TaskRecordID = &taskRecord.ID
	a.WorkingTaskID = &task.TaskID

	if err := db.DB.Delete(taskRecord).Error; err != nil {
		t.Fatal(err)
	}

	a.StopTracking(task.TaskID)

	if a.WorkingTaskID != nil {
		t.Fatalf("expected WorkingTaskID to be cleared after failed stop, got %v", *a.WorkingTaskID)
	}
	if a.ScreenshotService.TaskID != nil {
		t.Fatalf("expected ScreenshotService.TaskID to be cleared after failed stop")
	}
	if a.ScreenshotService.TaskRecordID != nil {
		t.Fatalf("expected ScreenshotService.TaskRecordID to be cleared after failed stop")
	}
}

func TestStopTrackingOfflineKeepsARecoverableLocalRecord(t *testing.T) {
	_, user, _, task, attendance, taskRecord := setupActiveTaskCacheTestDB(t)

	a := NewApp()
	a.ctx = context.Background()
	a.AuthUser = &models.AuthUser{LocalID: user.ID}
	a.Attendance = attendance
	a.Connectivity.SetOnline(false)
	a.AuthService.TaskService.Attendance = &a.Attendance
	a.AuthService.TaskService.WorkingTaskID = &a.WorkingTaskID
	a.AuthService.TaskService.ScreenshotService = a.ScreenshotService
	a.AuthService.TaskService.TaskRepository = repository.NewTaskRepository()
	a.ScreenshotService.TaskID = &task.TaskID
	a.ScreenshotService.TaskRecordID = &taskRecord.ID
	a.WorkingTaskID = &task.TaskID

	result, err := a.StopTracking(task.TaskID)
	if err != nil {
		t.Fatalf("StopTracking returned error: %v", err)
	}
	if result == nil || result.RecordID != taskRecord.ID {
		t.Fatalf("expected stopped record result for %d, got %#v", taskRecord.ID, result)
	}
	if result.SyncStatus != "pending" || result.SyncFailureCode != "offline" {
		t.Fatalf("expected pending offline result, got %#v", result)
	}

	var stored models.TaskRecord
	if err := db.DB.First(&stored, taskRecord.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.EndedAt == nil || stored.SyncStatus != "pending" {
		t.Fatalf("expected durable pending stopped record, got %#v", stored)
	}
}

func TestHandlePendingBrowserTaskEventActivatesImportedRecord(t *testing.T) {
	_, user, _, _, attendance, localRecord := setupActiveTaskCacheTestDB(t)

	browserTask := &models.Task{TaskID: "browser-task", Title: "Browser Task", ProjectName: "Browser Project"}
	if err := db.DB.Create(browserTask).Error; err != nil {
		t.Fatal(err)
	}
	productivityID := "browser-productivity-1"
	browserRecord := &models.TaskRecord{
		UserID:       user.ID,
		AttendanceID: attendance.ID,
		TaskID:       browserTask.TaskID,
		StartedAt:    time.Now().Add(-time.Minute),
		Productivity: &productivityID,
		SyncStatus:   "synced",
	}
	if err := db.DB.Create(browserRecord).Error; err != nil {
		t.Fatal(err)
	}

	a := NewApp()
	a.ctx = context.Background()
	a.AuthUser = &models.AuthUser{LocalID: user.ID}
	a.Attendance = attendance
	a.AuthService.TaskService.Attendance = &a.Attendance
	a.AuthService.TaskService.WorkingTaskID = &a.WorkingTaskID
	a.AuthService.TaskService.ScreenshotService = a.ScreenshotService
	a.AuthService.TaskService.Connect(a.ctx)

	if err := a.handlePendingBrowserTaskEvent(service_classes.BrowserTaskEvent{ProductivityID: productivityID}); err != nil {
		t.Fatalf("handle pending browser task event: %v", err)
	}
	if a.WorkingTaskID == nil || *a.WorkingTaskID != browserTask.TaskID {
		t.Fatalf("expected browser task to be active, got %#v", a.WorkingTaskID)
	}
	if a.ScreenshotService.TaskRecordID == nil || *a.ScreenshotService.TaskRecordID != browserRecord.ID {
		t.Fatalf("expected screenshots to target browser record, got %#v", a.ScreenshotService.TaskRecordID)
	}

	var stopped models.TaskRecord
	if err := db.DB.First(&stopped, localRecord.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stopped.EndedAt == nil || stopped.SyncStatus != "pending" {
		t.Fatalf("expected local conflicting task to be stopped and pending sync, got %#v", stopped)
	}
}

func TestActivateCurrentBrowserTaskRecordActivatesManualBrowserImport(t *testing.T) {
	_, user, _, _, attendance, localRecord := setupActiveTaskCacheTestDB(t)

	browserTask := &models.Task{TaskID: "manual-browser-task", Title: "Browser Task", ProjectName: "Browser Project"}
	if err := db.DB.Create(browserTask).Error; err != nil {
		t.Fatal(err)
	}
	productivityID := "manual-browser-productivity"
	browserRecord := &models.TaskRecord{
		UserID:       user.ID,
		AttendanceID: attendance.ID,
		TaskID:       browserTask.TaskID,
		StartedAt:    time.Now().Add(-time.Minute),
		Productivity: &productivityID,
		SyncStatus:   "synced",
	}
	if err := db.DB.Create(browserRecord).Error; err != nil {
		t.Fatal(err)
	}

	a := NewApp()
	a.ctx = context.Background()
	a.AuthUser = &models.AuthUser{LocalID: user.ID}
	a.Attendance = attendance
	a.AuthService.TaskService.Attendance = &a.Attendance
	a.AuthService.TaskService.WorkingTaskID = &a.WorkingTaskID
	a.AuthService.TaskService.ScreenshotService = a.ScreenshotService
	a.AuthService.TaskService.Connect(a.ctx)

	if err := a.activateCurrentBrowserTaskRecord(); err != nil {
		t.Fatalf("activate current browser task: %v", err)
	}
	if a.WorkingTaskID == nil || *a.WorkingTaskID != browserTask.TaskID {
		t.Fatalf("expected manual browser task to be active, got %#v", a.WorkingTaskID)
	}

	var stopped models.TaskRecord
	if err := db.DB.First(&stopped, localRecord.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stopped.EndedAt == nil {
		t.Fatalf("expected conflicting local record to be stopped")
	}
}

func TestGetLastTaskRecordPrefersActiveCurrentAttendanceRecord(t *testing.T) {
	_, user, _, task, attendance, activeRecord := setupActiveTaskCacheTestDB(t)

	endedAt := time.Now().Add(-time.Minute)
	historical := &models.TaskRecord{
		UserID:       user.ID,
		AttendanceID: attendance.ID,
		TaskID:       task.TaskID,
		StartedAt:    endedAt.Add(-time.Minute),
		EndedAt:      &endedAt,
		Duration:     60,
		SyncStatus:   "synced",
	}
	if err := db.DB.Create(historical).Error; err != nil {
		t.Fatal(err)
	}

	a := NewApp()
	a.AuthUser = &models.AuthUser{LocalID: user.ID}
	a.Attendance = attendance

	response, err := a.GetLastTaskRecord(task.TaskID)
	if err != nil {
		t.Fatalf("get task record for display: %v", err)
	}
	if response == nil || response.Record == nil {
		t.Fatal("expected active task record for display")
	}
	if response.Record.ID != activeRecord.ID || response.Record.EndedAt != nil {
		t.Fatalf("expected active record %d, got %#v", activeRecord.ID, response.Record)
	}
}
