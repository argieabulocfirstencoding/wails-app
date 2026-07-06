package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func setDevelopmentScreenshotTestRoot(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	t.Setenv("DEVELOPMENT", "1")

	return filepath.Join(tmpDir, "backend", "screenshots")
}

func TestGetScreenshotStorageSizeScopesToUser(t *testing.T) {
	ctx := context.Background()
	root := setDevelopmentScreenshotTestRoot(t)

	userOnePath, err := EnsureUserScreenshotStoragePath(ctx, "user-1")
	if err != nil {
		t.Fatalf("ensure user-1 path: %v", err)
	}
	userTwoPath, err := EnsureUserScreenshotStoragePath(ctx, "user-2")
	if err != nil {
		t.Fatalf("ensure user-2 path: %v", err)
	}

	if err := os.WriteFile(filepath.Join(userOnePath, "one.webp"), []byte("12345"), 0644); err != nil {
		t.Fatalf("write user-1 file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userTwoPath, "two.webp"), []byte("123456789"), 0644); err != nil {
		t.Fatalf("write user-2 file: %v", err)
	}

	userOneSize, err := GetScreenshotStorageSize(ctx, "user-1")
	if err != nil {
		t.Fatalf("get user-1 size: %v", err)
	}
	if userOneSize != 5 {
		t.Fatalf("expected user-1 size 5, got %d", userOneSize)
	}

	userTwoSize, err := GetScreenshotStorageSize(ctx, "user-2")
	if err != nil {
		t.Fatalf("get user-2 size: %v", err)
	}
	if userTwoSize != 9 {
		t.Fatalf("expected user-2 size 9, got %d", userTwoSize)
	}

	if _, err := os.Stat(filepath.Join(root, "user-1", "one.webp")); err != nil {
		t.Fatalf("expected user-1 file at scoped path: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "user-2", "two.webp")); err != nil {
		t.Fatalf("expected user-2 file at scoped path: %v", err)
	}
}

func TestClearScreenshotsRemovesOnlyRequestedUserFolder(t *testing.T) {
	ctx := context.Background()
	_ = setDevelopmentScreenshotTestRoot(t)

	userOnePath, err := EnsureUserScreenshotStoragePath(ctx, "user-1")
	if err != nil {
		t.Fatalf("ensure user-1 path: %v", err)
	}
	userTwoPath, err := EnsureUserScreenshotStoragePath(ctx, "user-2")
	if err != nil {
		t.Fatalf("ensure user-2 path: %v", err)
	}

	if err := os.WriteFile(filepath.Join(userOnePath, "one.webp"), []byte("user-one"), 0644); err != nil {
		t.Fatalf("write user-1 file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userTwoPath, "two.webp"), []byte("user-two"), 0644); err != nil {
		t.Fatalf("write user-2 file: %v", err)
	}

	if err := ClearScreenshots(ctx, "user-1"); err != nil {
		t.Fatalf("clear user-1 screenshots: %v", err)
	}

	if _, err := os.Stat(userOnePath); !os.IsNotExist(err) {
		t.Fatalf("expected user-1 folder removed, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(userTwoPath, "two.webp")); err != nil {
		t.Fatalf("expected user-2 file to remain: %v", err)
	}
}
