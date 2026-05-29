package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fesapp/backend/db/models"

	sqlite "github.com/Tryanks/gorm-sqlite" // modernc.org/sqlite-based GORM driver
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

var DB *gorm.DB

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

	// Detect environment via Wails
	env := runtime.Environment(ctx)

	var dbPath string
	if env.BuildType == "dev" {
		dbPath = filepath.Join(appDir, "fesapp-dev.db")
	} else {
		dbPath = filepath.Join(appDir, "fesapp.db")
	}

	// Open database
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	// Run migrations
	if err := DB.AutoMigrate(
		&User{},
		&Task{},
		&TaskRecord{},
		&Screenshot{},
		&models.Attendance{},
	); err != nil {
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	if err := repairUsersSchema(DB); err != nil {
		return nil, fmt.Errorf("failed to repair users schema: %w", err)
	}

	fmt.Println("Database initialized at", dbPath)
	return DB, nil
}

type tableInfoRow struct {
	Name string `gorm:"column:name"`
	Type string `gorm:"column:type"`
}

func repairUsersSchema(database *gorm.DB) (err error) {
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

	tx := database.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			err = fmt.Errorf("panic while repairing users schema: %v", r)
		}
	}()

	if err := tx.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Exec("ALTER TABLE users RENAME TO users_legacy").Error; err != nil {
		_ = tx.Rollback()
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

	if err := tx.Exec(createUsersSQL).Error; err != nil {
		_ = tx.Rollback()
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

	if err := tx.Exec(insertSQL).Error; err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Exec("DROP TABLE users_legacy").Error; err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit().Error
}
