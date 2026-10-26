package db

import (
	"fesapp/backend/db/models"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func newMemoryDB(t *testing.T) *gorm.DB {
	t.Helper()

	database, err := openSQLiteDatabase(":memory:")
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	if err := applySQLiteConnectionSettings(database); err != nil {
		t.Fatalf("apply sqlite settings: %v", err)
	}

	return database
}

func TestRunMigrations_FreshDatabaseCreatesCanonicalSchema(t *testing.T) {
	database := newMemoryDB(t)

	if err := runMigrations(database, ""); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	for _, table := range []string{"users", "clients", "tasks", "attendances", "task_records", "screenshots", "sync_conflicts", gormMigrationsTable} {
		if !database.Migrator().HasTable(table) {
			t.Fatalf("expected table %s to exist", table)
		}
	}

	for table, indexes := range schemaIndexDefinitions {
		for _, index := range indexes {
			if !database.Migrator().HasIndex(table, index) {
				t.Fatalf("expected index %s on table %s", index, table)
			}
		}
	}

	taskFKs, err := foreignKeysForTable(database, "tasks")
	if err != nil {
		t.Fatalf("load task foreign keys: %v", err)
	}
	if !tasksSchemaIsHealthy(taskFKs) {
		t.Fatalf("expected canonical tasks foreign key, got %+v", taskFKs)
	}

	attendanceFKs, err := foreignKeysForTable(database, "attendances")
	if err != nil {
		t.Fatalf("load attendance foreign keys: %v", err)
	}
	if len(attendanceFKs) != 1 {
		t.Fatalf("expected one attendance foreign key, got %+v", attendanceFKs)
	}
	if fk := attendanceFKs[0]; fk.Table != "users" || fk.From != "user_id" || fk.To != "id" || fk.OnUpdate != "CASCADE" || fk.OnDelete != "CASCADE" {
		t.Fatalf("expected canonical attendances foreign key, got %+v", fk)
	}
}

func TestRunMigrations_FreshDatabaseSupportsTaskClientPreload(t *testing.T) {
	database := newMemoryDB(t)

	if err := runMigrations(database, ""); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	client := models.Client{
		ID:          "client-1",
		Fullname:    "Client One",
		CompanyName: "Company One",
	}
	if err := database.Create(&client).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}

	task := models.Task{
		TaskID:      "task-1",
		Title:       "Task One",
		ProjectName: "Project One",
		ClientID:    models.StringPtr(client.ID),
	}
	if err := database.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	var loaded models.Task
	if err := database.Preload("Client").Where("task_id = ?", task.TaskID).First(&loaded).Error; err != nil {
		t.Fatalf("preload task client: %v", err)
	}
	if loaded.Client.ID != client.ID {
		t.Fatalf("expected preloaded client %q, got %q", client.ID, loaded.Client.ID)
	}
}

func TestRunMigrations_RepairsLegacyUsersSchema(t *testing.T) {
	database := newMemoryDB(t)

	if err := database.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY,
		email TEXT,
		birthday TEXT
	);`).Error; err != nil {
		t.Fatalf("create legacy users: %v", err)
	}
	if err := database.Exec(`INSERT INTO users (id, email, birthday) VALUES (7, 'test@example.com', '2000-01-02');`).Error; err != nil {
		t.Fatalf("insert legacy user: %v", err)
	}

	if err := runMigrations(database, ""); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	if !database.Migrator().HasColumn("users", "birthdate") {
		t.Fatalf("expected repaired users.birthdate column")
	}

	var birthdate string
	if err := database.Table("users").Select("birthdate").Where("id = ?", "7").Scan(&birthdate).Error; err != nil {
		t.Fatalf("load repaired birthdate: %v", err)
	}
	if birthdate != "2000-01-02" {
		t.Fatalf("expected birthdate to survive repair, got %q", birthdate)
	}
}

func TestRunMigrations_BackfillsTaskRecordUserID(t *testing.T) {
	database := newMemoryDB(t)

	if err := database.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY);`).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := database.Exec(`CREATE TABLE clients (id TEXT PRIMARY KEY, fullname TEXT, company_name TEXT);`).Error; err != nil {
		t.Fatalf("create clients: %v", err)
	}
	if err := database.Exec(`CREATE TABLE tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT UNIQUE,
		title TEXT,
		project_name TEXT,
		client_id TEXT,
		CONSTRAINT fk_clients_tasks FOREIGN KEY (client_id) REFERENCES clients(id)
	);`).Error; err != nil {
		t.Fatalf("create tasks: %v", err)
	}
	if err := database.Exec(`CREATE TABLE attendances (
		id INTEGER PRIMARY KEY,
		user_id TEXT NOT NULL,
		time_in DATETIME,
		sync_status TEXT
	);`).Error; err != nil {
		t.Fatalf("create attendances: %v", err)
	}
	if err := database.Exec(`CREATE TABLE task_records (
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
	if err := database.Exec(`INSERT INTO clients (id, fullname, company_name) VALUES ('client-1', 'Client', 'Company');`).Error; err != nil {
		t.Fatalf("insert client: %v", err)
	}
	if err := database.Exec(`INSERT INTO tasks (task_id, title, project_name, client_id) VALUES ('task-1', 'Title', 'Project', 'client-1');`).Error; err != nil {
		t.Fatalf("insert task: %v", err)
	}
	if err := database.Exec(`INSERT INTO attendances (id, user_id, time_in, sync_status) VALUES (1, 'user-1', CURRENT_TIMESTAMP, 'pending');`).Error; err != nil {
		t.Fatalf("insert attendance: %v", err)
	}
	if err := database.Exec(`INSERT INTO task_records (
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

	if err := runMigrations(database, ""); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	var userID string
	if err := database.Table("task_records").Select("user_id").Where("id = ?", 1).Scan(&userID).Error; err != nil {
		t.Fatalf("load repaired user_id: %v", err)
	}
	if userID != "user-1" {
		t.Fatalf("expected repaired user_id user-1, got %q", userID)
	}
}

func TestRunMigrations_RepairsCorruptedTaskAndScreenshotForeignKeys(t *testing.T) {
	database := newMemoryDB(t)

	if err := database.Exec(`CREATE TABLE clients (id TEXT PRIMARY KEY, fullname TEXT, company_name TEXT);`).Error; err != nil {
		t.Fatalf("create clients: %v", err)
	}
	if err := database.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY);`).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := database.Exec(`CREATE TABLE attendances (
		id INTEGER PRIMARY KEY,
		user_id TEXT NOT NULL,
		time_in DATETIME,
		sync_status TEXT
	);`).Error; err != nil {
		t.Fatalf("create attendances: %v", err)
	}
	if err := database.Exec(`CREATE TABLE tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT UNIQUE,
		title TEXT,
		project_name TEXT,
		client_id TEXT
	);`).Error; err != nil {
		t.Fatalf("create corrupted tasks: %v", err)
	}
	if err := database.Exec(`CREATE TABLE task_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		attendance_id INTEGER NOT NULL,
		task_id TEXT NOT NULL,
		started_at DATETIME NOT NULL,
		padagan INTEGER NOT NULL DEFAULT 0,
		notes TEXT NOT NULL DEFAULT '',
		ended_at DATETIME,
		duration INTEGER NOT NULL DEFAULT 0,
		synced_at DATETIME,
		productivity TEXT,
		sync_status TEXT NOT NULL DEFAULT 'pending',
		attempt_count INTEGER NOT NULL DEFAULT 0,
		last_error TEXT NOT NULL DEFAULT '',
		last_attempt_at DATETIME,
		gap INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY(user_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
		FOREIGN KEY(attendance_id) REFERENCES attendances(id) ON UPDATE CASCADE ON DELETE CASCADE,
		FOREIGN KEY(task_id) REFERENCES tasks(task_id) ON UPDATE CASCADE ON DELETE CASCADE
	);`).Error; err != nil {
		t.Fatalf("create task_records: %v", err)
	}
	if err := database.Exec(`CREATE TABLE screenshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_record_id INTEGER NOT NULL,
		task_id TEXT NOT NULL,
		memo TEXT NOT NULL DEFAULT '',
		notes TEXT NOT NULL DEFAULT '',
		file_path TEXT,
		cloud_url TEXT,
		taken_at DATETIME NOT NULL
	);`).Error; err != nil {
		t.Fatalf("create corrupted screenshots: %v", err)
	}

	if err := database.Exec(`INSERT INTO clients (id, fullname, company_name) VALUES ('client-1', 'Client', 'Company');`).Error; err != nil {
		t.Fatalf("insert client: %v", err)
	}
	if err := database.Exec(`INSERT INTO users (id) VALUES ('user-1');`).Error; err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := database.Exec(`INSERT INTO attendances (id, user_id, time_in, sync_status) VALUES (1, 'user-1', CURRENT_TIMESTAMP, 'pending');`).Error; err != nil {
		t.Fatalf("insert attendance: %v", err)
	}
	if err := database.Exec(`INSERT INTO tasks (id, task_id, title, project_name, client_id) VALUES (1, 'task-1', 'Same Name', 'Same Project', 'client-1');`).Error; err != nil {
		t.Fatalf("insert task: %v", err)
	}
	if err := database.Exec(`INSERT INTO task_records (id, user_id, attendance_id, task_id, started_at) VALUES (7, 'user-1', 1, 'task-1', CURRENT_TIMESTAMP);`).Error; err != nil {
		t.Fatalf("insert task record: %v", err)
	}
	if err := database.Exec(`INSERT INTO screenshots (id, task_record_id, task_id, memo, notes, file_path, cloud_url, taken_at) VALUES (9, 7, 'task-1', '', '', 'shot.png', NULL, CURRENT_TIMESTAMP);`).Error; err != nil {
		t.Fatalf("insert screenshot: %v", err)
	}

	if err := runMigrations(database, ""); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	taskFKs, err := foreignKeysForTable(database, "tasks")
	if err != nil {
		t.Fatalf("load task fks: %v", err)
	}
	if !tasksSchemaIsHealthy(taskFKs) {
		t.Fatalf("expected repaired tasks foreign keys, got %+v", taskFKs)
	}

	screenshotFKs, err := foreignKeysForTable(database, "screenshots")
	if err != nil {
		t.Fatalf("load screenshot fks: %v", err)
	}
	if !screenshotsSchemaIsHealthy(screenshotFKs) {
		t.Fatalf("expected repaired screenshots foreign keys, got %+v", screenshotFKs)
	}
}

func TestRunMigrations_FailsOnInvalidLegacyRows(t *testing.T) {
	t.Run("blank task id", func(t *testing.T) {
		database := newMemoryDB(t)

		if err := database.Exec(`CREATE TABLE clients (id TEXT PRIMARY KEY, fullname TEXT, company_name TEXT);`).Error; err != nil {
			t.Fatalf("create clients: %v", err)
		}
		if err := database.Exec(`CREATE TABLE tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id TEXT UNIQUE,
			title TEXT,
			project_name TEXT,
			client_id TEXT
		);`).Error; err != nil {
			t.Fatalf("create tasks: %v", err)
		}
		if err := database.Exec(`INSERT INTO tasks (id, task_id, title, project_name) VALUES (1, '   ', 'Bad', 'Legacy');`).Error; err != nil {
			t.Fatalf("insert invalid task: %v", err)
		}

		err := runMigrations(database, "")
		if err == nil || !strings.Contains(err.Error(), "blank task_id") {
			t.Fatalf("expected blank task_id failure, got %v", err)
		}
	})

	t.Run("null screenshot task_record_id", func(t *testing.T) {
		database := newMemoryDB(t)

		if err := database.Exec(`CREATE TABLE screenshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_record_id INTEGER,
			task_id TEXT NOT NULL,
			taken_at DATETIME NOT NULL
		);`).Error; err != nil {
			t.Fatalf("create screenshots: %v", err)
		}
		if err := database.Exec(`INSERT INTO screenshots (id, task_record_id, task_id, taken_at) VALUES (1, NULL, 'task-1', CURRENT_TIMESTAMP);`).Error; err != nil {
			t.Fatalf("insert invalid screenshot: %v", err)
		}

		err := runMigrations(database, "")
		if err == nil || !strings.Contains(err.Error(), "NULL task_record_id") {
			t.Fatalf("expected NULL task_record_id failure, got %v", err)
		}
	})
}

func TestNeedsMigrationBackup_UsesPendingMigrationPlans(t *testing.T) {
	t.Run("canonical db skips backup", func(t *testing.T) {
		database := newMemoryDB(t)
		if err := runMigrations(database, ""); err != nil {
			t.Fatalf("run migrations: %v", err)
		}

		needsBackup, err := needsMigrationBackup(database)
		if err != nil {
			t.Fatalf("needs migration backup: %v", err)
		}
		if needsBackup {
			t.Fatalf("expected canonical schema to skip backup")
		}
	})

	t.Run("legacy users with rows require backup", func(t *testing.T) {
		database := newMemoryDB(t)
		if err := database.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, birthday TEXT);`).Error; err != nil {
			t.Fatalf("create legacy users: %v", err)
		}
		if err := database.Exec(`INSERT INTO users (id, birthday) VALUES (1, '2000-01-02');`).Error; err != nil {
			t.Fatalf("insert legacy user: %v", err)
		}

		needsBackup, err := needsMigrationBackup(database)
		if err != nil {
			t.Fatalf("needs migration backup: %v", err)
		}
		if !needsBackup {
			t.Fatalf("expected legacy rebuild to require backup")
		}
	})
}

func TestRunMigrations_IsIdempotent(t *testing.T) {
	database := newMemoryDB(t)

	if err := runMigrations(database, ""); err != nil {
		t.Fatalf("first run migrations: %v", err)
	}
	if err := runMigrations(database, ""); err != nil {
		t.Fatalf("second run migrations: %v", err)
	}

	var ids []string
	if err := database.Table(gormMigrationsTable).Order("id ASC").Pluck("id", &ids).Error; err != nil {
		t.Fatalf("load migration ids: %v", err)
	}
	if len(ids) != len(migrationCatalog()) {
		t.Fatalf("expected %d migration ids, got %d", len(migrationCatalog()), len(ids))
	}
}
