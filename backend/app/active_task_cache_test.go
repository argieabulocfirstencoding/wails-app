package app

import (
	"context"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/repository"
	"fesapp/backend/service_classes"
	"testing"
	"time"

	sqlite "github.com/Tryanks/gorm-sqlite"
	"gorm.io/gorm"
)

func setupActiveTaskCacheTestDB(t *testing.T) (*gorm.DB, *models.User, *models.Client, *models.Task, *models.Attendance, *models.TaskRecord) {
	t.Helper()

	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	if err := testDB.AutoMigrate(
		&models.User{},
		&models.Client{},
		&models.Task{},
		&models.Attendance{},
		&models.TaskRecord{},
		&models.Screenshot{},
	); err != nil {
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
		ClientID:    client.ID,
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
