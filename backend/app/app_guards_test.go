package app

import (
	"context"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"testing"
)

func TestFetchUserDetailsRequiresAuthentication(t *testing.T) {
	a := NewApp()

	_, err := a.FetchUserDetails()
	if err == nil || err.Error() != "Login required." {
		t.Fatalf("expected auth-required error, got %v", err)
	}
}

func TestGetActiveTaskRecordReturnsNilWithoutAuthentication(t *testing.T) {
	a := NewApp()
	active := a.GetActiveTaskRecord()
	if active != nil {
		t.Fatalf("expected nil active task without authentication, got %v", *active)
	}
}

func TestShutdownStopsActiveTaskLocally(t *testing.T) {
	_, user, _, task, attendance, taskRecord := setupActiveTaskCacheTestDB(t)

	a := NewApp()
	a.ctx = context.Background()
	a.AuthUser = &models.AuthUser{LocalID: user.ID}
	a.Attendance = attendance
	a.AttendanceID = &attendance.ID
	a.WorkingTaskID = &task.TaskID
	a.AuthService.TaskService.Attendance = &a.Attendance
	a.AuthService.TaskService.WorkingTaskID = &a.WorkingTaskID
	a.AuthService.TaskService.Connect(a.ctx)
	a.ScreenshotService.TaskID = &task.TaskID
	a.ScreenshotService.TaskRecordID = &taskRecord.ID

	a.Shutdown(nil)

	var stopped models.TaskRecord
	if err := db.DB.First(&stopped, taskRecord.ID).Error; err != nil {
		t.Fatalf("expected task record after shutdown: %v", err)
	}
	if stopped.EndedAt == nil {
		t.Fatalf("expected shutdown to end the active task record")
	}
	if stopped.SyncStatus != "pending" {
		t.Fatalf("expected shutdown to leave task record retryable, got %q", stopped.SyncStatus)
	}
	if a.WorkingTaskID != nil {
		t.Fatalf("expected working task cache to be cleared after shutdown")
	}
}
