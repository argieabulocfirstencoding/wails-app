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

func TestReconcileLocalProductivityPresenceMarksOnlyMissingRecords(t *testing.T) {
	database, user, task := setupBrowserSyncTestDB(t)
	attendance := &models.Attendance{UserID: user.ID, TimeIn: time.Now().Add(-time.Hour), SyncStatus: syncStateSynced}
	if err := database.Create(attendance).Error; err != nil {
		t.Fatal(err)
	}

	verifiedID := "productivity-present"
	missingID := "productivity-missing"
	verifiedRecord := &models.TaskRecord{
		UserID: user.ID, AttendanceID: attendance.ID, TaskID: task.TaskID,
		StartedAt: time.Now().Add(-45 * time.Minute), Productivity: &verifiedID, SyncStatus: syncStateSynced,
	}
	missingRecord := &models.TaskRecord{
		UserID: user.ID, AttendanceID: attendance.ID, TaskID: task.TaskID,
		StartedAt: time.Now().Add(-30 * time.Minute), Productivity: &missingID, SyncStatus: syncStateSynced,
	}
	missingIdentifierRecord := &models.TaskRecord{
		UserID: user.ID, AttendanceID: attendance.ID, TaskID: task.TaskID,
		StartedAt: time.Now().Add(-15 * time.Minute), SyncStatus: syncStateSynced,
	}
	if err := database.Create(verifiedRecord).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(missingRecord).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(missingIdentifierRecord).Error; err != nil {
		t.Fatal(err)
	}

	if err := database.Transaction(func(tx *gorm.DB) error {
		verified, missingInBubble, missingIdentifier, err := reconcileLocalProductivityPresence(tx, user.ID, map[string]struct{}{verifiedID: {}})
		if err != nil {
			return err
		}
		if verified != 1 || missingInBubble != 1 || missingIdentifier != 1 {
			t.Fatalf("unexpected verification result: verified=%d missingInBubble=%d missingIdentifier=%d", verified, missingInBubble, missingIdentifier)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, recordID := range []uint{missingRecord.ID, missingIdentifierRecord.ID} {
		var stored models.TaskRecord
		if err := database.First(&stored, recordID).Error; err != nil {
			t.Fatal(err)
		}
		if stored.SyncStatus != syncStateRemoteMissing || stored.LastError == "" {
			t.Fatalf("expected record %d to be marked missing in Bubble, got %#v", recordID, stored)
		}
	}

	var storedVerified models.TaskRecord
	if err := database.First(&storedVerified, verifiedRecord.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedVerified.SyncStatus != syncStateSynced {
		t.Fatalf("expected present record to remain synced, got %#v", storedVerified)
	}
}

func TestReconcileLocalProductivityPresenceRestoresRemoteMissingRecord(t *testing.T) {
	database, user, task := setupBrowserSyncTestDB(t)
	attendance := &models.Attendance{UserID: user.ID, TimeIn: time.Now().Add(-time.Hour), SyncStatus: syncStateSynced}
	if err := database.Create(attendance).Error; err != nil {
		t.Fatal(err)
	}
	productivityID := "productivity-restored"
	record := &models.TaskRecord{
		UserID: user.ID, AttendanceID: attendance.ID, TaskID: task.TaskID,
		StartedAt: time.Now(), Productivity: &productivityID, SyncStatus: syncStateRemoteMissing, LastError: "missing",
	}
	if err := database.Create(record).Error; err != nil {
		t.Fatal(err)
	}

	if err := database.Transaction(func(tx *gorm.DB) error {
		_, missingInBubble, missingIdentifier, err := reconcileLocalProductivityPresence(tx, user.ID, map[string]struct{}{productivityID: {}})
		if err != nil {
			return err
		}
		if missingInBubble != 0 || missingIdentifier != 0 {
			t.Fatalf("expected restored record to have no mismatch, got missingInBubble=%d missingIdentifier=%d", missingInBubble, missingIdentifier)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var stored models.TaskRecord
	if err := database.First(&stored, record.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.SyncStatus != syncStateSynced || stored.LastError != "" || stored.SyncedAt == nil {
		t.Fatalf("expected restored record to be synced, got %#v", stored)
	}
}

func TestParseBubbleProductivitiesKeepsLegacyRowWithoutAttendanceID(t *testing.T) {
	attendanceTime := time.Now().Add(-time.Hour).Round(time.Millisecond)
	startedAt := attendanceTime.Add(5 * time.Minute)
	parsed, err := parseBubbleProductivities([]any{map[string]any{
		"_id":                 "productivity-legacy-1",
		"EmployeeID":          "employee-1",
		"Projects - Projects": []any{"task-management"},
		"Modified Date":       float64(startedAt.Add(time.Minute).UnixMilli()),
		"Actual Task In":      float64(startedAt.UnixMilli()),
		"Date":                float64(attendanceTime.UnixMilli()),
		"Trackers Used":       "N/A ()",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.SkippedMalformed != 0 || len(parsed.Records) != 1 {
		t.Fatalf("expected one legacy productivity row, got %#v", parsed)
	}
	if parsed.Records[0].AttendanceID != "" || !parsed.Records[0].AttendanceTime.Equal(attendanceTime) {
		t.Fatalf("expected exact Date attendance fallback metadata, got %#v", parsed.Records[0])
	}
	if parsed.Records[0].TaskID != "task-management" {
		t.Fatalf("expected list-shaped project reference to be accepted, got %#v", parsed.Records[0])
	}
}

func TestEnsureBrowserTaskCreatesFallbackBeforeProductivityImport(t *testing.T) {
	database, user, _ := setupBrowserSyncTestDB(t)
	startedAt := time.Now().Add(-time.Minute).Round(time.Millisecond)
	attendance := &models.Attendance{UserID: user.ID, TimeIn: startedAt.Add(-time.Hour), SyncStatus: syncStateSynced}
	if err := database.Create(attendance).Error; err != nil {
		t.Fatal(err)
	}
	remote := bubbleProductivityRecord{
		ID:         "productivity-fallback-1",
		EmployeeID: user.EmployeeId,
		TaskID:     "task-from-browser-only",
		StartedAt:  startedAt,
		ModifiedAt: startedAt.Add(time.Minute),
		TaskTitle:  "N/A ()",
	}

	if err := database.Transaction(func(tx *gorm.DB) error {
		createdFallback, err := ensureBrowserTask(tx, remote)
		if err != nil {
			return err
		}
		if !createdFallback {
			t.Fatal("expected missing Bubble project to create a fallback task")
		}
		created, updated, conflict, skipped, err := reconcileBrowserProductivity(tx, user.ID, attendance, remote)
		if err != nil {
			return err
		}
		if !created || updated || conflict || skipped {
			t.Fatalf("unexpected productivity import result: created=%t updated=%t conflict=%t skipped=%t", created, updated, conflict, skipped)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var task models.Task
	if err := database.Where("task_id = ?", remote.TaskID).First(&task).Error; err != nil {
		t.Fatal(err)
	}
	if task.Title != "N/A ()" || task.ProjectName != "Imported Bubble project" {
		t.Fatalf("unexpected fallback task: %#v", task)
	}
}
func TestResolveEmployeeIDUsesSavedProfile(t *testing.T) {
	service := NewSyncService()
	user := &models.User{EmployeeId: "employee-1"}

	got, err := service.resolveEmployeeID(user)
	if err != nil {
		t.Fatal(err)
	}
	if got != "employee-1" {
		t.Fatalf("resolveEmployeeID() = %q, want %q", got, "employee-1")
	}
}
