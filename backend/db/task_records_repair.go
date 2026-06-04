package db

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

func repairTaskRecordsSchema(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("nil database")
	}

	return withSQLiteForeignKeysDisabled(database, repairTaskRecordsSchemaTx)
}

func withSQLiteForeignKeysDisabled(database *gorm.DB, fn func(tx *gorm.DB) error) error {
	return database.WithContext(context.Background()).Connection(func(conn *gorm.DB) error {
		if err := conn.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
			return err
		}
		defer conn.Exec("PRAGMA foreign_keys = ON")

		return conn.Transaction(func(tx *gorm.DB) error {
			return fn(tx)
		})
	})
}

func repairTaskRecordsSchemaTx(tx *gorm.DB) error {
	needsRepair, err := taskRecordsSchemaNeedsRepairTx(tx)
	if err != nil {
		return err
	}
	if !needsRepair {
		return nil
	}

	if tx.Migrator().HasTable("task_records") {
		if err := tx.Exec("DROP TABLE IF EXISTS task_records_legacy").Error; err != nil {
			return err
		}
		if err := tx.Exec("ALTER TABLE task_records RENAME TO task_records_legacy").Error; err != nil {
			return err
		}
	}

	createSQL := `
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
);`

	if err := tx.Exec(createSQL).Error; err != nil {
		return err
	}

	if !tx.Migrator().HasTable("task_records_legacy") {
		return nil
	}

	copySQL := `
INSERT INTO task_records (
	id,
	user_id,
	attendance_id,
	task_id,
	started_at,
	padagan,
	notes,
	ended_at,
	duration,
	synced_at,
	productivity,
	sync_status,
	attempt_count,
	last_error,
	last_attempt_at,
	gap
)
SELECT
	tr.id,
	COALESCE(NULLIF(TRIM(tr.user_id), ''), a.user_id) AS user_id,
	tr.attendance_id,
	tr.task_id,
	COALESCE(tr.started_at, CURRENT_TIMESTAMP),
	COALESCE(tr.padagan, 0),
	COALESCE(tr.notes, ''),
	tr.ended_at,
	COALESCE(tr.duration, 0),
	tr.synced_at,
	tr.productivity,
	COALESCE(NULLIF(TRIM(tr.sync_status), ''), 'pending'),
	COALESCE(tr.attempt_count, 0),
	COALESCE(tr.last_error, ''),
	tr.last_attempt_at,
	COALESCE(tr.gap, 0)
FROM task_records_legacy AS tr
LEFT JOIN attendances AS a ON a.id = tr.attendance_id
INNER JOIN tasks AS t ON t.task_id = tr.task_id
WHERE COALESCE(NULLIF(TRIM(tr.user_id), ''), a.user_id) IS NOT NULL;`

	if err := tx.Exec(copySQL).Error; err != nil {
		return err
	}

	if err := tx.Exec("DROP TABLE task_records_legacy").Error; err != nil {
		return err
	}

	return nil
}

type tableInfoDetailRow struct {
	Name         string  `gorm:"column:name"`
	Type         string  `gorm:"column:type"`
	NotNull      int     `gorm:"column:notnull"`
	DefaultValue *string `gorm:"column:dflt_value"`
}

func taskRecordsSchemaNeedsRepair(database *gorm.DB) (bool, error) {
	if database == nil {
		return false, nil
	}

	return taskRecordsSchemaNeedsRepairTx(database)
}

func taskRecordsSchemaNeedsRepairTx(tx *gorm.DB) (bool, error) {
	if !tx.Migrator().HasTable("task_records") {
		return true, nil
	}

	var columns []tableInfoDetailRow
	if err := tx.Raw(`PRAGMA table_info("task_records")`).Scan(&columns).Error; err != nil {
		return false, err
	}
	if len(columns) == 0 {
		return true, nil
	}

	columnMap := make(map[string]tableInfoDetailRow, len(columns))
	for _, col := range columns {
		columnMap[strings.ToLower(col.Name)] = col
	}

	requiredColumns := []string{
		"id",
		"user_id",
		"attendance_id",
		"task_id",
		"started_at",
		"padagan",
		"notes",
		"ended_at",
		"duration",
		"synced_at",
		"productivity",
		"sync_status",
		"attempt_count",
		"last_error",
		"last_attempt_at",
		"gap",
	}
	for _, name := range requiredColumns {
		if _, ok := columnMap[name]; !ok {
			return true, nil
		}
	}

	requiredNotNull := []string{"user_id", "attendance_id", "task_id", "started_at", "sync_status", "attempt_count", "last_error", "gap"}
	for _, name := range requiredNotNull {
		if columnMap[name].NotNull == 0 {
			return true, nil
		}
	}

	expectedDefaults := map[string]string{
		"padagan":       "0",
		"notes":         "",
		"duration":      "0",
		"sync_status":   "pending",
		"attempt_count": "0",
		"last_error":    "",
		"gap":           "0",
	}
	for name, expected := range expectedDefaults {
		if normalizeSQLiteDefaultValue(columnMap[name].DefaultValue) != expected {
			return true, nil
		}
	}

	fks, err := foreignKeysForTable(tx, "task_records")
	if err != nil {
		return false, err
	}

	expectedFKs := map[string]struct {
		table    string
		to       string
		onUpdate string
		onDelete string
	}{
		"user_id":       {table: "users", to: "id", onUpdate: "CASCADE", onDelete: "CASCADE"},
		"attendance_id": {table: "attendances", to: "id", onUpdate: "CASCADE", onDelete: "CASCADE"},
		"task_id":       {table: "tasks", to: "task_id", onUpdate: "CASCADE", onDelete: "CASCADE"},
	}

	if len(fks) != len(expectedFKs) {
		return true, nil
	}

	for _, fk := range fks {
		expected, ok := expectedFKs[strings.ToLower(fk.From)]
		if !ok {
			return true, nil
		}
		if !strings.EqualFold(fk.Table, expected.table) ||
			!strings.EqualFold(fk.To, expected.to) ||
			!strings.EqualFold(fk.OnUpdate, expected.onUpdate) ||
			!strings.EqualFold(fk.OnDelete, expected.onDelete) {
			return true, nil
		}
	}

	return false, nil
}

func normalizeSQLiteDefaultValue(value *string) string {
	if value == nil {
		return ""
	}

	normalized := strings.TrimSpace(*value)
	normalized = strings.Trim(normalized, "()")
	normalized = strings.Trim(normalized, "'\"")
	return normalized
}
