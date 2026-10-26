package db

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

func withSQLiteForeignKeysDisabled(database *gorm.DB, fn func(tx *gorm.DB) error) error {
	return database.WithContext(context.Background()).Connection(func(conn *gorm.DB) error {
		if err := conn.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
			return err
		}
		defer conn.Exec("PRAGMA foreign_keys = ON")

		return conn.Transaction(func(tx *gorm.DB) error {
			if err := fn(tx); err != nil {
				return err
			}
			return runSQLiteForeignKeyCheck(tx)
		})
	})
}

type sqliteRebuildSpec struct {
	Table          string
	TempTable      string
	CreateSQL      string
	CopySQL        string
	ValidateLegacy func(*gorm.DB) error
	AfterRenameSQL []string
}

func rebuildSQLiteTable(database *gorm.DB, spec sqliteRebuildSpec) error {
	if database == nil {
		return fmt.Errorf("nil database")
	}

	return withSQLiteForeignKeysDisabled(database, func(tx *gorm.DB) error {
		if spec.ValidateLegacy != nil {
			if err := spec.ValidateLegacy(tx); err != nil {
				return err
			}
		}

		if err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", quoteSQLiteIdentifier(spec.TempTable))).Error; err != nil {
			return err
		}
		if err := tx.Exec(spec.CreateSQL).Error; err != nil {
			return err
		}

		if tx.Migrator().HasTable(spec.Table) {
			if strings.TrimSpace(spec.CopySQL) != "" {
				if err := tx.Exec(spec.CopySQL).Error; err != nil {
					return err
				}
			}
			if err := tx.Exec(fmt.Sprintf("DROP TABLE %s", quoteSQLiteIdentifier(spec.Table))).Error; err != nil {
				return err
			}
		}

		if err := tx.Exec(fmt.Sprintf(
			"ALTER TABLE %s RENAME TO %s",
			quoteSQLiteIdentifier(spec.TempTable),
			quoteSQLiteIdentifier(spec.Table),
		)).Error; err != nil {
			return err
		}

		for _, stmt := range spec.AfterRenameSQL {
			if err := tx.Exec(stmt).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func runSQLiteForeignKeyCheck(tx *gorm.DB) error {
	type violationRow struct {
		Table  string `gorm:"column:table"`
		RowID  int64  `gorm:"column:rowid"`
		Parent string `gorm:"column:parent"`
		FKID   int64  `gorm:"column:fkid"`
	}

	var violations []violationRow
	if err := tx.Raw("PRAGMA foreign_key_check").Scan(&violations).Error; err != nil {
		return err
	}
	if len(violations) == 0 {
		return nil
	}

	first := violations[0]
	return fmt.Errorf(
		"foreign key validation failed: table=%s rowid=%d parent=%s fkid=%d",
		first.Table,
		first.RowID,
		first.Parent,
		first.FKID,
	)
}

func planTaskRecordsMigration(database *gorm.DB) (schemaPlan, error) {
	if database == nil {
		return schemaPlan{}, fmt.Errorf("nil database")
	}

	needsRepair, err := taskRecordsSchemaNeedsRepair(database)
	if err != nil {
		return schemaPlan{}, err
	}
	if !needsRepair {
		return schemaPlan{Action: migrationActionNoop}, nil
	}
	if !database.Migrator().HasTable("task_records") {
		return schemaPlan{Action: migrationActionCreate}, nil
	}

	hasRows, err := tableHasRows(database, "task_records")
	if err != nil {
		return schemaPlan{}, err
	}

	return schemaPlan{
		Action:         migrationActionRebuild,
		RequiresBackup: hasRows,
	}, nil
}

func ensureTaskRecordsSchema(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("nil database")
	}

	needsRepair, err := taskRecordsSchemaNeedsRepair(database)
	if err != nil {
		return err
	}
	if !needsRepair {
		return nil
	}

	clientSessionID := "NULL"
	taskInAttemptedAt := "NULL"
	taskInIdempotent := "0"
	if database.Migrator().HasTable("task_records") {
		if database.Migrator().HasColumn("task_records", "client_session_id") {
			clientSessionID = "tr.client_session_id"
		}
		if database.Migrator().HasColumn("task_records", "task_in_attempted_at") {
			taskInAttemptedAt = "tr.task_in_attempted_at"
		}
		if database.Migrator().HasColumn("task_records", "task_in_idempotent") {
			taskInIdempotent = "COALESCE(tr.task_in_idempotent, 0)"
		}
	}

	copySQL := fmt.Sprintf(`
INSERT INTO new_task_records (
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
	client_session_id,
	task_in_attempted_at,
	task_in_idempotent,
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
	%s,
	%s,
	%s,
	COALESCE(NULLIF(TRIM(tr.sync_status), ''), 'pending'),
	COALESCE(tr.attempt_count, 0),
	COALESCE(tr.last_error, ''),
	tr.last_attempt_at,
	COALESCE(tr.gap, 0)
FROM task_records AS tr
LEFT JOIN attendances AS a ON a.id = tr.attendance_id
INNER JOIN tasks AS t ON t.task_id = tr.task_id
WHERE COALESCE(NULLIF(TRIM(tr.user_id), ''), a.user_id) IS NOT NULL;`, clientSessionID, taskInAttemptedAt, taskInIdempotent)

	if !database.Migrator().HasTable("task_records") {
		copySQL = ""
	}

	return rebuildSQLiteTable(database, sqliteRebuildSpec{
		Table:     "task_records",
		TempTable: "new_task_records",
		CreateSQL: `
CREATE TABLE new_task_records (
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
	client_session_id TEXT,
	task_in_attempted_at DATETIME,
	task_in_idempotent INTEGER NOT NULL DEFAULT 0,
	sync_status TEXT NOT NULL DEFAULT 'pending',
	attempt_count INTEGER NOT NULL DEFAULT 0,
	last_error TEXT NOT NULL DEFAULT '',
	last_attempt_at DATETIME,
	gap INTEGER NOT NULL DEFAULT 0,
	FOREIGN KEY(user_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
	FOREIGN KEY(attendance_id) REFERENCES attendances(id) ON UPDATE CASCADE ON DELETE CASCADE,
	FOREIGN KEY(task_id) REFERENCES tasks(task_id) ON UPDATE CASCADE ON DELETE CASCADE
);`,
		CopySQL:        copySQL,
		ValidateLegacy: validateLegacyTaskRecordRows,
	})
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
		"client_session_id",
		"task_in_attempted_at",
		"task_in_idempotent",
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

	requiredNotNull := []string{"user_id", "attendance_id", "task_id", "started_at", "task_in_idempotent", "sync_status", "attempt_count", "last_error", "gap"}
	for _, name := range requiredNotNull {
		if columnMap[name].NotNull == 0 {
			return true, nil
		}
	}

	expectedDefaults := map[string]string{
		"padagan":            "0",
		"notes":              "",
		"duration":           "0",
		"task_in_idempotent": "0",
		"sync_status":        "pending",
		"attempt_count":      "0",
		"last_error":         "",
		"gap":                "0",
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

func validateLegacyTaskRecordRows(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("task_records") {
		return nil
	}

	var unresolvedUserCount int64
	if err := tx.Table("task_records AS tr").
		Joins("LEFT JOIN attendances AS a ON a.id = tr.attendance_id").
		Where("COALESCE(NULLIF(TRIM(tr.user_id), ''), a.user_id) IS NULL").
		Count(&unresolvedUserCount).Error; err != nil {
		return err
	}
	if unresolvedUserCount > 0 {
		return fmt.Errorf("task_records repair aborted: found %d rows without a resolved user_id", unresolvedUserCount)
	}

	var missingTaskCount int64
	if err := tx.Table("task_records AS tr").
		Joins("LEFT JOIN tasks AS t ON t.task_id = tr.task_id").
		Where("t.task_id IS NULL").
		Count(&missingTaskCount).Error; err != nil {
		return err
	}
	if missingTaskCount > 0 {
		return fmt.Errorf("task_records repair aborted: found %d rows with missing task references", missingTaskCount)
	}

	return nil
}

func tableHasRows(database *gorm.DB, table string) (bool, error) {
	var count int64
	if err := database.Table(table).Limit(1).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
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
