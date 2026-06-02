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

	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return "", err
	}

	if err := copyFile(dbPath, backupPath); err != nil {
		return "", err
	}

	// SQLite WAL databases keep live state in the -wal/-shm sidecars.
	// Back up those files as well so any rollback has the same consistent view
	// of the database that existed before migrations started.
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(dbPath + suffix); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}

		if err := copyFile(dbPath+suffix, backupPath+suffix); err != nil {
			return "", err
		}
	}

	return backupPath, nil
}

func copyFile(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}

	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	return dst.Sync()
}
