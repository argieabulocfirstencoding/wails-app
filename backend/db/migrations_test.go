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

func TestRepairTaskRecordsSchema_BackfillsMissingUserID(t *testing.T) {
	database := newMemoryDB(t)

	if err := database.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY);`).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := database.Exec(`CREATE TABLE attendances (id INTEGER PRIMARY KEY, user_id TEXT NOT NULL, time_in DATETIME, sync_status TEXT);`).Error; err != nil {
		t.Fatalf("create attendances: %v", err)
	}
	if err := database.Exec(`CREATE TABLE tasks (task_id TEXT PRIMARY KEY, title TEXT, project_name TEXT, client_id TEXT);`).Error; err != nil {
		t.Fatalf("create tasks: %v", err)
	}
	if err := database.Exec(`
		CREATE TABLE task_records (
			id INTEGER PRIMARY KEY,
			user_id TEXT,
			attendance_id INTEGER,
			task_id TEXT,
			started_at DATETIME,
			padagan INTEGER,
			notes TEXT,
			ended_at DATETIME,
			duration INTEGER,
			synced_at DATETIME,
			productivity TEXT,
			sync_status TEXT,
			attempt_count INTEGER,
			last_error TEXT,
			last_attempt_at DATETIME,
			gap INTEGER
		);`).Error; err != nil {
		t.Fatalf("create task_records: %v", err)
	}

	if err := database.Exec(`INSERT INTO users (id) VALUES ('user-1');`).Error; err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := database.Exec(`INSERT INTO attendances (id, user_id, time_in, sync_status) VALUES (1, 'user-1', CURRENT_TIMESTAMP, 'pending');`).Error; err != nil {
		t.Fatalf("insert attendance: %v", err)
	}
	if err := database.Exec(`INSERT INTO tasks (task_id, title, project_name, client_id) VALUES ('task-1', 'Title', 'Project', 'client-1');`).Error; err != nil {
		t.Fatalf("insert task: %v", err)
	}
	if err := database.Exec(`
		INSERT INTO task_records (
			id, user_id, attendance_id, task_id, started_at, padagan, notes,
			ended_at, duration, synced_at, productivity, sync_status,
			attempt_count, last_error, last_attempt_at, gap
		) VALUES (
			1, NULL, 1, 'task-1', CURRENT_TIMESTAMP, 0, '',
			NULL, 0, NULL, NULL, 'pending',
			0, '', NULL, 0
		);`).Error; err != nil {
		t.Fatalf("insert legacy task_record: %v", err)
	}

	if err := repairTaskRecordsSchema(database); err != nil {
		t.Fatalf("repair task records schema: %v", err)
	}

	var count int64
	if err := database.Table("task_records").Count(&count).Error; err != nil {
		t.Fatalf("count repaired task records: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 task record after repair, got %d", count)
	}

	var userID string
	if err := database.Table("task_records").Select("user_id").Where("id = ?", 1).Scan(&userID).Error; err != nil {
		t.Fatalf("load repaired user_id: %v", err)
	}
	if userID != "user-1" {
		t.Fatalf("expected repaired user_id user-1, got %q", userID)
	}
}
