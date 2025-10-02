package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

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
func Init(ctx context.Context) *gorm.DB {
	// Use user config directory
	dir, _ := os.UserConfigDir()
	appDir := filepath.Join(dir, "wails-app")
	_ = os.MkdirAll(appDir, 0o755)

	// Detect environment via Wails
	env := runtime.Environment(ctx)

	var dbPath string
	if env.BuildType == "dev" {
		dbPath = filepath.Join(appDir, "fesapp-dev.db")
	} else {
		dbPath = filepath.Join(appDir, "fesapp.db")
	}

	// Open database
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// Run migrations
	if err := DB.AutoMigrate(
		&User{},
		&Task{},
		&TaskRecord{},
		&Screenshot{},
		&models.Attendance{},
	); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}

	fmt.Println("✅ Database initialized at", dbPath)
	return DB
}
