package services

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"testing"
	"time"
)

func TestGetAttendanceAndRecordsIncludesPendingScreenshotCountsForCurrentUser(t *testing.T) {
	testDB, err := db.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}

	originalDB := db.DB
	db.DB = testDB
	t.Cleanup(func() {
		db.DB = originalDB
	})

	user := &models.User{ID: "user-1", Email: "user1@example.com"}
	otherUser := &models.User{ID: "user-2", Email: "user2@example.com"}
	if err := testDB.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	if err := testDB.Create(otherUser).Error; err != nil {
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

	// Keep every fixture on the queried date, including runs just after midnight.
	now := time.Date(2026, time.July, 1, 12, 0, 0, 0, time.FixedZone("PHT", 8*60*60))
	attendance := &models.Attendance{
		UserID:     user.ID,
		TimeIn:     now.Add(-time.Hour),
		SyncStatus: "pending",
	}
	otherAttendance := &models.Attendance{
		UserID:     otherUser.ID,
		TimeIn:     now.Add(-30 * time.Minute),
		SyncStatus: "pending",
	}
	if err := testDB.Create(attendance).Error; err != nil {
		t.Fatal(err)
	}
	if err := testDB.Create(otherAttendance).Error; err != nil {
		t.Fatal(err)
	}

	record := &models.TaskRecord{
		UserID:       user.ID,
		AttendanceID: attendance.ID,
		TaskID:       task.TaskID,
		StartedAt:    now.Add(-50 * time.Minute),
		SyncStatus:   "synced",
	}
	otherRecord := &models.TaskRecord{
		UserID:       otherUser.ID,
		AttendanceID: otherAttendance.ID,
		TaskID:       task.TaskID,
		StartedAt:    now.Add(-20 * time.Minute),
		SyncStatus:   "synced",
	}
	if err := testDB.Create(record).Error; err != nil {
		t.Fatal(err)
	}
	if err := testDB.Create(otherRecord).Error; err != nil {
		t.Fatal(err)
	}

	if err := testDB.Create(&models.Screenshot{
		TaskRecordID: record.ID,
		TaskID:       record.TaskID,
		FilePath:     `["pending.webp"]`,
		TakenAt:      now.Add(-45 * time.Minute),
	}).Error; err != nil {
		t.Fatal(err)
	}
	uploadedCloudURL := `["remote.webp"]`
	if err := testDB.Create(&models.Screenshot{
		TaskRecordID: otherRecord.ID,
		TaskID:       otherRecord.TaskID,
		FilePath:     `["uploaded.webp"]`,
		CloudURL:     &uploadedCloudURL,
		TakenAt:      now.Add(-10 * time.Minute),
	}).Error; err != nil {
		t.Fatal(err)
	}

	dateStr := now.Format("2006-01-02")
	records, err := GetAttendanceAndRecords(dateStr, user.ID)
	if err != nil {
		t.Fatalf("GetAttendanceAndRecords returned error: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 attendance record, got %d", len(records))
	}
	if len(records[0].Records) != 1 {
		t.Fatalf("expected 1 task record, got %d", len(records[0].Records))
	}
	if records[0].Records[0].PendingScreenshotCount != 1 {
		t.Fatalf("expected pending screenshot count 1, got %d", records[0].Records[0].PendingScreenshotCount)
	}
}
