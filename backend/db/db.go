package db

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"fesapp/backend/db/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

// Example model
type Note struct {
	ID    uint `gorm:"primaryKey"`
	Title string
	Body  string
}
type User = models.User

func Init() {
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
	if err := DB.AutoMigrate(&Note{}); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}

	fmt.Println("✅ Database initialized at", dbPath)
}
