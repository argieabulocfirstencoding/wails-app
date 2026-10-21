package db

import (
	"fesapp/backend/db/models"
	"testing"
	"time"
)

func TestTaskSessionIdentityMigrationPreservesLegacyRows(t *testing.T) {
	database := newMemoryDB(t)
	// Apply the previous catalog, then upgrade a populated database.
	for _, migration := range migrationCatalog()[:8] {
		if err := migration.Migrate(database); err != nil {
			t.Fatal(err)
		}
	}
	user := &models.User{ID: "legacy-user", Email: "user@example.com"}
	task := &models.Task{TaskID: "legacy-task", Title: "Task"}
	attendance := &models.Attendance{UserID: user.ID, TimeIn: time.Now()}
	if err := database.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(attendance).Error; err != nil {
		t.Fatal(err)
	}
	// Raw insert deliberately uses the previous schema.
	for i := 0; i < 2; i++ {
		if err := database.Exec("INSERT INTO task_records (user_id, attendance_id, task_id, started_at, productivity, sync_status, attempt_count) VALUES (?, ?, ?, ?, ?, ?, ?)", user.ID, attendance.ID, task.TaskID, time.Now(), "same-old-remote", "failed", 2).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := ensureTaskSessionIdentity(database); err != nil {
		t.Fatal(err)
	}
	if err := ensureTaskSessionIdentity(database); err != nil {
		t.Fatal(err)
	}
	var rows []models.TaskRecord
	if err := database.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("changed historical rows: %d", len(rows))
	}
	for _, row := range rows {
		if row.ClientSessionID != nil || row.TaskInAttemptedAt != nil || row.TaskInIdempotent || row.AttemptCount != 2 || row.Productivity == nil || *row.Productivity != "same-old-remote" {
			t.Fatalf("modified legacy identity: %#v", row)
		}
	}
	if err := database.Model(&models.TaskRecord{}).Where("id = ?", rows[0].ID).Update("client_session_id", "session-unique").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&models.TaskRecord{}).Where("id = ?", rows[1].ID).Update("client_session_id", "session-unique").Error; err == nil {
		t.Fatal("duplicate populated session key was accepted")
	}
}
