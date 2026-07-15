package service_classes

import (
	"testing"
	"time"

	"fesapp/backend/db"
	"fesapp/backend/db/models"

	"gorm.io/gorm"
)

func setupBrowserSyncTestDB(t *testing.T) (*gorm.DB, *models.User, *models.Task) {
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

	user := &models.User{ID: "user-1", Email: "user@example.com", EmployeeId: "employee-1"}
	if err := db.DB.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	task := &models.Task{TaskID: "task-1", Title: "Task", ProjectName: "Project"}
	if err := db.DB.Create(task).Error; err != nil {
		t.Fatal(err)
	}

	return testDB, user, task
}

func TestReconcileBrowserRecordsCreatesThenUpdates(t *testing.T) {
	database, user, task := setupBrowserSyncTestDB(t)
	remoteModifiedAt := time.Now().Add(-time.Minute).Round(time.Millisecond)
	remoteAttendance := bubbleAttendanceRecord{
		ID:         "attendance-remote-1",
		EmployeeID: user.EmployeeId,
		ModifiedAt: remoteModifiedAt,
		TimeIn:     remoteModifiedAt.Add(-time.Hour),
	}
	remoteProductivity := bubbleProductivityRecord{
		ID:           "productivity-remote-1",
		EmployeeID:   user.EmployeeId,
		AttendanceID: remoteAttendance.ID,
		TaskID:       task.TaskID,
		ModifiedAt:   remoteModifiedAt,
		StartedAt:    remoteModifiedAt.Add(-30 * time.Minute),
		Duration:     60,
		Notes:        "created in browser",
	}

	var attendance *models.Attendance
	if err := database.Transaction(func(tx *gorm.DB) error {
		var err error
		attendance, _, _, _, err = reconcileBrowserAttendance(tx, user.ID, remoteAttendance)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if attendance == nil || attendance.LoginLogoutID == nil || *attendance.LoginLogoutID != remoteAttendance.ID {
		t.Fatalf("expected imported attendance, got %#v", attendance)
	}

	if err := database.Transaction(func(tx *gorm.DB) error {
		created, updated, conflict, skipped, err := reconcileBrowserProductivity(tx, user.ID, attendance, remoteProductivity)
		if err != nil {
			return err
		}
		if !created || updated || conflict || skipped {
			t.Fatalf("unexpected first productivity result: created=%t updated=%t conflict=%t skipped=%t", created, updated, conflict, skipped)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var stored models.TaskRecord
	if err := database.Where("productivity = ?", remoteProductivity.ID).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Notes != "created in browser" || stored.RemoteModifiedAt == nil {
		t.Fatalf("expected imported browser productivity metadata, got %#v", stored)
	}

	remoteProductivity.ModifiedAt = remoteModifiedAt.Add(time.Minute)
	remoteProductivity.Notes = "edited in browser"
	if err := database.Transaction(func(tx *gorm.DB) error {
		created, updated, conflict, skipped, err := reconcileBrowserProductivity(tx, user.ID, attendance, remoteProductivity)
		if err != nil {
			return err
		}
		if created || !updated || conflict || skipped {
			t.Fatalf("unexpected update productivity result: created=%t updated=%t conflict=%t skipped=%t", created, updated, conflict, skipped)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := database.Where("id = ?", stored.ID).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Notes != "edited in browser" {
		t.Fatalf("expected newer Bubble note to update local record, got %q", stored.Notes)
	}
}

func TestReconcileBrowserProductivityPreservesLocalPendingRecord(t *testing.T) {
	database, user, task := setupBrowserSyncTestDB(t)
	modifiedAt := time.Now().Add(-time.Minute).Round(time.Millisecond)
	attendanceID := "attendance-remote-1"
	attendance := &models.Attendance{
		UserID:        user.ID,
		TimeIn:        modifiedAt.Add(-time.Hour),
		LoginLogoutID: &attendanceID,
		SyncStatus:    syncStateSynced,
	}
	if err := database.Create(attendance).Error; err != nil {
		t.Fatal(err)
	}

	productivityID := "productivity-remote-1"
	local := &models.TaskRecord{
		UserID:           user.ID,
		AttendanceID:     attendance.ID,
		TaskID:           task.TaskID,
		StartedAt:        modifiedAt.Add(-30 * time.Minute),
		Notes:            "keep local change",
		Productivity:     &productivityID,
		RemoteModifiedAt: &modifiedAt,
		SyncStatus:       syncStatePending,
	}
	if err := database.Create(local).Error; err != nil {
		t.Fatal(err)
	}

	remote := bubbleProductivityRecord{
		ID:           productivityID,
		EmployeeID:   user.EmployeeId,
		AttendanceID: attendanceID,
		TaskID:       task.TaskID,
		ModifiedAt:   modifiedAt.Add(time.Minute),
		StartedAt:    local.StartedAt,
		Notes:        "browser change",
	}

	if err := database.Transaction(func(tx *gorm.DB) error {
		created, updated, conflict, skipped, err := reconcileBrowserProductivity(tx, user.ID, attendance, remote)
		if err != nil {
			return err
		}
		if created || updated || !conflict || skipped {
			t.Fatalf("unexpected conflict productivity result: created=%t updated=%t conflict=%t skipped=%t", created, updated, conflict, skipped)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := database.Where("id = ?", local.ID).First(local).Error; err != nil {
		t.Fatal(err)
	}
	if local.Notes != "keep local change" {
		t.Fatalf("expected pending local record to be preserved, got %q", local.Notes)
	}
}
