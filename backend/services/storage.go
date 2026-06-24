package services

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func getFolderSize(path string) (int64, error) {
	var size int64

	err := filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			size += info.Size()
		}
		return nil
	})

	return size, err
}

func ResolveScreenshotStoragePath(ctx context.Context) (string, error) {
	if isDevelopmentBuild(ctx) {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(wd, "backend", "screenshots"), nil
	}

	cacheDir, err := os.UserCacheDir()
	if err == nil && strings.TrimSpace(cacheDir) != "" {
		return filepath.Join(cacheDir, "FESApp", "screenshots"), nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	base := filepath.Dir(exePath)
	return filepath.Join(base, "backend", "screenshots"), nil
}

func EnsureScreenshotStoragePath(ctx context.Context) (string, error) {
	path, err := ResolveScreenshotStoragePath(ctx)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return "", err
	}

	return path, nil
}

func isDevelopmentBuild(ctx context.Context) bool {
	if _, exists := os.LookupEnv("DEVELOPMENT"); exists {
		return true
	}
	if ctx == nil {
		return false
	}

	var buildType string
	ok := false
	func() {
		defer func() {
			if recover() != nil {
				ok = false
			}
		}()
		buildType = runtime.Environment(ctx).BuildType
		ok = true
	}()

	return ok && strings.EqualFold(strings.TrimSpace(buildType), "dev")
}

func GetScreenshotStorageSize(ctx context.Context) (int64, error) {
	path, err := ResolveScreenshotStoragePath(ctx)
	if err != nil {
		return 0, err
	}

	// If folder does not exist, return 0 instead of error
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return 0, nil
	}

	return getFolderSize(path)
}

func ClearScreenshots(ctx context.Context) error {
	path, err := ResolveScreenshotStoragePath(ctx)
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		entryPath := filepath.Join(path, entry.Name())
		err := os.RemoveAll(entryPath) // removes files or subfolders recursively
		if err != nil {
			return err
		}
	}

	return nil
}
