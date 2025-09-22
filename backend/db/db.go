package db

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"fesapp/backend/db/models"

	"github.com/Tryanks/gorm-sqlite" // Or another modernc.org/sqlite-based GORM driver
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

var DB *gorm.DB

// Example model
type Note struct {
	ID    uint `gorm:"primaryKey"`
	Title string
	Body  string
}
type User = models.User

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

	fmt.Println("✅ Database initialized at", dbPath)
	return DB
}
