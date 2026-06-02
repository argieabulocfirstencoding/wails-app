package db

import (
	"strings"
	"testing"
	"time"

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

func TestMigrateSchema_RepairsTaskTrackingForeignKeys(t *testing.T) {
	database := newMemoryDB(t)

	if err := database.AutoMigrate(&schemaMigration{}); err != nil {
		t.Fatalf("auto migrate schema migrations: %v", err)
	}

	if err := database.Create(&schemaMigration{
		Version:    2,
		Name:       "hot_query_indexes",
		AppliedAt:  time.Now().UTC(),
		Successful: true,
	}).Error; err != nil {
		t.Fatalf("seed schema version: %v", err)
	}

	// Simulate the legacy circular foreign-key layout that the startup repair must remove.
	if err := database.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY);`).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := database.Exec(`CREATE TABLE attendances (
		id INTEGER PRIMARY KEY,
		user_id TEXT NOT NULL,
		time_in DATETIME,
		sync_status TEXT NOT NULL DEFAULT "pending"
	);`).Error; err != nil {
		t.Fatalf("create attendances: %v", err)
	}
	if err := database.Exec(`CREATE TABLE clients (id TEXT PRIMARY KEY);`).Error; err != nil {
		t.Fatalf("create clients: %v", err)
	}
	if err := database.Exec(`CREATE TABLE task_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		attendance_id INTEGER NOT NULL,
		task_id TEXT NOT NULL,
		started_at DATETIME,
		padagan NUMERIC NOT NULL DEFAULT false,
		notes TEXT NOT NULL DEFAULT "",
		ended_at DATETIME,
		duration INTEGER,
		synced_at DATETIME,
		productivity TEXT,
		sync_status TEXT NOT NULL DEFAULT "pending",
		attempt_count INTEGER NOT NULL DEFAULT 0,
		last_error TEXT NOT NULL DEFAULT "",
		last_attempt_at DATETIME,
		gap INTEGER NOT NULL DEFAULT 0,
		CONSTRAINT fk_task_records_user FOREIGN KEY (user_id) REFERENCES users(id),
		CONSTRAINT fk_task_records_attendance FOREIGN KEY (attendance_id) REFERENCES attendances(id),
		CONSTRAINT fk_tasks_task_record FOREIGN KEY (task_id) REFERENCES tasks(id)
	);`).Error; err != nil {
		t.Fatalf("create legacy task_records: %v", err)
	}
	if err := database.Exec(`CREATE TABLE screenshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_record_id INTEGER NOT NULL,
		task_id TEXT NOT NULL,
		memo TEXT NOT NULL DEFAULT "",
		notes TEXT NOT NULL DEFAULT "",
		file_path TEXT,
		cloud_url TEXT,
		taken_at DATETIME NOT NULL,
		CONSTRAINT fk_task_records_screenshots FOREIGN KEY (task_record_id) REFERENCES task_records(id)
	);`).Error; err != nil {
		t.Fatalf("create legacy screenshots: %v", err)
	}
	if err := database.Exec(`CREATE TABLE tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id char(36),
		title TEXT,
		project_name TEXT,
		client_id TEXT,
		CONSTRAINT fk_clients_tasks FOREIGN KEY (client_id) REFERENCES clients(id),
		CONSTRAINT fk_task_records_task FOREIGN KEY (task_id) REFERENCES task_records(task_id),
		CONSTRAINT fk_screenshots_task FOREIGN KEY (task_id) REFERENCES screenshots(task_id),
		CONSTRAINT uni_tasks_task_id UNIQUE (task_id)
	);`).Error; err != nil {
		t.Fatalf("create legacy tasks: %v", err)
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

	var taskSQL string
	if err := database.Raw(`SELECT sql FROM sqlite_master WHERE type='table' AND name='tasks'`).Scan(&taskSQL).Error; err != nil {
		t.Fatalf("inspect tasks schema: %v", err)
	}
	if strings.Contains(taskSQL, "task_records") || strings.Contains(taskSQL, "screenshots") {
		t.Fatalf("expected repaired tasks schema without legacy circular foreign keys, got: %s", taskSQL)
	}

	var taskRecordSQL string
	if err := database.Raw(`SELECT sql FROM sqlite_master WHERE type='table' AND name='task_records'`).Scan(&taskRecordSQL).Error; err != nil {
		t.Fatalf("inspect task_records schema: %v", err)
	}
	if strings.Contains(taskRecordSQL, "REFERENCES `tasks`") || strings.Contains(taskRecordSQL, "REFERENCES tasks") {
		t.Fatalf("expected repaired task_records schema without task foreign key, got: %s", taskRecordSQL)
	}
}
