package db

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

const latestSchemaVersion = 3

type schemaMigration struct {
	Version    int       `gorm:"primaryKey"`
	Name       string    `gorm:"not null"`
	AppliedAt  time.Time `gorm:"not null"`
	Successful bool      `gorm:"not null;default:false"`
}

type migrationStep struct {
	version int
	name    string
	up      func(tx *gorm.DB) error
	down    func(tx *gorm.DB) error
	// Some SQLite schema repairs need to toggle PRAGMA foreign_keys and rebuild tables.
	// Those operations must run outside an explicit transaction.
	transactional bool
}

func migrationPlan() []migrationStep {
	return []migrationStep{
		{
			version: 1,
			name:    "repair_users_schema",
			up: func(tx *gorm.DB) error {
				return repairUsersSchema(tx)
			},
			down: func(tx *gorm.DB) error {
				return nil
			},
			transactional: true,
		},
		{
			version: 2,
			name:    "hot_query_indexes",
			up:      applyHotQueryIndexes,
			down:    dropHotQueryIndexes,
			transactional: true,
		},
		{
			version: 3,
			name:    "repair_task_tracking_schema",
			up:      repairTaskTrackingSchema,
			down: func(tx *gorm.DB) error {
				return nil
			},
			transactional: false,
		},
	}
}

func repairTaskTrackingSchema(database *gorm.DB) error {
	if database == nil {
		return nil
	}

	if err := database.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
		return err
	}
	defer database.Exec("PRAGMA foreign_keys = ON")

	if database.Migrator().HasTable("tasks") {
		if err := database.Exec("DROP TABLE IF EXISTS tasks_rebuilt").Error; err != nil {
			return err
		}
		if err := database.Exec(`
CREATE TABLE IF NOT EXISTS tasks_rebuilt (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	task_id TEXT,
	title TEXT,
	project_name TEXT,
	client_id TEXT,
	CONSTRAINT uni_tasks_task_id UNIQUE (task_id),
	CONSTRAINT fk_clients_tasks FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE CASCADE ON UPDATE CASCADE
)`).Error; err != nil {
			return err
		}
		if err := database.Exec(`
INSERT INTO tasks_rebuilt (id, task_id, title, project_name, client_id)
SELECT id, CAST(task_id AS TEXT), title, project_name, client_id FROM tasks`).Error; err != nil {
			return err
		}
		if err := database.Exec("DROP TABLE tasks").Error; err != nil {
			return err
		}
		if err := database.Exec("ALTER TABLE tasks_rebuilt RENAME TO tasks").Error; err != nil {
			return err
		}
	}

	if database.Migrator().HasTable("task_records") {
		columns, err := migrationTableColumns(database, "task_records")
		if err != nil {
			return err
		}
		padaganExpr := migrationColumnExpr(columns, "padagan", "false")
		notesExpr := migrationColumnExpr(columns, "notes", `""`)
		durationExpr := migrationColumnExpr(columns, "duration", "0")
		syncedAtExpr := migrationColumnExpr(columns, "synced_at", "NULL")
		productivityExpr := migrationColumnExpr(columns, "productivity", "NULL")
		attemptCountExpr := migrationColumnExpr(columns, "attempt_count", "0")
		lastErrorExpr := migrationColumnExpr(columns, "last_error", `""`)
		lastAttemptAtExpr := migrationColumnExpr(columns, "last_attempt_at", "NULL")
		gapExpr := migrationColumnExpr(columns, "gap", "0")

		if err := database.Exec("DROP TABLE IF EXISTS task_records_rebuilt").Error; err != nil {
			return err
		}
		if err := database.Exec(`
CREATE TABLE IF NOT EXISTS task_records_rebuilt (
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
	CONSTRAINT fk_task_records_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE,
	CONSTRAINT fk_task_records_attendance FOREIGN KEY (attendance_id) REFERENCES attendances(id) ON DELETE CASCADE ON UPDATE CASCADE
)`).Error; err != nil {
			return err
		}
		if err := database.Exec(fmt.Sprintf(`
INSERT INTO task_records_rebuilt (
	id, user_id, attendance_id, task_id, started_at, padagan, notes, ended_at, duration,
	synced_at, productivity, sync_status, attempt_count, last_error, last_attempt_at, gap
)
SELECT
	id, user_id, attendance_id, CAST(task_id AS TEXT), started_at, %s, COALESCE(%s, ""), ended_at, COALESCE(%s, 0),
	%s, %s, COALESCE(sync_status, "pending"), COALESCE(%s, 0),
	COALESCE(%s, ""), %s, COALESCE(%s, 0)
FROM task_records`,
			padaganExpr,
			notesExpr,
			durationExpr,
			syncedAtExpr,
			productivityExpr,
			attemptCountExpr,
			lastErrorExpr,
			lastAttemptAtExpr,
			gapExpr,
		)).Error; err != nil {
			return err
		}
		if err := database.Exec("DROP TABLE task_records").Error; err != nil {
			return err
		}
		if err := database.Exec("ALTER TABLE task_records_rebuilt RENAME TO task_records").Error; err != nil {
			return err
		}
	}

	if database.Migrator().HasTable("screenshots") {
		columns, err := migrationTableColumns(database, "screenshots")
		if err != nil {
			return err
		}
		memoExpr := migrationColumnExpr(columns, "memo", `""`)
		notesExpr := migrationColumnExpr(columns, "notes", `""`)
		filePathExpr := migrationColumnExpr(columns, "file_path", "NULL")
		cloudURLExpr := migrationColumnExpr(columns, "cloud_url", "NULL")

		if err := database.Exec("DROP TABLE IF EXISTS screenshots_rebuilt").Error; err != nil {
			return err
		}
		if err := database.Exec(`
CREATE TABLE IF NOT EXISTS screenshots_rebuilt (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	task_record_id INTEGER NOT NULL,
	task_id TEXT NOT NULL,
	memo TEXT NOT NULL DEFAULT "",
	notes TEXT NOT NULL DEFAULT "",
	file_path TEXT,
	cloud_url TEXT,
	taken_at DATETIME NOT NULL,
	CONSTRAINT fk_task_records_screenshots FOREIGN KEY (task_record_id) REFERENCES task_records(id) ON DELETE CASCADE ON UPDATE CASCADE
)`).Error; err != nil {
			return err
		}
		if err := database.Exec(fmt.Sprintf(`
INSERT INTO screenshots_rebuilt (id, task_record_id, task_id, memo, notes, file_path, cloud_url, taken_at)
SELECT id, task_record_id, CAST(task_id AS TEXT), COALESCE(%s, ""), COALESCE(%s, ""), %s, %s, taken_at
FROM screenshots`,
			memoExpr,
			notesExpr,
			filePathExpr,
			cloudURLExpr,
		)).Error; err != nil {
			return err
		}
		if err := database.Exec("DROP TABLE screenshots").Error; err != nil {
			return err
		}
		if err := database.Exec("ALTER TABLE screenshots_rebuilt RENAME TO screenshots").Error; err != nil {
			return err
		}
	}

	return applyHotQueryIndexes(database)
}

func migrationTableColumns(database *gorm.DB, table string) (map[string]struct{}, error) {
	var columns []tableInfoRow
	if err := database.Raw("PRAGMA table_info(" + table + ")").Scan(&columns).Error; err != nil {
		return nil, err
	}

	result := make(map[string]struct{}, len(columns))
	for _, column := range columns {
		result[strings.ToLower(column.Name)] = struct{}{}
	}
	return result, nil
}

func migrationColumnExpr(columns map[string]struct{}, columnName, fallback string) string {
	if _, ok := columns[strings.ToLower(columnName)]; ok {
		return columnName
	}
	return fallback
}

func migrateSchema(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("nil database")
	}

	if err := database.AutoMigrate(&schemaMigration{}); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	current, err := currentSchemaVersion(database)
	if err != nil {
		return err
	}
	if current > latestSchemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d", current, latestSchemaVersion)
	}

	for _, step := range migrationPlan() {
		if step.version <= current {
			continue
		}
		if err := runMigrationStep(database, step); err != nil {
			return err
		}
	}

	return nil
}

func currentSchemaVersion(database *gorm.DB) (int, error) {
	var row schemaMigration
	err := database.
		Where("successful = ?", true).
		Order("version DESC").
		First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, fmt.Errorf("resolve current schema version: %w", err)
	}
	return row.Version, nil
}

func runMigrationStep(database *gorm.DB, step migrationStep) error {
	appliedAt := time.Now().UTC()
	record := schemaMigration{
		Version:    step.version,
		Name:       step.name,
		AppliedAt:  appliedAt,
		Successful: false,
	}
	if step.transactional {
		tx := database.Begin()
		if tx.Error != nil {
			return tx.Error
		}

		if err := tx.Create(&record).Error; err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("create migration record v%d: %w", step.version, err)
		}

		if err := step.up(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration v%d (%s): %w", step.version, step.name, err)
		}

		if err := tx.Model(&schemaMigration{}).
			Where("version = ?", step.version).
			Update("successful", true).Error; err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("mark migration v%d successful: %w", step.version, err)
		}

		if err := tx.Commit().Error; err != nil {
			return fmt.Errorf("commit migration v%d: %w", step.version, err)
		}

		return nil
	}

	if err := database.Create(&record).Error; err != nil {
		return fmt.Errorf("create migration record v%d: %w", step.version, err)
	}

	if err := step.up(database); err != nil {
		return fmt.Errorf("apply migration v%d (%s): %w", step.version, step.name, err)
	}

	if err := database.Model(&schemaMigration{}).
		Where("version = ?", step.version).
		Update("successful", true).Error; err != nil {
		return fmt.Errorf("mark migration v%d successful: %w", step.version, err)
	}

	return nil
}
