package db

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	
)

var DB *sql.DB

func InitDB(appName string) {
	// Store DB in user's config/data folder
	dir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}

	dbPath := filepath.Join(dir, appName, "app.db")

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatal(err)
	}

	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	if err := DB.Ping(); err != nil {
		log.Fatal("DB connection failed:", err)
	}

	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS notes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT,
			content TEXT
		);
		`)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("SQLite initialized at:", dbPath)
}
