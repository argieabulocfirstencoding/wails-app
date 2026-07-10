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

type sqliteIndexListRow struct {
	Name   string `gorm:"column:name"`
	Unique int    `gorm:"column:unique"`
}

type sqliteIndexInfoRow struct {
	Name string `gorm:"column:name"`
}

func planTasksMigration(database *gorm.DB) (schemaPlan, error) {
	needsRepair, err := tasksSchemaNeedsRepair(database)
	if err != nil {
		return schemaPlan{}, err
	}
	if !needsRepair {
		return schemaPlan{Action: migrationActionNoop}, nil
	}
	if !database.Migrator().HasTable("tasks") {
		return schemaPlan{Action: migrationActionCreate}, nil
	}

	hasRows, err := tableHasRows(database, "tasks")
	if err != nil {
		return schemaPlan{}, err
	}

	return schemaPlan{
		Action:         migrationActionRebuild,
		RequiresBackup: hasRows,
	}, nil
}

func ensureTasksSchema(database *gorm.DB) error {
	needsRepair, err := tasksSchemaNeedsRepair(database)
	if err != nil {
		return err
	}
	if !needsRepair {
		return nil
	}

	copySQL := `
INSERT INTO new_tasks (id, task_id, title, project_name, client_id)
SELECT
	id,
	NULLIF(TRIM(CAST(task_id AS TEXT)), ''),
	title,
	project_name,
	CASE
		WHEN client_id IS NULL THEN NULL
		WHEN TRIM(CAST(client_id AS TEXT)) = '' THEN NULL
		WHEN EXISTS(SELECT 1 FROM clients AS c WHERE c.id = CAST(legacy_tasks.client_id AS TEXT)) THEN CAST(client_id AS TEXT)
		ELSE NULL
	END
FROM tasks AS legacy_tasks;`

	if !database.Migrator().HasTable("tasks") {
		copySQL = ""
	}

	return rebuildSQLiteTable(database, sqliteRebuildSpec{
		Table:     "tasks",
		TempTable: "new_tasks",
		CreateSQL: `
CREATE TABLE new_tasks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	task_id CHAR(36) UNIQUE,
	title TEXT,
	project_name TEXT,
	client_id TEXT,
	CONSTRAINT fk_clients_tasks FOREIGN KEY (client_id) REFERENCES clients(id) ON UPDATE CASCADE ON DELETE SET NULL
);`,
		CopySQL:        copySQL,
		ValidateLegacy: validateLegacyTaskRows,
	})
}

func planScreenshotsMigration(database *gorm.DB) (schemaPlan, error) {
	needsRepair, err := screenshotsSchemaNeedsRepair(database)
	if err != nil {
		return schemaPlan{}, err
	}
	if !needsRepair {
		return schemaPlan{Action: migrationActionNoop}, nil
	}
	if !database.Migrator().HasTable("screenshots") {
		return schemaPlan{Action: migrationActionCreate}, nil
	}

	hasRows, err := tableHasRows(database, "screenshots")
	if err != nil {
		return schemaPlan{}, err
	}

	return schemaPlan{
		Action:         migrationActionRebuild,
		RequiresBackup: hasRows,
	}, nil
}

func ensureScreenshotsSchema(database *gorm.DB) error {
	needsRepair, err := screenshotsSchemaNeedsRepair(database)
	if err != nil {
		return err
	}
	if !needsRepair {
		return nil
	}

	columnMap := map[string]struct{}{}
	if database.Migrator().HasTable("screenshots") {
		columnMap, err = tableColumnMap(database, "screenshots")
		if err != nil {
			return err
		}
	}

	copySQL := `
INSERT INTO new_screenshots (id, task_record_id, task_id, memo, notes, file_path, cloud_url, taken_at)
SELECT
	id,
	task_record_id,
	CAST(task_id AS TEXT),
	` + columnExpr(columnMap, "memo", "''") + `,
	` + columnExpr(columnMap, "notes", "''") + `,
	` + columnExpr(columnMap, "file_path", "NULL") + `,
	` + columnExpr(columnMap, "cloud_url", "NULL") + `,
	COALESCE(` + columnExpr(columnMap, "taken_at", "CURRENT_TIMESTAMP") + `, CURRENT_TIMESTAMP)
FROM screenshots;`

	if !database.Migrator().HasTable("screenshots") {
		copySQL = ""
	}

	return rebuildSQLiteTable(database, sqliteRebuildSpec{
		Table:     "screenshots",
		TempTable: "new_screenshots",
		CreateSQL: `
CREATE TABLE new_screenshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	task_record_id INTEGER NOT NULL,
	task_id TEXT NOT NULL,
	memo TEXT NOT NULL DEFAULT '',
	notes TEXT NOT NULL DEFAULT '',
	file_path TEXT,
	cloud_url TEXT,
	taken_at DATETIME NOT NULL,
	CONSTRAINT fk_task_records_screenshots FOREIGN KEY (task_record_id) REFERENCES task_records(id)
);`,
		CopySQL:        copySQL,
		ValidateLegacy: validateLegacyScreenshotRows,
	})
}

func foreignKeysForTable(tx *gorm.DB, table string) ([]foreignKeyRow, error) {
	var rows []foreignKeyRow
	if err := tx.Raw(fmt.Sprintf("PRAGMA foreign_key_list(%s)", quoteSQLiteIdentifier(table))).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func tasksSchemaNeedsRepair(database *gorm.DB) (bool, error) {
	if database == nil {
		return false, nil
	}
	if !database.Migrator().HasTable("tasks") {
		return true, nil
	}

	columns, err := tableColumnMap(database, "tasks")
	if err != nil {
		return false, err
	}
	for _, name := range []string{"id", "task_id", "title", "project_name", "client_id"} {
		if _, ok := columns[name]; !ok {
			return true, nil
		}
	}

	hasUniqueTaskID, err := hasUniqueIndexOnColumn(database, "tasks", "task_id")
	if err != nil {
		return false, err
	}
	if !hasUniqueTaskID {
		return true, nil
	}

	fks, err := foreignKeysForTable(database, "tasks")
	if err != nil {
		return false, err
	}
	return !tasksSchemaIsHealthy(fks), nil
}

func tasksSchemaIsHealthy(fks []foreignKeyRow) bool {
	if len(fks) != 1 {
		return false
	}

	fk := fks[0]
	return strings.EqualFold(fk.Table, "clients") &&
		strings.EqualFold(fk.From, "client_id") &&
		strings.EqualFold(fk.To, "id") &&
		strings.EqualFold(fk.OnUpdate, "CASCADE") &&
		strings.EqualFold(fk.OnDelete, "SET NULL")
}

func screenshotsSchemaNeedsRepair(database *gorm.DB) (bool, error) {
	if database == nil {
		return false, nil
	}
	if !database.Migrator().HasTable("screenshots") {
		return true, nil
	}

	columns, err := tableColumnMap(database, "screenshots")
	if err != nil {
		return false, err
	}
	for _, name := range []string{"id", "task_record_id", "task_id", "memo", "notes", "file_path", "cloud_url", "taken_at"} {
		if _, ok := columns[name]; !ok {
			return true, nil
		}
	}

	var details []tableInfoDetailRow
	if err := database.Raw(`PRAGMA table_info("screenshots")`).Scan(&details).Error; err != nil {
		return false, err
	}
	if len(details) == 0 {
		return true, nil
	}

	columnDetails := make(map[string]tableInfoDetailRow, len(details))
	for _, detail := range details {
		columnDetails[strings.ToLower(detail.Name)] = detail
	}

	requiredNotNull := []string{"task_record_id", "task_id", "memo", "notes", "taken_at"}
	for _, name := range requiredNotNull {
		if columnDetails[name].NotNull == 0 {
			return true, nil
		}
	}

	expectedDefaults := map[string]string{
		"memo":  "",
		"notes": "",
	}
	for name, expected := range expectedDefaults {
		if normalizeSQLiteDefaultValue(columnDetails[name].DefaultValue) != expected {
			return true, nil
		}
	}

	fks, err := foreignKeysForTable(database, "screenshots")
	if err != nil {
		return false, err
	}
	return !screenshotsSchemaIsHealthy(fks), nil
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

func hasUniqueIndexOnColumn(tx *gorm.DB, table, column string) (bool, error) {
	var indexes []sqliteIndexListRow
	if err := tx.Raw(fmt.Sprintf("PRAGMA index_list(%s)", quoteSQLiteIdentifier(table))).Scan(&indexes).Error; err != nil {
		return false, err
	}

	for _, index := range indexes {
		if index.Unique == 0 {
			continue
		}

		var columns []sqliteIndexInfoRow
		if err := tx.Raw(fmt.Sprintf("PRAGMA index_info(%s)", quoteSQLiteString(index.Name))).Scan(&columns).Error; err != nil {
			return false, err
		}
		if len(columns) != 1 {
			continue
		}
		if strings.EqualFold(columns[0].Name, column) {
			return true, nil
		}
	}

	return false, nil
}
