package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"fesapp/backend/db/models"

	sqlite "github.com/Tryanks/gorm-sqlite"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

var DB *gorm.DB
var DBPath string

type User = models.User
type Task = models.Task
type TaskRecord = models.TaskRecord
type Screenshot = models.Screenshot

// Init initializes the application database.
func Init(ctx context.Context) (*gorm.DB, error) {
	dbPath, err := resolveDatabasePath(ctx)
	if err != nil {
		return nil, err
	}

	database, err := openSQLiteDatabase(dbPath)
	if err != nil {
		return nil, err
	}

	if err := applySQLiteConnectionSettings(database); err != nil {
		return nil, err
	}

	if err := runMigrations(database, dbPath); err != nil {
		return nil, err
	}

	DB = database
	DBPath = dbPath

	fmt.Println("Database initialized at", dbPath)
	return DB, nil
}

func resolveDatabasePath(ctx context.Context) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user config directory: %w", err)
	}

	appDir := filepath.Join(dir, "wails-app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create app directory: %w", err)
	}

	env := runtime.Environment(ctx)
	if env.BuildType == "dev" {
		return filepath.Join(appDir, "fesapp-dev.db"), nil
	}

	return filepath.Join(appDir, "fesapp.db"), nil
}

func openSQLiteDatabase(dbPath string) (*gorm.DB, error) {
	database, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	if err := configureSQLitePool(database, dbPath); err != nil {
		return nil, err
	}

	return database, nil
}

func configureSQLitePool(database *gorm.DB, dbPath string) error {
	if database == nil {
		return fmt.Errorf("nil database")
	}

	sqlDB, err := database.DB()
	if err != nil {
		return fmt.Errorf("failed to resolve sqlite connection pool: %w", err)
	}

	limitSQLiteConnections(sqlDB, dbPath)
	return nil
}

func limitSQLiteConnections(sqlDB *sql.DB, dbPath string) {
	if sqlDB == nil {
		return
	}

	if dbPath == ":memory:" {
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
	}
}

func applySQLiteConnectionSettings(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("nil database")
	}

	statements := []struct {
		sql string
		msg string
	}{
		{sql: "PRAGMA foreign_keys = ON", msg: "failed to enable foreign keys"},
		{sql: "PRAGMA journal_mode = WAL", msg: "failed to enable WAL mode"},
		{sql: "PRAGMA busy_timeout = 5000", msg: "failed to set busy timeout"},
	}

	for _, statement := range statements {
		if err := database.Exec(statement.sql).Error; err != nil {
			return fmt.Errorf("%s: %w", statement.msg, err)
		}
	}

	return nil
}

// NewTestDB creates an in-memory SQLite database with the full migration stack applied.
func NewTestDB() (*gorm.DB, error) {
	database, err := openSQLiteDatabase(":memory:")
	if err != nil {
		return nil, err
	}

	if err := applySQLiteConnectionSettings(database); err != nil {
		return nil, err
	}

	if err := runMigrations(database, ""); err != nil {
		return nil, err
	}

	return database, nil
}
