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

func TestStartTracking_InvalidTaskDoesNotStopActiveRecord(t *testing.T) {
	database := setupWorkflowTestDB(t)

	user := seedWorkflowUser(t, database)
	task := seedWorkflowTask(t, database, "Production")
	attendance := seedWorkflowAttendance(t, database, user.ID)
	taskService := newOfflineWorkflowTaskService(attendance)

	if err := taskService.StartTracking(task.TaskID, "local-workflow", false); err != nil {
		t.Fatalf("start valid task: %v", err)
	}

	var activeBefore models.TaskRecord
	if err := database.Where("user_id = ? AND attendance_id = ? AND task_id = ?", user.ID, attendance.ID, task.TaskID).
		First(&activeBefore).Error; err != nil {
		t.Fatalf("fetch active record: %v", err)
	}

	if err := taskService.StartTracking("missing-task-id", "local-workflow", false); err == nil {
		t.Fatal("expected invalid task start to fail")
	}

	var activeAfter models.TaskRecord
	if err := database.Where("id = ?", activeBefore.ID).First(&activeAfter).Error; err != nil {
		t.Fatalf("reload active record: %v", err)
	}
	if activeAfter.EndedAt != nil {
		t.Fatalf("invalid task start closed active record: ended_at=%v", activeAfter.EndedAt)
	}
}

func TestStopTracking_OnlyStopsCurrentAttendanceRecord(t *testing.T) {
	database := setupWorkflowTestDB(t)

	user := seedWorkflowUser(t, database)
	task := seedWorkflowTask(t, database, "Production")
	oldAttendance := seedWorkflowAttendance(t, database, user.ID)
	currentAttendance := seedWorkflowAttendance(t, database, user.ID)
	startedAt := time.Now().Add(-10 * time.Minute)

	oldRecord := models.TaskRecord{
		UserID:       user.ID,
		AttendanceID: oldAttendance.ID,
		TaskID:       task.TaskID,
		StartedAt:    startedAt,
	}
	currentRecord := models.TaskRecord{
		UserID:       user.ID,
		AttendanceID: currentAttendance.ID,
		TaskID:       task.TaskID,
		StartedAt:    startedAt,
	}
	if err := database.Create(&oldRecord).Error; err != nil {
		t.Fatalf("seed old active record: %v", err)
	}
	if err := database.Create(&currentRecord).Error; err != nil {
		t.Fatalf("seed current active record: %v", err)
	}

	taskService := newOfflineWorkflowTaskService(currentAttendance)
	if err := taskService.StopTracking(task.TaskID); err != nil {
		t.Fatalf("stop current task: %v", err)
	}

	var reloadedOld models.TaskRecord
	if err := database.Where("id = ?", oldRecord.ID).First(&reloadedOld).Error; err != nil {
		t.Fatalf("reload old record: %v", err)
	}
	if reloadedOld.EndedAt != nil {
		t.Fatalf("old attendance record should remain open, got ended_at=%v", reloadedOld.EndedAt)
	}

	var reloadedCurrent models.TaskRecord
	if err := database.Where("id = ?", currentRecord.ID).First(&reloadedCurrent).Error; err != nil {
		t.Fatalf("reload current record: %v", err)
	}
	if reloadedCurrent.EndedAt == nil {
		t.Fatal("current attendance record should be closed")
	}
}

func TestGetLastTaskRecord_UsesCurrentAttendance(t *testing.T) {
	database := setupWorkflowTestDB(t)

	user := seedWorkflowUser(t, database)
	task := seedWorkflowTask(t, database, "Production")
	currentAttendance := seedWorkflowAttendance(t, database, user.ID)
	otherAttendance := seedWorkflowAttendance(t, database, user.ID)

	currentRecord := models.TaskRecord{
		UserID:       user.ID,
		AttendanceID: currentAttendance.ID,
		TaskID:       task.TaskID,
		StartedAt:    time.Now().Add(-20 * time.Minute),
	}
	newerOtherRecord := models.TaskRecord{
		UserID:       user.ID,
		AttendanceID: otherAttendance.ID,
		TaskID:       task.TaskID,
		StartedAt:    time.Now(),
	}
	if err := database.Create(&currentRecord).Error; err != nil {
		t.Fatalf("seed current record: %v", err)
	}
	if err := database.Create(&newerOtherRecord).Error; err != nil {
		t.Fatalf("seed other record: %v", err)
	}

	taskService := newOfflineWorkflowTaskService(currentAttendance)
	record, err := taskService.GetLastTaskRecord(user.ID, task.TaskID)
	if err != nil {
		t.Fatalf("get last task record: %v", err)
	}
	if record == nil {
		t.Fatal("expected task record")
	}
	if record.ID != currentRecord.ID {
		t.Fatalf("expected current attendance record %d, got %d", currentRecord.ID, record.ID)
	}
}

func seedWorkflowUser(t *testing.T, database *gorm.DB) *models.User {
	t.Helper()

	user := &models.User{
		ID:    "user-1",
		Email: "test@example.com",
	}
	if err := database.Create(user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user
}

func seedWorkflowTask(t *testing.T, database *gorm.DB, projectName string) *models.Task {
	t.Helper()

	client := &models.Client{
		ID:          "client-" + projectName,
		Fullname:    "Client " + projectName,
		CompanyName: "Client " + projectName,
	}
	if err := database.Create(client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}

	task := &models.Task{
		Title:       "Task " + projectName,
		ProjectName: projectName,
		ClientID:    client.ID,
	}
	if err := database.Create(task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	return task
}

func seedWorkflowAttendance(t *testing.T, database *gorm.DB, userID string) *models.Attendance {
	t.Helper()

	attendance := &models.Attendance{
		UserID: userID,
		TimeIn: time.Now(),
	}
	if err := database.Create(attendance).Error; err != nil {
		t.Fatalf("seed attendance: %v", err)
	}
	return attendance
}

func newOfflineWorkflowTaskService(attendance *models.Attendance) *TaskService {
	connectivity := NewConnectivityStateService()
	connectivity.SetOnline(false)
	taskService := NewTaskService(nil, connectivity)
	taskService.Connectivity.SetOnline(false)

	workingTaskID := (*string)(nil)
	taskService.WorkingTaskID = &workingTaskID
	attendancePtr := attendance
	taskService.Attendance = &attendancePtr

	return taskService
}
