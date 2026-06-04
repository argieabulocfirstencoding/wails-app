package db

import (
	"reflect"
	"strings"
	"testing"

	"fesapp/backend/db/models"
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

func TestAutoMigrateScreenshot_DoesNotTouchTaskRecords(t *testing.T) {
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
	if err := database.Exec(`CREATE TABLE task_records (id INTEGER PRIMARY KEY, user_id TEXT NOT NULL, attendance_id INTEGER NOT NULL, task_id TEXT NOT NULL, started_at DATETIME NOT NULL, sync_status TEXT);`).Error; err != nil {
		t.Fatalf("create task_records: %v", err)
	}

	if err := database.AutoMigrate(&models.Screenshot{}); err != nil {
		t.Fatalf("auto migrate screenshot: %v", err)
	}

	if !database.Migrator().HasTable("screenshots") {
		t.Fatalf("expected screenshots table to be created")
	}
	if !database.Migrator().HasColumn("screenshots", "task_record_id") {
		t.Fatalf("expected screenshots.task_record_id column")
	}
	if !database.Migrator().HasColumn("screenshots", "task_id") {
		t.Fatalf("expected screenshots.task_id column")
	}
}

func TestAutoMigrateTask_DoesNotTouchTaskRecords(t *testing.T) {
	database := newMemoryDB(t)

	if err := database.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY);`).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := database.Exec(`CREATE TABLE tasks (id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT UNIQUE, title TEXT, project_name TEXT, client_id TEXT);`).Error; err != nil {
		t.Fatalf("create tasks: %v", err)
	}
	if err := database.Exec(`CREATE TABLE attendances (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id TEXT NOT NULL);`).Error; err != nil {
		t.Fatalf("create attendances: %v", err)
	}
	if err := database.Exec(`CREATE TABLE task_records (id INTEGER PRIMARY KEY, user_id TEXT, attendance_id INTEGER, task_id TEXT, started_at DATETIME, sync_status TEXT);`).Error; err != nil {
		t.Fatalf("create task_records: %v", err)
	}

	if err := database.AutoMigrate(&models.Task{}); err != nil {
		t.Fatalf("auto migrate task: %v", err)
	}

	var columns []tableInfoRow
	if err := database.Raw("PRAGMA table_info(task_records)").Scan(&columns).Error; err != nil {
		t.Fatalf("inspect task_records columns: %v", err)
	}
	if len(columns) != 6 {
		t.Fatalf("expected task_records to remain unchanged with 6 columns, got %d", len(columns))
	}
}

func TestBootstrapCoreSchema_DoesNotTouchTaskRecords(t *testing.T) {
	database := newMemoryDB(t)

	if err := database.Exec(`CREATE TABLE task_records (id INTEGER PRIMARY KEY, user_id TEXT, attendance_id INTEGER, task_id TEXT, started_at DATETIME, sync_status TEXT);`).Error; err != nil {
		t.Fatalf("create task_records: %v", err)
	}

	if err := bootstrapCoreSchema(database); err != nil {
		t.Fatalf("bootstrap core schema: %v", err)
	}

	var columns []tableInfoRow
	if err := database.Raw("PRAGMA table_info(task_records)").Scan(&columns).Error; err != nil {
		t.Fatalf("inspect task_records columns: %v", err)
	}
	if len(columns) != 6 {
		t.Fatalf("expected task_records to remain unchanged with 6 columns, got %d", len(columns))
	}
}

func TestRepairTaskRecordsSchema_SkipsHealthyTable(t *testing.T) {
	database := newMemoryDB(t)

	if err := database.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY);`).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := database.Exec(`CREATE TABLE attendances (id INTEGER PRIMARY KEY, user_id TEXT NOT NULL);`).Error; err != nil {
		t.Fatalf("create attendances: %v", err)
	}
	if err := database.Exec(`CREATE TABLE tasks (task_id TEXT PRIMARY KEY);`).Error; err != nil {
		t.Fatalf("create tasks: %v", err)
	}
	if err := database.Exec(`
		CREATE TABLE task_records (
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

	needsRepair, err := taskRecordsSchemaNeedsRepair(database)
	if err != nil {
		t.Fatalf("detect task_records repair: %v", err)
	}
	if needsRepair {
		t.Fatalf("expected healthy task_records schema to skip repair")
	}

	if err := repairTaskRecordsSchema(database); err != nil {
		t.Fatalf("repair healthy task_records schema: %v", err)
	}

	if database.Migrator().HasTable("task_records_legacy") {
		t.Fatalf("expected no legacy table after healthy repair no-op")
	}
}

func TestNeedsMigrationBackup_SkipsLatestHealthySchema(t *testing.T) {
	database := newMemoryDB(t)

	if err := bootstrapCoreSchema(database); err != nil {
		t.Fatalf("bootstrap core schema: %v", err)
	}
	if err := repairTaskRecordsSchema(database); err != nil {
		t.Fatalf("repair task_records: %v", err)
	}
	if err := bootstrapScreenshotSchema(database); err != nil {
		t.Fatalf("bootstrap screenshots: %v", err)
	}
	if err := migrateSchema(database); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}

	needsBackup, err := needsMigrationBackup(database)
	if err != nil {
		t.Fatalf("needs migration backup: %v", err)
	}
	if needsBackup {
		t.Fatalf("expected latest healthy schema to skip backup")
	}
}

func TestTaskRecordsSchemaNeedsRepair_DetectsWrongDefaultsAndFKActions(t *testing.T) {
	database := newMemoryDB(t)

	if err := database.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY);`).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := database.Exec(`CREATE TABLE attendances (id INTEGER PRIMARY KEY, user_id TEXT NOT NULL);`).Error; err != nil {
		t.Fatalf("create attendances: %v", err)
	}
	if err := database.Exec(`CREATE TABLE tasks (task_id TEXT PRIMARY KEY);`).Error; err != nil {
		t.Fatalf("create tasks: %v", err)
	}
	if err := database.Exec(`
		CREATE TABLE task_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			attendance_id INTEGER NOT NULL,
			task_id TEXT NOT NULL,
			started_at DATETIME NOT NULL,
			padagan INTEGER NOT NULL DEFAULT 1,
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
			FOREIGN KEY(user_id) REFERENCES users(id),
			FOREIGN KEY(attendance_id) REFERENCES attendances(id),
			FOREIGN KEY(task_id) REFERENCES tasks(task_id)
		);`).Error; err != nil {
		t.Fatalf("create task_records: %v", err)
	}

	needsRepair, err := taskRecordsSchemaNeedsRepair(database)
	if err != nil {
		t.Fatalf("detect task_records repair: %v", err)
	}
	if !needsRepair {
		t.Fatalf("expected wrong defaults and FK actions to require repair")
	}
}

func TestBootstrapSchemasMatchRuntimeModels(t *testing.T) {
	assertStructParity(t, bootstrapUser{}, models.User{}, []string{"Attendance"})
	assertStructParity(t, bootstrapTask{}, models.Task{}, []string{"Client"})
	assertStructParity(t, bootstrapAttendance{}, models.Attendance{}, []string{"User", "Records"})
	assertStructParity(t, bootstrapScreenshot{}, models.Screenshot{}, nil)
}

func assertStructParity(t *testing.T, bootstrap, runtime any, ignored []string) {
	t.Helper()

	ignore := make(map[string]struct{}, len(ignored))
	for _, name := range ignored {
		ignore[name] = struct{}{}
	}

	bootstrapType := reflect.TypeOf(bootstrap)
	runtimeType := reflect.TypeOf(runtime)

	runtimeFields := collectStructFields(runtimeType, ignore)

	for i := 0; i < bootstrapType.NumField(); i++ {
		field := bootstrapType.Field(i)
		runtimeField, ok := runtimeFields[field.Name]
		if !ok {
			t.Fatalf("bootstrap field %s missing from runtime model %s", field.Name, runtimeType.Name())
		}
		if field.Type != runtimeField.Type {
			t.Fatalf("field %s type mismatch: bootstrap=%s runtime=%s", field.Name, field.Type, runtimeField.Type)
		}
		if normalizeTag(field.Tag.Get("gorm")) != normalizeTag(runtimeField.Tag.Get("gorm")) {
			t.Fatalf("field %s gorm tag mismatch: bootstrap=%q runtime=%q", field.Name, field.Tag.Get("gorm"), runtimeField.Tag.Get("gorm"))
		}
		delete(runtimeFields, field.Name)
	}

	for name := range runtimeFields {
		t.Fatalf("runtime field %s missing from bootstrap model %s", name, bootstrapType.Name())
	}
}

func collectStructFields(structType reflect.Type, ignore map[string]struct{}) map[string]reflect.StructField {
	fields := make(map[string]reflect.StructField)

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if _, skipped := ignore[field.Name]; skipped {
			continue
		}

		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			for name, nested := range collectStructFields(field.Type, ignore) {
				fields[name] = nested
			}
			continue
		}

		fields[field.Name] = field
	}

	return fields
}

func normalizeTag(tag string) string {
	return strings.ToLower(strings.TrimSpace(tag))
}

func TestMigrateSchema_RepairsCorruptedTaskAndScreenshotForeignKeys(t *testing.T) {
	database := newMemoryDB(t)

	if err := database.Exec(`CREATE TABLE clients (id TEXT PRIMARY KEY);`).Error; err != nil {
		t.Fatalf("create clients: %v", err)
	}
	if err := database.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY);`).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := database.Exec(`CREATE TABLE attendances (id INTEGER PRIMARY KEY, user_id TEXT NOT NULL, time_in DATETIME, sync_status TEXT);`).Error; err != nil {
		t.Fatalf("create attendances: %v", err)
	}
	if err := database.Exec(`CREATE TABLE task_records_legacy (id INTEGER PRIMARY KEY, task_id TEXT);`).Error; err != nil {
		t.Fatalf("create task_records_legacy: %v", err)
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
		t.Fatalf("create screenshots shell: %v", err)
	}
	if err := database.Exec(`ALTER TABLE screenshots RENAME TO screenshots_legacy_ref;`).Error; err != nil {
		t.Fatalf("rename screenshots shell: %v", err)
	}
	if err := database.Exec(`CREATE TABLE screenshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_record_id INTEGER NOT NULL,
		task_id TEXT NOT NULL,
		memo TEXT NOT NULL DEFAULT '',
		notes TEXT NOT NULL DEFAULT '',
		file_path TEXT,
		cloud_url TEXT,
		taken_at DATETIME NOT NULL,
		CONSTRAINT fk_task_records_screenshots FOREIGN KEY (task_record_id) REFERENCES task_records_legacy(id)
	);`).Error; err != nil {
		t.Fatalf("create corrupted screenshots: %v", err)
	}
	if err := database.Exec(`CREATE TABLE tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT UNIQUE,
		title TEXT,
		project_name TEXT,
		client_id TEXT,
		CONSTRAINT fk_clients_tasks FOREIGN KEY (client_id) REFERENCES clients(id),
		CONSTRAINT fk_task_records_task FOREIGN KEY (task_id) REFERENCES task_records_legacy(task_id) ON DELETE CASCADE ON UPDATE CASCADE,
		CONSTRAINT fk_screenshots_task FOREIGN KEY (task_id) REFERENCES screenshots(task_id) ON DELETE CASCADE ON UPDATE CASCADE
	);`).Error; err != nil {
		t.Fatalf("create corrupted tasks: %v", err)
	}

	if err := database.Exec(`INSERT INTO clients (id) VALUES ('client-1');`).Error; err != nil {
		t.Fatalf("insert client: %v", err)
	}
	if err := database.Exec(`INSERT INTO users (id) VALUES ('user-1');`).Error; err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := database.Exec(`INSERT INTO attendances (id, user_id, time_in, sync_status) VALUES (1, 'user-1', CURRENT_TIMESTAMP, 'pending');`).Error; err != nil {
		t.Fatalf("insert attendance: %v", err)
	}
	if err := database.Exec(`INSERT INTO tasks (id, task_id, title, project_name, client_id) VALUES (1, 'task-1', 'Same Name', 'Same Project', 'client-1');`).Error; err != nil {
		t.Fatalf("insert task-1: %v", err)
	}
	if err := database.Exec(`INSERT INTO tasks (id, task_id, title, project_name, client_id) VALUES (2, 'task-2', 'Same Name', 'Same Project', 'client-1');`).Error; err != nil {
		t.Fatalf("insert task-2: %v", err)
	}
	if err := database.Exec(`CREATE TABLE task_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		attendance_id INTEGER NOT NULL,
		task_id TEXT NOT NULL,
		started_at DATETIME NOT NULL,
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
		gap INTEGER,
		FOREIGN KEY(user_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
		FOREIGN KEY(attendance_id) REFERENCES attendances(id) ON UPDATE CASCADE ON DELETE CASCADE,
		FOREIGN KEY(task_id) REFERENCES tasks(task_id) ON UPDATE CASCADE ON DELETE CASCADE
	);`).Error; err != nil {
		t.Fatalf("create task_records: %v", err)
	}
	if err := database.Exec(`INSERT INTO task_records (id, user_id, attendance_id, task_id, started_at) VALUES (7, 'user-1', 1, 'task-1', CURRENT_TIMESTAMP);`).Error; err != nil {
		t.Fatalf("insert task record: %v", err)
	}
	if err := database.Exec(`INSERT INTO screenshots (id, task_record_id, task_id, memo, notes, file_path, cloud_url, taken_at) VALUES (9, 7, 'task-1', '', '', 'shot.png', NULL, CURRENT_TIMESTAMP);`).Error; err != nil {
		t.Fatalf("insert screenshot: %v", err)
	}

	if err := migrateSchema(database); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}

	var taskFKs []foreignKeyRow
	if err := database.Raw(`PRAGMA foreign_key_list("tasks")`).Scan(&taskFKs).Error; err != nil {
		t.Fatalf("load tasks fks: %v", err)
	}
	if !tasksSchemaIsHealthy(taskFKs) {
		t.Fatalf("expected repaired tasks foreign keys, got %+v", taskFKs)
	}

	var screenshotFKs []foreignKeyRow
	if err := database.Raw(`PRAGMA foreign_key_list("screenshots")`).Scan(&screenshotFKs).Error; err != nil {
		t.Fatalf("load screenshots fks: %v", err)
	}
	if !screenshotsSchemaIsHealthy(screenshotFKs) {
		t.Fatalf("expected repaired screenshots foreign keys, got %+v", screenshotFKs)
	}

	var taskRecordFKs []foreignKeyRow
	if err := database.Raw(`PRAGMA foreign_key_list("task_records")`).Scan(&taskRecordFKs).Error; err != nil {
		t.Fatalf("load task_records fks: %v", err)
	}
	var foundTaskFK bool
	for _, fk := range taskRecordFKs {
		if fk.Table == "tasks" && fk.From == "task_id" && fk.To == "task_id" {
			foundTaskFK = true
			break
		}
	}
	if !foundTaskFK {
		t.Fatalf("expected task_records.task_id to reference tasks(task_id), got %+v", taskRecordFKs)
	}

	var taskCount int64
	if err := database.Table("tasks").Where("title = ? AND project_name = ?", "Same Name", "Same Project").Count(&taskCount).Error; err != nil {
		t.Fatalf("count duplicate-name tasks: %v", err)
	}
	if taskCount != 2 {
		t.Fatalf("expected duplicate-name tasks to survive repair, got %d", taskCount)
	}

	var screenshotCount int64
	if err := database.Table("screenshots").Count(&screenshotCount).Error; err != nil {
		t.Fatalf("count screenshots: %v", err)
	}
	if screenshotCount != 1 {
		t.Fatalf("expected screenshot row to survive repair, got %d", screenshotCount)
	}

	if err := migrateSchema(database); err != nil {
		t.Fatalf("second migrate schema: %v", err)
	}
}

func TestRepairTaskAndScreenshotSchemas_FailsOnBlankTaskID(t *testing.T) {
	database := newMemoryDB(t)

	if err := database.Exec(`CREATE TABLE clients (id TEXT PRIMARY KEY);`).Error; err != nil {
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
	if err := database.Exec(`INSERT INTO tasks (id, task_id, title, project_name, client_id) VALUES (1, '   ', 'Bad', 'Legacy', NULL);`).Error; err != nil {
		t.Fatalf("insert blank task_id task: %v", err)
	}

	err := repairTaskAndScreenshotSchemas(database)
	if err == nil {
		t.Fatalf("expected blank task_id repair failure")
	}
}

func TestRepairTaskAndScreenshotSchemas_FailsOnNullScreenshotTaskRecordID(t *testing.T) {
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

	err := repairTaskAndScreenshotSchemas(database)
	if err == nil {
		t.Fatalf("expected null task_record_id repair failure")
	}
}
