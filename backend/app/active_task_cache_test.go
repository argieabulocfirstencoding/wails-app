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

func TestShutdownKeepsActiveTaskRecordOpenAndTimerContinuous(t *testing.T) {
	_, user, _, _, attendance, activeRecord := setupActiveTaskCacheTestDB(t)

	a := NewApp()
	a.ctx = context.Background()
	a.AuthUser = &models.AuthUser{LocalID: user.ID}
	a.Attendance = attendance
	a.AuthService.TaskService.Attendance = &a.Attendance

	// Shutdown accepts a nil lifecycle context in this unit test. A real Wails
	// lifecycle context is only needed for runtime logging, not persistence.
	a.Shutdown(nil)

	var persisted models.TaskRecord
	if err := db.DB.First(&persisted, activeRecord.ID).Error; err != nil {
		t.Fatalf("load active task after shutdown: %v", err)
	}
	if persisted.EndedAt != nil {
		t.Fatalf("shutdown must not write task-out; got ended_at %v", persisted.EndedAt)
	}

	summary, err := a.GetCurrentShiftTime()
	if err != nil {
		t.Fatalf("get current shift time after shutdown: %v", err)
	}
	if summary.CurrentSessionStartedAt == nil || !summary.CurrentSessionStartedAt.Equal(activeRecord.StartedAt) {
		t.Fatalf("timer start = %v, want persisted start %v", summary.CurrentSessionStartedAt, activeRecord.StartedAt)
	}
	if summary.CompletedShiftSeconds != 0 {
		t.Fatalf("completed shift seconds = %d, want 0 for the still-running record", summary.CompletedShiftSeconds)
	}
	if elapsed := time.Since(*summary.CurrentSessionStartedAt); elapsed < 4*time.Minute {
		t.Fatalf("expected timer to continue from the original start; elapsed = %v", elapsed)
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

func TestGetTaskShiftTimeUsesOnlyCurrentAttendance(t *testing.T) {
	_, user, client, task, attendance, activeRecord := setupActiveTaskCacheTestDB(t)

	endedAt := time.Now().Add(-time.Minute)
	currentCompleted := &models.TaskRecord{
		UserID:       user.ID,
		AttendanceID: attendance.ID,
		TaskID:       task.TaskID,
		StartedAt:    endedAt.Add(-2 * time.Minute),
		EndedAt:      &endedAt,
		Duration:     120,
		SyncStatus:   "synced",
	}
	if err := db.DB.Create(currentCompleted).Error; err != nil {
		t.Fatal(err)
	}
	otherTask := &models.Task{
		TaskID:      "task-2",
		Title:       "Another Task",
		ProjectName: "Another Project",
		ClientID:    models.StringPtr(client.ID),
	}
	if err := db.DB.Create(otherTask).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(&models.TaskRecord{
		UserID:       user.ID,
		AttendanceID: attendance.ID,
		TaskID:       otherTask.TaskID,
		StartedAt:    endedAt.Add(-5 * time.Minute),
		EndedAt:      &endedAt,
		Duration:     300,
		SyncStatus:   "synced",
	}).Error; err != nil {
		t.Fatal(err)
	}

	previousAttendance := &models.Attendance{
		UserID:     user.ID,
		TimeIn:     time.Now().Add(-24 * time.Hour),
		SyncStatus: "synced",
	}
	if err := db.DB.Create(previousAttendance).Error; err != nil {
		t.Fatal(err)
	}
	previousEndedAt := time.Now().Add(-23 * time.Hour)
	if err := db.DB.Create(&models.TaskRecord{
		UserID:       user.ID,
		AttendanceID: previousAttendance.ID,
		TaskID:       task.TaskID,
		StartedAt:    previousEndedAt.Add(-time.Hour),
		EndedAt:      &previousEndedAt,
		Duration:     3600,
		SyncStatus:   "synced",
	}).Error; err != nil {
		t.Fatal(err)
	}
	otherUser := &models.User{ID: "other-user", Email: "other@example.com"}
	if err := db.DB.Create(otherUser).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(&models.TaskRecord{
		UserID:       otherUser.ID,
		AttendanceID: attendance.ID,
		TaskID:       task.TaskID,
		StartedAt:    endedAt.Add(-time.Hour),
		EndedAt:      &endedAt,
		Duration:     1800,
		SyncStatus:   "synced",
	}).Error; err != nil {
		t.Fatal(err)
	}

	a := NewApp()
	a.AuthUser = &models.AuthUser{LocalID: user.ID}
	a.Attendance = attendance

	summary, err := a.GetTaskShiftTime(task.TaskID)
	if err != nil {
		t.Fatalf("GetTaskShiftTime returned error: %v", err)
	}
	if summary.CompletedShiftSeconds != 120 {
		t.Fatalf("completed shift seconds = %d, want 120", summary.CompletedShiftSeconds)
	}
	if summary.CurrentSessionStartedAt == nil || !summary.CurrentSessionStartedAt.Equal(activeRecord.StartedAt) {
		t.Fatalf("current session start = %v, want %v", summary.CurrentSessionStartedAt, activeRecord.StartedAt)
	}

	shiftSummary, err := a.GetCurrentShiftTime()
	if err != nil {
		t.Fatalf("GetCurrentShiftTime returned error: %v", err)
	}
	if shiftSummary.CompletedShiftSeconds != 420 {
		t.Fatalf("all-project completed shift seconds = %d, want 420", shiftSummary.CompletedShiftSeconds)
	}
	if shiftSummary.CurrentSessionStartedAt == nil || !shiftSummary.CurrentSessionStartedAt.Equal(activeRecord.StartedAt) {
		t.Fatalf("all-project current session start = %v, want %v", shiftSummary.CurrentSessionStartedAt, activeRecord.StartedAt)
	}
}

func TestGetTaskShiftTimeReturnsZeroForUnusedTaskInCurrentAttendance(t *testing.T) {
	_, user, _, task, _, _ := setupActiveTaskCacheTestDB(t)

	newAttendance := &models.Attendance{
		UserID:     user.ID,
		TimeIn:     time.Now(),
		SyncStatus: "pending",
	}
	if err := db.DB.Create(newAttendance).Error; err != nil {
		t.Fatal(err)
	}

	a := NewApp()
	a.AuthUser = &models.AuthUser{LocalID: user.ID}
	a.Attendance = newAttendance

	summary, err := a.GetTaskShiftTime(task.TaskID)
	if err != nil {
		t.Fatalf("GetTaskShiftTime returned error: %v", err)
	}
	if summary.CompletedShiftSeconds != 0 || summary.CurrentSessionStartedAt != nil {
		t.Fatalf("expected zero timer state, got %#v", summary)
	}
}

func TestStaleOpenTaskRecordsExcludeCurrentAttendanceAndOtherUsers(t *testing.T) {
	_, user, _, task, attendance, _ := setupActiveTaskCacheTestDB(t)

	previousAttendance := &models.Attendance{
		UserID:     user.ID,
		TimeIn:     time.Now().Add(-24 * time.Hour),
		SyncStatus: "synced",
	}
	if err := db.DB.Create(previousAttendance).Error; err != nil {
		t.Fatal(err)
	}
	staleStartedAt := time.Now().Add(-23 * time.Hour).Truncate(time.Second)
	stale := &models.TaskRecord{
		UserID:       user.ID,
		AttendanceID: previousAttendance.ID,
		TaskID:       task.TaskID,
		StartedAt:    staleStartedAt,
		SyncStatus:   "pending",
	}
	if err := db.DB.Create(stale).Error; err != nil {
		t.Fatal(err)
	}
	latestScreenshot := staleStartedAt.Add(15 * time.Minute)
	if err := db.DB.Create(&models.Screenshot{
		TaskRecordID: stale.ID,
		TaskID:       task.TaskID,
		TakenAt:      latestScreenshot,
	}).Error; err != nil {
		t.Fatal(err)
	}

	otherUser := &models.User{ID: "other-user", Email: "other@example.com"}
	if err := db.DB.Create(otherUser).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(&models.TaskRecord{
		UserID:       otherUser.ID,
		AttendanceID: previousAttendance.ID,
		TaskID:       task.TaskID,
		StartedAt:    staleStartedAt,
		SyncStatus:   "pending",
	}).Error; err != nil {
		t.Fatal(err)
	}

	a := NewApp()
	a.AuthUser = &models.AuthUser{LocalID: user.ID}
	a.Attendance = attendance

	records, err := a.GetStaleOpenTaskRecords()
	if err != nil {
		t.Fatalf("GetStaleOpenTaskRecords returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one stale record, got %#v", records)
	}
	if got := records[0]; got.ID != stale.ID || got.TaskTitle != task.Title || got.AttendanceID != previousAttendance.ID {
		t.Fatalf("unexpected stale record: %#v", got)
	}
	if records[0].SuggestedEndedAt == nil || !records[0].SuggestedEndedAt.Equal(latestScreenshot) {
		t.Fatalf("suggested task-out = %v, want %v", records[0].SuggestedEndedAt, latestScreenshot)
	}
}

func TestResolveStaleOpenTaskRecordClosesOnlySelectedPreviousRecord(t *testing.T) {
	_, user, _, task, attendance, currentRecord := setupActiveTaskCacheTestDB(t)

	previousAttendance := &models.Attendance{
		UserID:     user.ID,
		TimeIn:     time.Now().Add(-24 * time.Hour),
		SyncStatus: "synced",
	}
	if err := db.DB.Create(previousAttendance).Error; err != nil {
		t.Fatal(err)
	}
	staleStartedAt := time.Now().Add(-23 * time.Hour).Truncate(time.Second)
	stale := &models.TaskRecord{
		UserID:       user.ID,
		AttendanceID: previousAttendance.ID,
		TaskID:       task.TaskID,
		StartedAt:    staleStartedAt,
		SyncStatus:   "synced",
	}
	if err := db.DB.Create(stale).Error; err != nil {
		t.Fatal(err)
	}
	endedAt := staleStartedAt.Add(45 * time.Minute).UTC()

	a := NewApp()
	a.AuthUser = &models.AuthUser{LocalID: user.ID}
	a.Attendance = attendance
	a.AuthService.TaskService.Attendance = &a.Attendance

	result, err := a.ResolveStaleOpenTaskRecord(stale.ID, endedAt.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("ResolveStaleOpenTaskRecord returned error: %v", err)
	}
	if result == nil || result.RecordID != stale.ID || result.SyncStatus != "pending" {
		t.Fatalf("unexpected resolve result: %#v", result)
	}

	var resolved models.TaskRecord
	if err := db.DB.First(&resolved, stale.ID).Error; err != nil {
		t.Fatal(err)
	}
	if resolved.EndedAt == nil || !resolved.EndedAt.Equal(endedAt) || resolved.Duration != int32((45*time.Minute).Seconds()) {
		t.Fatalf("stale record was not closed as requested: %#v", resolved)
	}
	if resolved.SyncStatus != "pending" {
		t.Fatalf("resolved sync status = %q, want pending", resolved.SyncStatus)
	}

	var current models.TaskRecord
	if err := db.DB.First(&current, currentRecord.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.EndedAt != nil {
		t.Fatalf("current record must remain active, got %#v", current)
	}
}

func TestResolveStaleOpenTaskRecordRejectsCurrentRecordAndInvalidTime(t *testing.T) {
	_, user, _, _, attendance, currentRecord := setupActiveTaskCacheTestDB(t)
	a := NewApp()
	a.AuthUser = &models.AuthUser{LocalID: user.ID}
	a.Attendance = attendance
	a.AuthService.TaskService.Attendance = &a.Attendance

	if _, err := a.ResolveStaleOpenTaskRecord(currentRecord.ID, time.Now().UTC().Format(time.RFC3339)); err == nil {
		t.Fatal("expected current-attendance record to be rejected")
	}
	if _, err := a.ResolveStaleOpenTaskRecord(currentRecord.ID+1000, "not-a-date"); err == nil {
		t.Fatal("expected invalid task-out time to be rejected")
	}
}
