package db

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type tableInfoRow struct {
	Name string `gorm:"column:name"`
	Type string `gorm:"column:type"`
}

func planUsersMigration(database *gorm.DB) (schemaPlan, error) {
	needsRepair, err := usersSchemaNeedsRepair(database)
	if err != nil {
		return schemaPlan{}, err
	}
	if !needsRepair {
		return schemaPlan{Action: migrationActionNoop}, nil
	}
	if !database.Migrator().HasTable("users") {
		return schemaPlan{Action: migrationActionCreate}, nil
	}

	hasRows, err := tableHasRows(database, "users")
	if err != nil {
		return schemaPlan{}, err
	}

	return schemaPlan{
		Action:         migrationActionRebuild,
		RequiresBackup: hasRows,
	}, nil
}

func ensureUsersSchema(database *gorm.DB) error {
	needsRepair, err := usersSchemaNeedsRepair(database)
	if err != nil {
		return err
	}
	if !needsRepair {
		return nil
	}

	columnMap, err := userSchemaColumnMap(database)
	if err != nil {
		return err
	}

	selectExpr := func(col string) string {
		if _, ok := columnMap[col]; ok {
			return "CAST(" + col + " AS TEXT)"
		}
		return "NULL"
	}

	birthExpr := "NULL"
	if _, ok := columnMap["birthdate"]; ok {
		birthExpr = "CAST(birthdate AS TEXT)"
	} else if _, ok := columnMap["birthday"]; ok {
		birthExpr = "CAST(birthday AS TEXT)"
	}

	copySQL := `
INSERT INTO new_users (id, email, name, first_name, last_name, middle_name, position, birthdate, phone, address, photo, fes_id, supervisor_id, employee_id, employment_id)
SELECT
	CAST(id AS TEXT),
	` + selectExpr("email") + `,
	` + selectExpr("name") + `,
	` + selectExpr("first_name") + `,
	` + selectExpr("last_name") + `,
	` + selectExpr("middle_name") + `,
	` + selectExpr("position") + `,
	` + birthExpr + `,
	` + selectExpr("phone") + `,
	` + selectExpr("address") + `,
	` + selectExpr("photo") + `,
	` + selectExpr("fes_id") + `,
	` + selectExpr("supervisor_id") + `,
	` + selectExpr("employee_id") + `,
	` + selectExpr("employment_id") + `
FROM users;`

	if !database.Migrator().HasTable("users") {
		copySQL = ""
	}

	return rebuildSQLiteTable(database, sqliteRebuildSpec{
		Table:     "users",
		TempTable: "new_users",
		CreateSQL: `
CREATE TABLE new_users (
	id TEXT PRIMARY KEY,
	email TEXT,
	name TEXT,
	first_name TEXT,
	last_name TEXT,
	middle_name TEXT,
	position TEXT,
	birthdate TEXT,
	phone TEXT,
	address TEXT,
	photo TEXT,
	fes_id TEXT,
	supervisor_id TEXT,
	employee_id TEXT,
	employment_id TEXT
)`,
		CopySQL:        copySQL,
		ValidateLegacy: validateLegacyUsersRows,
	})
}

func usersSchemaNeedsRepair(database *gorm.DB) (bool, error) {
	if database == nil {
		return false, nil
	}
	if !database.Migrator().HasTable("users") {
		return true, nil
	}

	columnMap, err := userSchemaColumnMap(database)
	if err != nil {
		return false, err
	}

	requiredColumns := []string{
		"id",
		"email",
		"name",
		"first_name",
		"last_name",
		"middle_name",
		"position",
		"birthdate",
		"phone",
		"address",
		"photo",
		"fes_id",
		"supervisor_id",
		"employee_id",
		"employment_id",
	}
	for _, name := range requiredColumns {
		if _, ok := columnMap[name]; !ok {
			if name == "birthdate" {
				if _, hasBirthday := columnMap["birthday"]; hasBirthday {
					return true, nil
				}
			}
			return true, nil
		}
	}

	idType := columnMap["id"]
	return strings.Contains(idType, "INT") || strings.Contains(idType, "REAL") || strings.Contains(idType, "NUM"), nil
}

func userSchemaColumnMap(database *gorm.DB) (map[string]string, error) {
	if database == nil || !database.Migrator().HasTable("users") {
		return map[string]string{}, nil
	}

	var columns []tableInfoRow
	if err := database.Raw("PRAGMA table_info(users)").Scan(&columns).Error; err != nil {
		return nil, err
	}

	return buildUserSchemaColumnMap(columns), nil
}

func buildUserSchemaColumnMap(columns []tableInfoRow) map[string]string {
	columnMap := make(map[string]string, len(columns))
	for _, col := range columns {
		columnMap[strings.ToLower(col.Name)] = strings.ToUpper(strings.TrimSpace(col.Type))
	}
	return columnMap
}

func validateLegacyUsersRows(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("users") {
		return nil
	}

	var invalidCount int64
	if err := tx.Table("users").
		Where("id IS NULL OR TRIM(CAST(id AS TEXT)) = ''").
		Count(&invalidCount).Error; err != nil {
		return err
	}
	if invalidCount > 0 {
		return fmt.Errorf("users repair aborted: found %d rows with blank id", invalidCount)
	}

	return nil
}

func planClientsMigration(database *gorm.DB) (schemaPlan, error) {
	needsRepair, err := clientsSchemaNeedsRepair(database)
	if err != nil {
		return schemaPlan{}, err
	}
	if !needsRepair {
		return schemaPlan{Action: migrationActionNoop}, nil
	}
	if !database.Migrator().HasTable("clients") {
		return schemaPlan{Action: migrationActionCreate}, nil
	}

	hasRows, err := tableHasRows(database, "clients")
	if err != nil {
		return schemaPlan{}, err
	}

	return schemaPlan{
		Action:         migrationActionRebuild,
		RequiresBackup: hasRows,
	}, nil
}

func ensureClientsSchema(database *gorm.DB) error {
	needsRepair, err := clientsSchemaNeedsRepair(database)
	if err != nil {
		return err
	}
	if !needsRepair {
		return nil
	}

	columnMap := map[string]struct{}{}
	if database.Migrator().HasTable("clients") {
		columnMap, err = tableColumnMap(database, "clients")
		if err != nil {
			return err
		}
	}

	copySQL := `
INSERT INTO new_clients (id, fullname, company_name)
SELECT
	CAST(id AS TEXT),
	` + columnExpr(columnMap, "fullname", "NULL") + `,
	` + columnExpr(columnMap, "company_name", "NULL") + `
FROM clients;`

	if !database.Migrator().HasTable("clients") {
		copySQL = ""
	}

	return rebuildSQLiteTable(database, sqliteRebuildSpec{
		Table:     "clients",
		TempTable: "new_clients",
		CreateSQL: `
CREATE TABLE new_clients (
	id TEXT PRIMARY KEY,
	fullname TEXT,
	company_name TEXT
)`,
		CopySQL:        copySQL,
		ValidateLegacy: validateLegacyClientRows,
	})
}

func clientsSchemaNeedsRepair(database *gorm.DB) (bool, error) {
	if database == nil {
		return false, nil
	}
	if !database.Migrator().HasTable("clients") {
		return true, nil
	}

	columnMap, err := tableColumnMap(database, "clients")
	if err != nil {
		return false, err
	}
	for _, name := range []string{"id", "fullname", "company_name"} {
		if _, ok := columnMap[name]; !ok {
			return true, nil
		}
	}

	return false, nil
}

func validateLegacyClientRows(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("clients") {
		return nil
	}

	var invalidCount int64
	if err := tx.Table("clients").
		Where("id IS NULL OR TRIM(CAST(id AS TEXT)) = ''").
		Count(&invalidCount).Error; err != nil {
		return err
	}
	if invalidCount > 0 {
		return fmt.Errorf("clients repair aborted: found %d rows with blank id", invalidCount)
	}

	return nil
}

func planAttendancesMigration(database *gorm.DB) (schemaPlan, error) {
	needsRepair, err := attendancesSchemaNeedsRepair(database)
	if err != nil {
		return schemaPlan{}, err
	}
	if !needsRepair {
		return schemaPlan{Action: migrationActionNoop}, nil
	}
	if !database.Migrator().HasTable("attendances") {
		return schemaPlan{Action: migrationActionCreate}, nil
	}

	hasRows, err := tableHasRows(database, "attendances")
	if err != nil {
		return schemaPlan{}, err
	}

	return schemaPlan{
		Action:         migrationActionRebuild,
		RequiresBackup: hasRows,
	}, nil
}

func ensureAttendancesSchema(database *gorm.DB) error {
	needsRepair, err := attendancesSchemaNeedsRepair(database)
	if err != nil {
		return err
	}
	if !needsRepair {
		return nil
	}

	columnMap := map[string]struct{}{}
	if database.Migrator().HasTable("attendances") {
		columnMap, err = tableColumnMap(database, "attendances")
		if err != nil {
			return err
		}
	}

	copySQL := `
INSERT INTO new_attendances (
	id,
	created_at,
	updated_at,
	deleted_at,
	user_id,
	client_time,
	total_gap,
	time_in,
	time_out,
	login_logout_id,
	synced_at,
	sync_status,
	attempt_count,
	last_error,
	last_attempt_at
)
SELECT
	id,
	COALESCE(` + columnExpr(columnMap, "created_at", "NULL") + `, ` + columnExpr(columnMap, "time_in", "CURRENT_TIMESTAMP") + `, CURRENT_TIMESTAMP),
	COALESCE(` + columnExpr(columnMap, "updated_at", "NULL") + `, ` + columnExpr(columnMap, "created_at", "NULL") + `, ` + columnExpr(columnMap, "time_in", "CURRENT_TIMESTAMP") + `, CURRENT_TIMESTAMP),
	` + columnExpr(columnMap, "deleted_at", "NULL") + `,
	CAST(user_id AS TEXT),
	COALESCE(` + columnExpr(columnMap, "client_time", "0") + `, 0),
	COALESCE(` + columnExpr(columnMap, "total_gap", "0") + `, 0),
	` + columnExpr(columnMap, "time_in", "NULL") + `,
	` + columnExpr(columnMap, "time_out", "NULL") + `,
	` + columnExpr(columnMap, "login_logout_id", "NULL") + `,
	` + columnExpr(columnMap, "synced_at", "NULL") + `,
	COALESCE(NULLIF(TRIM(` + columnExpr(columnMap, "sync_status", "''") + `), ''), 'pending'),
	COALESCE(` + columnExpr(columnMap, "attempt_count", "0") + `, 0),
	COALESCE(` + columnExpr(columnMap, "last_error", "''") + `, ''),
	` + columnExpr(columnMap, "last_attempt_at", "NULL") + `
FROM attendances;`

	if !database.Migrator().HasTable("attendances") {
		copySQL = ""
	}

	return rebuildSQLiteTable(database, sqliteRebuildSpec{
		Table:     "attendances",
		TempTable: "new_attendances",
		CreateSQL: `
CREATE TABLE new_attendances (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at DATETIME,
	updated_at DATETIME,
	deleted_at DATETIME,
	user_id TEXT NOT NULL,
	client_time INTEGER NOT NULL DEFAULT 0,
	total_gap INTEGER NOT NULL DEFAULT 0,
	time_in DATETIME,
	time_out DATETIME,
	login_logout_id TEXT,
	synced_at DATETIME,
	sync_status TEXT NOT NULL DEFAULT 'pending',
	attempt_count INTEGER NOT NULL DEFAULT 0,
	last_error TEXT NOT NULL DEFAULT '',
	last_attempt_at DATETIME,
	FOREIGN KEY(user_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE
)`,
		CopySQL:        copySQL,
		ValidateLegacy: validateLegacyAttendanceRows,
	})
}

func attendancesSchemaNeedsRepair(database *gorm.DB) (bool, error) {
	if database == nil {
		return false, nil
	}
	if !database.Migrator().HasTable("attendances") {
		return true, nil
	}

	var columns []tableInfoDetailRow
	if err := database.Raw(`PRAGMA table_info("attendances")`).Scan(&columns).Error; err != nil {
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
		"created_at",
		"updated_at",
		"deleted_at",
		"user_id",
		"client_time",
		"total_gap",
		"time_in",
		"time_out",
		"login_logout_id",
		"synced_at",
		"sync_status",
		"attempt_count",
		"last_error",
		"last_attempt_at",
	}
	for _, name := range requiredColumns {
		if _, ok := columnMap[name]; !ok {
			return true, nil
		}
	}

	requiredNotNull := []string{"user_id", "client_time", "total_gap", "sync_status", "attempt_count", "last_error"}
	for _, name := range requiredNotNull {
		if columnMap[name].NotNull == 0 {
			return true, nil
		}
	}

	expectedDefaults := map[string]string{
		"client_time":   "0",
		"total_gap":     "0",
		"sync_status":   "pending",
		"attempt_count": "0",
		"last_error":    "",
	}
	for name, expected := range expectedDefaults {
		if normalizeSQLiteDefaultValue(columnMap[name].DefaultValue) != expected {
			return true, nil
		}
	}

	fks, err := foreignKeysForTable(database, "attendances")
	if err != nil {
		return false, err
	}
	if len(fks) != 1 {
		return true, nil
	}

	fk := fks[0]
	if !strings.EqualFold(fk.Table, "users") ||
		!strings.EqualFold(fk.From, "user_id") ||
		!strings.EqualFold(fk.To, "id") ||
		!strings.EqualFold(fk.OnUpdate, "CASCADE") ||
		!strings.EqualFold(fk.OnDelete, "CASCADE") {
		return true, nil
	}

	return false, nil
}

func validateLegacyAttendanceRows(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("attendances") {
		return nil
	}

	var invalidCount int64
	if err := tx.Table("attendances").
		Where("user_id IS NULL OR TRIM(CAST(user_id AS TEXT)) = ''").
		Count(&invalidCount).Error; err != nil {
		return err
	}
	if invalidCount > 0 {
		return fmt.Errorf("attendances repair aborted: found %d rows with blank user_id", invalidCount)
	}

	var missingUserCount int64
	if err := tx.Table("attendances AS a").
		Joins("LEFT JOIN users AS u ON u.id = a.user_id").
		Where("u.id IS NULL").
		Count(&missingUserCount).Error; err != nil {
		return err
	}
	if missingUserCount > 0 {
		return fmt.Errorf("attendances repair aborted: found %d rows with missing user references", missingUserCount)
	}

	return nil
}

func quoteSQLiteString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
