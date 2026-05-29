package db

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func createMigrationBackup(dbPath string) (string, error) {
	if dbPath == "" {
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
	src, err := os.Open(dbPath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return "", err
	}

	dst, err := os.Create(backupPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}

	return backupPath, nil
}
