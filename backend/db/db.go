package db

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"fesapp/backend/db/models"

	sqlite "github.com/Tryanks/gorm-sqlite" // Or another modernc.org/sqlite-based GORM driver
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

var DB *gorm.DB

type User = models.User
type Task = models.Task
type TaskRecord = models.TaskRecord
type Screenshot = models.Screenshot

func Init() *gorm.DB {
	dir, _ := os.UserConfigDir()
	appDir := filepath.Join(dir, "wails-app")
	_ = os.MkdirAll(appDir, 0o755)
	dbPath := filepath.Join(appDir, "fesapp.db")

	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// Run migrations here
	if err := DB.AutoMigrate(&User{}); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}
	if err := DB.AutoMigrate(&Task{}); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}
	if err := DB.AutoMigrate(&TaskRecord{}); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}
	if err := DB.AutoMigrate(&Screenshot{}); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}

	fmt.Println("✅ Database initialized at", dbPath)
	return DB
}
