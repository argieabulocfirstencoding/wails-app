package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateMigrationBackup_CopiesWALSidecars(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fesapp.db")

	if err := os.WriteFile(dbPath, []byte("main-db"), 0o644); err != nil {
		t.Fatalf("write main db: %v", err)
	}
	if err := os.WriteFile(dbPath+"-wal", []byte("wal-data"), 0o644); err != nil {
		t.Fatalf("write wal file: %v", err)
	}
	if err := os.WriteFile(dbPath+"-shm", []byte("shm-data"), 0o644); err != nil {
		t.Fatalf("write shm file: %v", err)
	}

	backupPath, err := createMigrationBackup(dbPath)
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if backupPath == "" {
		t.Fatalf("expected backup path")
	}

	assertCopied := func(path, expected string) {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read backup %s: %v", path, err)
		}
		if strings.TrimSpace(string(data)) != expected {
			t.Fatalf("unexpected contents for %s: got %q want %q", path, string(data), expected)
		}
	}

	assertCopied(backupPath, "main-db")
	assertCopied(backupPath+"-wal", "wal-data")
	assertCopied(backupPath+"-shm", "shm-data")
}
