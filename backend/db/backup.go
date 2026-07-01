package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"
)

func createMigrationBackup(database *gorm.DB, dbPath string) (string, error) {
	if database == nil || dbPath == "" {
		return "", nil
	}

	info, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	if info.Size() == 0 {
		return "", nil
	}

	backupPath := fmt.Sprintf("%s.pre_migration_%s.bak", dbPath, time.Now().UTC().Format("20060102_150405"))
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return "", err
	}
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if err := database.WithContext(context.Background()).Connection(func(conn *gorm.DB) error {
		return conn.Exec("VACUUM INTO " + quoteSQLiteString(backupPath)).Error
	}); err != nil {
		return "", err
	}

	return backupPath, nil
}
