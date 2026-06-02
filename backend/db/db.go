package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fesapp/backend/db/models"
	"fesapp/backend/utils"

	sqlite "github.com/Tryanks/gorm-sqlite" // modernc.org/sqlite-based GORM driver
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

var DB *gorm.DB
var DBPath string

type User = models.User
type Task = models.Task
type TaskRecord = models.TaskRecord
type Screenshot = models.Screenshot

// Init initializes the database
func Init(ctx context.Context) (*gorm.DB, error) {
	// Use user config directory
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve user config directory: %w", err)
	}
	appDir := filepath.Join(dir, "wails-app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create app directory: %w", err)
	}
	utils.Logger.Printf("db init: appDir=%s", appDir)

	// Detect environment via Wails
	env := runtime.Environment(ctx)

	var dbPath string
	if env.BuildType == "dev" {
		dbPath = filepath.Join(appDir, "fesapp-dev.db")
	} else {
		dbPath = filepath.Join(appDir, "fesapp.db")
	}
	utils.Logger.Printf("db init: buildType=%s dbPath=%s", env.BuildType, dbPath)

	// Open database
	utils.Logger.Printf("db init: opening database")
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}
	DBPath = dbPath

	if err := DB.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	backupPath, err := createMigrationBackup(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create pre-migration backup: %w", err)
	}
	if backupPath != "" {
		utils.Logger.Printf("db init: migration backup created path=%s", backupPath)
	}

	if err := repairLegacyTaskRecordUserIDs(DB); err != nil {
		return nil, fmt.Errorf("failed to repair legacy task records: %w", err)
	}

	// Run migrations
	utils.Logger.Printf("db init: running AutoMigrate")
	if err := DB.AutoMigrate(
		&User{},
		&Task{},
		&TaskRecord{},
		&Screenshot{},
		&models.Attendance{},
	); err != nil {
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	utils.Logger.Printf("db init: running schema migrations")
	if err := migrateSchema(DB); err != nil {
		if backupPath != "" {
			return nil, fmt.Errorf("failed to apply schema migrations: %w (rollback backup: %s)", err, backupPath)
		}
		return nil, fmt.Errorf("failed to apply schema migrations: %w", err)
	}

	utils.Logger.Printf("db init: completed dbPath=%s", dbPath)
	fmt.Println("Database initialized at", dbPath)
	return DB, nil
}

func repairLegacyTaskRecordUserIDs(database *gorm.DB) error {
	if database == nil {
		return nil
	}

	if !database.Migrator().HasTable("task_records") {
		return nil
	}

	var missingBefore int64
	if err := database.Model(&models.TaskRecord{}).
		Where("user_id IS NULL OR TRIM(user_id) = ''").
		Count(&missingBefore).Error; err != nil {
		return fmt.Errorf("count legacy task_records with missing user_id: %w", err)
	}

	if missingBefore == 0 {
		return nil
	}

	utils.Logger.Printf("db init: repairing legacy task_records.user_id rows count=%d", missingBefore)

	if err := database.Exec(`
UPDATE task_records
SET user_id = (
	SELECT user_id
	FROM attendances
	WHERE attendances.id = task_records.attendance_id
)
WHERE user_id IS NULL OR TRIM(user_id) = ''
`).Error; err != nil {
		return fmt.Errorf("backfill task_records.user_id from attendances: %w", err)
	}

	var missingAfter int64
	if err := database.Model(&models.TaskRecord{}).
		Where("user_id IS NULL OR TRIM(user_id) = ''").
		Count(&missingAfter).Error; err != nil {
		return fmt.Errorf("recount legacy task_records with missing user_id: %w", err)
	}

	if missingAfter > 0 {
		return fmt.Errorf("task_records still contains %d rows with missing user_id after backfill", missingAfter)
	}

	return nil
}

type tableInfoRow struct {
	Name string `gorm:"column:name"`
	Type string `gorm:"column:type"`
}

func repairUsersSchema(database *gorm.DB) error {
	if database == nil {
		return nil
	}

	if !database.Migrator().HasTable("users") {
		return nil
	}

	var columns []tableInfoRow
	if err := database.Raw("PRAGMA table_info(users)").Scan(&columns).Error; err != nil {
		return err
	}

	if len(columns) == 0 {
		return nil
	}

	columnMap := make(map[string]string, len(columns))
	for _, col := range columns {
		columnMap[strings.ToLower(col.Name)] = strings.ToUpper(strings.TrimSpace(col.Type))
	}

	idType, hasID := columnMap["id"]
	_, hasBirthdate := columnMap["birthdate"]
	_, hasBirthday := columnMap["birthday"]

	needsRebuild := !hasID || strings.Contains(idType, "INT") || strings.Contains(idType, "REAL") || strings.Contains(idType, "NUM")
	needsBirthdateRepair := !hasBirthdate && hasBirthday

	if !needsRebuild && !needsBirthdateRepair {
		return nil
	}

	if err := database.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
		return err
	}
	defer database.Exec("PRAGMA foreign_keys = ON")

	if err := database.Exec("ALTER TABLE users RENAME TO users_legacy").Error; err != nil {
		return err
	}

	createUsersSQL := `
CREATE TABLE users (
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
)`

	if err := database.Exec(createUsersSQL).Error; err != nil {
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

	insertSQL := `
INSERT INTO users (id, email, name, first_name, last_name, middle_name, position, birthdate, phone, address, photo, fes_id, supervisor_id, employee_id, employment_id)
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
FROM users_legacy`

	if err := database.Exec(insertSQL).Error; err != nil {
		return err
	}

	if err := database.Exec("DROP TABLE users_legacy").Error; err != nil {
		return err
	}
	return nil
}
