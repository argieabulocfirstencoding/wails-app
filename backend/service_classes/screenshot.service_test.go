package service_classes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func setupScreenshotServiceTestDB(t *testing.T) (*gorm.DB, *models.User, *models.TaskRecord, *models.TaskRecord) {
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

	user := &models.User{ID: "user-1", Email: "user@example.com"}
	if err := testDB.Create(user).Error; err != nil {
		t.Fatal(err)
	}

	client := &models.Client{ID: "client-1", Fullname: "Client", CompanyName: "Company"}
	if err := testDB.Create(client).Error; err != nil {
		t.Fatal(err)
	}

	task := &models.Task{
		TaskID:      "task-1",
		Title:       "Task",
		ProjectName: "Project",
		ClientID:    models.StringPtr(client.ID),
	}
	if err := testDB.Create(task).Error; err != nil {
		t.Fatal(err)
	}

	attendance := &models.Attendance{
		UserID:     user.ID,
		TimeIn:     time.Now().Add(-2 * time.Hour),
		SyncStatus: "pending",
	}
	if err := testDB.Create(attendance).Error; err != nil {
		t.Fatal(err)
	}

	oldEndedAt := time.Now().Add(-80 * time.Minute)
	oldRecord := &models.TaskRecord{
		UserID:       user.ID,
		AttendanceID: attendance.ID,
		TaskID:       task.TaskID,
		StartedAt:    time.Now().Add(-90 * time.Minute),
		EndedAt:      &oldEndedAt,
		SyncStatus:   "pending",
	}
	if err := testDB.Create(oldRecord).Error; err != nil {
		t.Fatal(err)
	}

	newEndedAt := time.Now().Add(-20 * time.Minute)
	newRecord := &models.TaskRecord{
		UserID:       user.ID,
		AttendanceID: attendance.ID,
		TaskID:       task.TaskID,
		StartedAt:    time.Now().Add(-30 * time.Minute),
		EndedAt:      &newEndedAt,
		SyncStatus:   "pending",
	}
	if err := testDB.Create(newRecord).Error; err != nil {
		t.Fatal(err)
	}

	return testDB, user, oldRecord, newRecord
}

func setDevelopmentScreenshotWorkingDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	t.Setenv("DEVELOPMENT", "1")

	return tmpDir
}

func TestGetLatestRecordsWithScreenshotsUsesLatestTaskRecordScreenshot(t *testing.T) {
	_, user, oldRecord, newRecord := setupScreenshotServiceTestDB(t)
	service := NewScreenshotService()
	service.ctx = context.Background()

	tempDir := t.TempDir()
	oldPath := tempDir + "\\old.webp"
	newPath := tempDir + "\\new.webp"
	oldContent := []byte("old-image")
	newContent := []byte("new-image")

	if err := os.WriteFile(oldPath, oldContent, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, newContent, 0644); err != nil {
		t.Fatal(err)
	}

	oldJSON, _ := json.Marshal([]string{oldPath})
	newJSON, _ := json.Marshal([]string{newPath})

	if err := db.DB.Create(&models.Screenshot{
		TaskRecordID: oldRecord.ID,
		TaskID:       oldRecord.TaskID,
		FilePath:     string(oldJSON),
		TakenAt:      time.Now().Add(-70 * time.Minute),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.Create(&models.Screenshot{
		TaskRecordID: newRecord.ID,
		TaskID:       newRecord.TaskID,
		FilePath:     string(newJSON),
		TakenAt:      time.Now().Add(-10 * time.Minute),
	}).Error; err != nil {
		t.Fatal(err)
	}

<<<<<<< HEAD
	record, err := service.GetLatestRecordsWithScreenshots(user.ID, newRecord.TaskID)
=======
	record, err := service.GetLatestRecordsWithScreenshots(user.ID, newRecord.TaskID, newRecord.AttendanceID)
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
	if err != nil {
		t.Fatalf("GetLatestRecordsWithScreenshots returned error: %v", err)
	}
	if record == nil || record.Record == nil {
		t.Fatalf("expected record metadata")
	}
	if record.Record.ID != newRecord.ID {
		t.Fatalf("expected latest task record ID %d, got %d", newRecord.ID, record.Record.ID)
	}
	if len(record.Screenshots) != 1 {
		t.Fatalf("expected 1 screenshot, got %d", len(record.Screenshots))
	}

	expected := base64.StdEncoding.EncodeToString(newContent)
	if !strings.Contains(record.Screenshots[0], expected) {
		t.Fatalf("expected latest screenshot payload to be returned")
	}
}

func TestGetLatestRecordsWithScreenshotsSkipsMissingFiles(t *testing.T) {
	_, user, _, record := setupScreenshotServiceTestDB(t)
	service := NewScreenshotService()

	tempDir := t.TempDir()
	existingPath := tempDir + "\\shot.webp"
	if err := os.WriteFile(existingPath, []byte("existing-image"), 0644); err != nil {
		t.Fatal(err)
	}

	jsonPaths, _ := json.Marshal([]string{existingPath, tempDir + "\\missing.webp"})
	if err := db.DB.Create(&models.Screenshot{
		TaskRecordID: record.ID,
		TaskID:       record.TaskID,
		FilePath:     string(jsonPaths),
		TakenAt:      time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}

<<<<<<< HEAD
	result, err := service.GetLatestRecordsWithScreenshots(user.ID, record.TaskID)
=======
	result, err := service.GetLatestRecordsWithScreenshots(user.ID, record.TaskID, record.AttendanceID)
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
	if err != nil {
		t.Fatalf("GetLatestRecordsWithScreenshots returned error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected screenshot result")
	}
	if len(result.Screenshots) != 1 {
		t.Fatalf("expected only existing screenshots to be returned, got %d", len(result.Screenshots))
	}
}

func TestScreenshotStorageRootUsesTaskRecordUserFolder(t *testing.T) {
	_, user, _, record := setupScreenshotServiceTestDB(t)
	tmpDir := setDevelopmentScreenshotWorkingDir(t)

	service := NewScreenshotService()
	root, err := service.screenshotStorageRoot(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("screenshotStorageRoot returned error: %v", err)
	}

	want := filepath.Join(tmpDir, "backend", "screenshots", user.ID)
	if root != want {
		t.Fatalf("expected screenshot root %q, got %q", want, root)
	}

	if info, err := os.Stat(root); err != nil {
		t.Fatalf("expected screenshot root to exist: %v", err)
	} else if !info.IsDir() {
		t.Fatalf("expected screenshot root to be a directory")
	}
}
