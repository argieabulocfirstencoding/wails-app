package services

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"testing"
	"time"
)

func TestGetTodaySummarySeparatesProjectsForAuthenticatedUser(t *testing.T) {
	testDB, err := db.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}

	originalDB := db.DB
	db.DB = testDB
	t.Cleanup(func() { db.DB = originalDB })

	user := &models.User{ID: "employee-1", Email: "employee@example.com"}
	otherUser := &models.User{ID: "employee-2", Email: "other@example.com"}
	if err := testDB.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	if err := testDB.Create(otherUser).Error; err != nil {
		t.Fatal(err)
	}

	client := &models.Client{ID: "shared-client", Fullname: "FES", CompanyName: "First Encoding"}
	if err := testDB.Create(client).Error; err != nil {
		t.Fatal(err)
	}
	for _, task := range []*models.Task{
		{TaskID: "project-break", Title: "N/A ()", ProjectName: "Break", ClientID: models.StringPtr(client.ID)},
		{TaskID: "project-management", Title: "N/A ()", ProjectName: "Management", ClientID: models.StringPtr(client.ID)},
		{TaskID: "project-training", Title: "Toptracker ()", ProjectName: "Training", ClientID: models.StringPtr(client.ID)},
	} {
		if err := testDB.Create(task).Error; err != nil {
			t.Fatal(err)
		}
	}

	manila, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().In(manila)
	attendanceTime := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, manila)
	attendance := &models.Attendance{UserID: user.ID, TimeIn: attendanceTime, SyncStatus: "synced"}
	otherAttendance := &models.Attendance{UserID: otherUser.ID, TimeIn: attendanceTime, SyncStatus: "synced"}
	if err := testDB.Create(attendance).Error; err != nil {
		t.Fatal(err)
	}
	if err := testDB.Create(otherAttendance).Error; err != nil {
		t.Fatal(err)
	}

	records := []*models.TaskRecord{
		{UserID: user.ID, AttendanceID: attendance.ID, TaskID: "project-break", StartedAt: attendanceTime.Add(time.Minute), Duration: 600, SyncStatus: "synced"},
		{UserID: user.ID, AttendanceID: attendance.ID, TaskID: "project-break", StartedAt: attendanceTime.Add(12 * time.Minute), Duration: 1200, SyncStatus: "synced"},
		{UserID: user.ID, AttendanceID: attendance.ID, TaskID: "project-management", StartedAt: attendanceTime.Add(35 * time.Minute), Duration: 2400, SyncStatus: "synced"},
		{UserID: user.ID, AttendanceID: attendance.ID, TaskID: "project-training", StartedAt: attendanceTime.Add(80 * time.Minute), Duration: 3000, SyncStatus: "synced"},
		{UserID: otherUser.ID, AttendanceID: otherAttendance.ID, TaskID: "project-break", StartedAt: attendanceTime.Add(time.Minute), Duration: 156000, SyncStatus: "synced"},
	}
	for _, record := range records {
		if err := testDB.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}

	summaries, err := GetTodaySummary(attendanceTime.Format("2006-01-02"), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 3 {
		t.Fatalf("expected one total for each project, got %#v", summaries)
	}

	durationByProject := make(map[string]int64, len(summaries))
	for _, summary := range summaries {
		durationByProject[summary.TaskID] = summary.TotalDuration
	}
	if got := durationByProject["project-break"]; got != 1800 {
		t.Fatalf("Break duration = %d, want 1800; other employee duration must not be included", got)
	}
	if got := durationByProject["project-management"]; got != 2400 {
		t.Fatalf("Management duration = %d, want 2400", got)
	}
	if got := durationByProject["project-training"]; got != 3000 {
		t.Fatalf("Training duration = %d, want 3000", got)
	}
}

func TestGetTodaySummaryRequiresAuthenticatedUser(t *testing.T) {
	if _, err := GetTodaySummary("2026-07-15", " "); err == nil {
		t.Fatal("expected empty user ID to be rejected")
	}
}
