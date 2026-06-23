package db

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type foreignKeyRow struct {
	ID       int    `gorm:"column:id"`
	Seq      int    `gorm:"column:seq"`
	Table    string `gorm:"column:table"`
	From     string `gorm:"column:from"`
	To       string `gorm:"column:to"`
	OnUpdate string `gorm:"column:on_update"`
	OnDelete string `gorm:"column:on_delete"`
	Match    string `gorm:"column:match"`
}

type tableColumnRow struct {
	Name string `gorm:"column:name"`
}

func repairTaskAndScreenshotSchemas(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("nil database")
	}

	return withSQLiteForeignKeysDisabled(database, func(tx *gorm.DB) error {
		if err := validateLegacyTaskRows(tx); err != nil {
			return err
		}

		if err := repairTasksTable(tx); err != nil {
			return err
		}

		// Rebuild child tables after tasks so renamed-parent references are restored.
		if tx.Migrator().HasTable("tasks") && tx.Migrator().HasTable("task_records") {
			if err := repairTaskRecordsSchemaTx(tx); err != nil {
				return err
			}
		}

		if err := validateLegacyScreenshotRows(tx); err != nil {
			return err
		}

		if err := repairScreenshotsTable(tx); err != nil {
			return err
		}

		if err := ensureRebuiltTableIndexes(tx); err != nil {
			return err
		}

		return nil
	})
}

func taskAndScreenshotSchemasNeedRepair(database *gorm.DB) (bool, error) {
	if database == nil {
		return false, nil
	}

	if database.Migrator().HasTable("tasks") {
		fks, err := foreignKeysForTable(database, "tasks")
		if err != nil {
			return false, err
		}
		if !tasksSchemaIsHealthy(fks) {
			return true, nil
		}
	}

	if database.Migrator().HasTable("screenshots") {
		fks, err := foreignKeysForTable(database, "screenshots")
		if err != nil {
			return false, err
		}
		if !screenshotsSchemaIsHealthy(fks) {
			return true, nil
		}
	}

	return false, nil
}

func repairTasksTable(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("tasks") {
		return nil
	}

	fks, err := foreignKeysForTable(tx, "tasks")
	if err != nil {
		return err
	}

	if tasksSchemaIsHealthy(fks) {
		return nil
	}

	if err := tx.Exec("DROP TABLE IF EXISTS tasks_legacy").Error; err != nil {
		return err
	}
	if err := tx.Exec("ALTER TABLE tasks RENAME TO tasks_legacy").Error; err != nil {
		return err
	}

	createSQL := `
CREATE TABLE tasks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	task_id CHAR(36) UNIQUE,
	title TEXT,
	project_name TEXT,
	client_id TEXT,
	CONSTRAINT fk_clients_tasks FOREIGN KEY (client_id) REFERENCES clients(id)
);`
	if err := tx.Exec(createSQL).Error; err != nil {
		return err
	}

	copySQL := `
INSERT INTO tasks (id, task_id, title, project_name, client_id)
SELECT
	id,
	NULLIF(TRIM(CAST(task_id AS TEXT)), ''),
	title,
	project_name,
	client_id
FROM tasks_legacy;`
	if err := tx.Exec(copySQL).Error; err != nil {
		return err
	}

	if err := tx.Exec("DROP TABLE tasks_legacy").Error; err != nil {
		return err
	}

	return nil
}

func repairScreenshotsTable(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("screenshots") {
		return nil
	}

	fks, err := foreignKeysForTable(tx, "screenshots")
	if err != nil {
		return err
	}

	if screenshotsSchemaIsHealthy(fks) {
		return nil
	}

	if err := tx.Exec("DROP TABLE IF EXISTS screenshots_legacy").Error; err != nil {
		return err
	}
	if err := tx.Exec("ALTER TABLE screenshots RENAME TO screenshots_legacy").Error; err != nil {
		return err
	}

	createSQL := `
CREATE TABLE screenshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	task_record_id INTEGER NOT NULL,
	task_id TEXT NOT NULL,
	memo TEXT NOT NULL DEFAULT '',
	notes TEXT NOT NULL DEFAULT '',
	file_path TEXT,
	cloud_url TEXT,
	taken_at DATETIME NOT NULL,
	CONSTRAINT fk_task_records_screenshots FOREIGN KEY (task_record_id) REFERENCES task_records(id)
);`
	if err := tx.Exec(createSQL).Error; err != nil {
		return err
	}

	columnMap, err := tableColumnMap(tx, "screenshots_legacy")
	if err != nil {
		return err
	}

	copySQL := `
INSERT INTO screenshots (id, task_record_id, task_id, memo, notes, file_path, cloud_url, taken_at)
SELECT
	id,
	task_record_id,
	CAST(task_id AS TEXT),
	` + columnExpr(columnMap, "memo", "''") + `,
	` + columnExpr(columnMap, "notes", "''") + `,
	` + columnExpr(columnMap, "file_path", "NULL") + `,
	` + columnExpr(columnMap, "cloud_url", "NULL") + `,
	COALESCE(` + columnExpr(columnMap, "taken_at", "CURRENT_TIMESTAMP") + `, CURRENT_TIMESTAMP)
FROM screenshots_legacy;`
	if err := tx.Exec(copySQL).Error; err != nil {
		return err
	}

	if err := tx.Exec("DROP TABLE screenshots_legacy").Error; err != nil {
		return err
	}

	return nil
}

func foreignKeysForTable(tx *gorm.DB, table string) ([]foreignKeyRow, error) {
	var rows []foreignKeyRow
	if err := tx.Raw(fmt.Sprintf("PRAGMA foreign_key_list(%s)", quoteSQLiteIdentifier(table))).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func tasksSchemaIsHealthy(fks []foreignKeyRow) bool {
	if len(fks) != 1 {
		return false
	}

	fk := fks[0]
	return strings.EqualFold(fk.Table, "clients") &&
		strings.EqualFold(fk.From, "client_id") &&
		strings.EqualFold(fk.To, "id")
}

func screenshotsSchemaIsHealthy(fks []foreignKeyRow) bool {
	if len(fks) != 1 {
		return false
	}

	fk := fks[0]
	return strings.EqualFold(fk.Table, "task_records") &&
		strings.EqualFold(fk.From, "task_record_id") &&
		strings.EqualFold(fk.To, "id")
}

func quoteSQLiteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func ensureRebuiltTableIndexes(tx *gorm.DB) error {
	statements := []string{
		"CREATE INDEX IF NOT EXISTS idx_screenshots_task_record_taken ON screenshots(task_record_id, taken_at)",
		"CREATE INDEX IF NOT EXISTS idx_screenshots_task_taken ON screenshots(task_id, taken_at)",
	}

	for _, stmt := range statements {
		if err := tx.Exec(stmt).Error; err != nil {
			return err
		}
	}

	return nil
}

func validateLegacyTaskRows(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("tasks") {
		return nil
	}

	var invalidCount int64
	if err := tx.Table("tasks").
		Where("task_id IS NOT NULL AND TRIM(CAST(task_id AS TEXT)) = ''").
		Count(&invalidCount).Error; err != nil {
		return err
	}
	if invalidCount > 0 {
		return fmt.Errorf("tasks repair aborted: found %d rows with blank task_id", invalidCount)
	}

	return nil
}

func validateLegacyScreenshotRows(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("screenshots") {
		return nil
	}

	var invalidCount int64
	if err := tx.Table("screenshots").Where("task_record_id IS NULL").Count(&invalidCount).Error; err != nil {
		return err
	}
	if invalidCount > 0 {
		return fmt.Errorf("screenshots repair aborted: found %d rows with NULL task_record_id", invalidCount)
	}

	return nil
}

func tableColumnMap(tx *gorm.DB, table string) (map[string]struct{}, error) {
	var rows []tableColumnRow
	if err := tx.Raw(fmt.Sprintf("PRAGMA table_info(%s)", quoteSQLiteIdentifier(table))).Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		out[strings.ToLower(row.Name)] = struct{}{}
	}
	return out, nil
}

func columnExpr(columns map[string]struct{}, name, fallback string) string {
	if _, ok := columns[strings.ToLower(name)]; ok {
		return name
	}
	return fallback
}
