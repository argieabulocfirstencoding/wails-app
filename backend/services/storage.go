package services

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

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

func getScreenshotPath(ctx context.Context) (string, error) {
	env := runtime.Environment(ctx)

	if env.BuildType == "dev" {
		// Development: assume working directory is project root
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(wd, "backend", "screenshots"), nil
	}

	// Production: use executable folder
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	base := filepath.Dir(exePath)
	return filepath.Join(base, "backend", "screenshots"), nil
}

func GetScreenshotStorageSize(ctx context.Context) (int64, error) {
	path, err := getScreenshotPath(ctx)
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
	path, err := getScreenshotPath(ctx)
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(path)
	if err != nil {
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
