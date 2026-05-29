package db

import (
	"testing"

	sqlite "github.com/Tryanks/gorm-sqlite"
	"gorm.io/gorm"
)

func newMemoryDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	return database
}

func TestMigrateSchema_FromLegacyWithoutVersionTable(t *testing.T) {
	database := newMemoryDB(t)

	// Simulate an older release without schema_migrations and with legacy users columns.
	if err := database.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY,
		email TEXT,
		birthday TEXT
	);`).Error; err != nil {
		t.Fatalf("create legacy users table: %v", err)
	}
	if err := database.Exec(`CREATE TABLE attendances (id INTEGER PRIMARY KEY, user_id TEXT, time_in DATETIME, sync_status TEXT);`).Error; err != nil {
		t.Fatalf("create attendances table: %v", err)
	}
	if err := database.Exec(`CREATE TABLE task_records (id INTEGER PRIMARY KEY, user_id TEXT, attendance_id INTEGER, task_id TEXT, started_at DATETIME, ended_at DATETIME, sync_status TEXT);`).Error; err != nil {
		t.Fatalf("create task_records table: %v", err)
	}
	if err := database.Exec(`CREATE TABLE screenshots (id INTEGER PRIMARY KEY, task_record_id INTEGER, task_id TEXT, taken_at DATETIME);`).Error; err != nil {
		t.Fatalf("create screenshots table: %v", err)
	}

	if err := migrateSchema(database); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}

	version, err := currentSchemaVersion(database)
	if err != nil {
		t.Fatalf("current schema version: %v", err)
	}
	if version != latestSchemaVersion {
		t.Fatalf("expected schema version %d, got %d", latestSchemaVersion, version)
	}

	if !database.Migrator().HasColumn("users", "birthdate") {
		t.Fatalf("expected repaired users.birthdate column")
	}
}

func TestMigrateSchema_CreatesHotQueryIndexes(t *testing.T) {
	database := newMemoryDB(t)

	if err := database.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY);`).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := database.Exec(`CREATE TABLE attendances (id INTEGER PRIMARY KEY, user_id TEXT, time_in DATETIME, sync_status TEXT);`).Error; err != nil {
		t.Fatalf("create attendances: %v", err)
	}
	if err := database.Exec(`CREATE TABLE task_records (id INTEGER PRIMARY KEY, user_id TEXT, attendance_id INTEGER, task_id TEXT, started_at DATETIME, ended_at DATETIME, sync_status TEXT);`).Error; err != nil {
		t.Fatalf("create task_records: %v", err)
	}
	if err := database.Exec(`CREATE TABLE screenshots (id INTEGER PRIMARY KEY, task_record_id INTEGER, task_id TEXT, taken_at DATETIME);`).Error; err != nil {
		t.Fatalf("create screenshots: %v", err)
	}

	if err := migrateSchema(database); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}

	requiredIndexes := map[string][]string{
		"attendances": {
			"idx_attendances_user_time_in",
			"idx_attendances_sync_status",
		},
		"task_records": {
			"idx_task_records_user_attendance_ended",
			"idx_task_records_attendance_sync_started",
			"idx_task_records_task_started",
		},
		"screenshots": {
			"idx_screenshots_task_record_taken",
			"idx_screenshots_task_taken",
		},
	}
	for table, indexes := range requiredIndexes {
		for _, idx := range indexes {
			if !database.Migrator().HasIndex(table, idx) {
				t.Fatalf("expected index %s on table %s", idx, table)
			}
		}
	}
}
