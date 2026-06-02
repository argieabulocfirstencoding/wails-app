package service_classes

import (
	"path/filepath"
	"testing"
	"time"

	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/repository"

	sqlite "github.com/Tryanks/gorm-sqlite"
	"gorm.io/gorm"
)

func setupWorkflowTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "workflow.db")
	database, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("resolve sql db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
		db.DB = nil
	})

	if err := database.AutoMigrate(
		&models.User{},
		&models.Client{},
		&models.Task{},
		&models.Attendance{},
		&models.TaskRecord{},
		&models.Screenshot{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	db.DB = database
	return database
}

func TestLocalWorkflow_WritesAttendanceAndTaskRecords(t *testing.T) {
	database := setupWorkflowTestDB(t)

	user := &models.User{
		ID:    "user-1",
		Email: "test@example.com",
	}
	if err := database.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	client := &models.Client{
		ID:          "client-1",
		Fullname:    "Client One",
		CompanyName: "Client One Company",
	}
	if err := database.Create(client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}

	task := &models.Task{
		Title:       "Task A",
		ProjectName: "Production",
		ClientID:    client.ID,
	}
	if err := database.Create(task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	attendanceRepo := repository.NewAttendanceRepository()
	attendance, err := attendanceRepo.CreateAttendance(user.ID)
	if err != nil {
		t.Fatalf("create attendance: %v", err)
	}
	if attendance == nil {
		t.Fatal("expected attendance record")
	}

	connectivity := NewConnectivityStateService()
	connectivity.SetOnline(false)
	syncService := NewSyncService(connectivity)
	taskService := NewTaskService(syncService, connectivity)
	taskService.Connectivity.SetOnline(false)
	taskService.SyncService = nil
	taskService.ScreenshotService.SyncService = nil

	workingTaskID := (*string)(nil)
	taskService.WorkingTaskID = &workingTaskID
	attendancePtr := attendance
	taskService.Attendance = &attendancePtr

	if err := taskService.StartTracking(task.TaskID, "local-workflow", false); err != nil {
		t.Fatalf("start tracking: %v", err)
	}

	var record models.TaskRecord
	if err := database.Where("user_id = ? AND attendance_id = ? AND task_id = ?", user.ID, attendance.ID, task.TaskID).
		First(&record).Error; err != nil {
		t.Fatalf("fetch started task record: %v", err)
	}
	if record.EndedAt != nil {
		t.Fatalf("expected active task record, got ended_at=%v", record.EndedAt)
	}

	if err := taskService.StopTracking(task.TaskID); err != nil {
		t.Fatalf("stop tracking: %v", err)
	}

	if err := database.Where("id = ?", record.ID).First(&record).Error; err != nil {
		t.Fatalf("reload stopped task record: %v", err)
	}
	if record.EndedAt == nil {
		t.Fatal("expected task record to be closed")
	}
	if record.Duration < 0 {
		t.Fatalf("expected non-negative duration, got %d", record.Duration)
	}

	if err := attendanceRepo.LogoutAttendance(attendance); err != nil {
		t.Fatalf("logout attendance: %v", err)
	}

	var savedAttendance models.Attendance
	if err := database.Where("id = ?", attendance.ID).First(&savedAttendance).Error; err != nil {
		t.Fatalf("reload attendance: %v", err)
	}
	if savedAttendance.TimeOut == nil {
		t.Fatal("expected attendance timeout to be written")
	}
	if savedAttendance.SyncStatus != "pending" {
		t.Fatalf("expected pending sync status after logout, got %q", savedAttendance.SyncStatus)
	}

	// Give any started background goroutine a brief chance to exit cleanly.
	if taskService.ScreenshotService != nil {
		taskService.ScreenshotService.Stop()
	}
	time.Sleep(10 * time.Millisecond)
}
