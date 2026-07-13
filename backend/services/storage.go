package services

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
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

func ResolveUserScreenshotStoragePath(ctx context.Context, userID string) (string, error) {
	basePath, err := ResolveScreenshotStoragePath(ctx)
	if err != nil {
		return "", err
	}

	userFolder := sanitizeScreenshotStorageSegment(userID)
	if userFolder == "" {
		return "", fmt.Errorf("screenshot storage requires a valid user id")
	}

	return filepath.Join(basePath, userFolder), nil
}

func EnsureUserScreenshotStoragePath(ctx context.Context, userID string) (string, error) {
	path, err := ResolveUserScreenshotStoragePath(ctx, userID)
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

func GetScreenshotStorageSize(ctx context.Context, userID string) (int64, error) {
	path, err := ResolveUserScreenshotStoragePath(ctx, userID)
	if err != nil {
		return 0, err
	}

	// If folder does not exist, return 0 instead of error
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return 0, nil
	}

	return getFolderSize(path)
}

func ClearScreenshots(ctx context.Context, userID string) error {
	path, err := ResolveUserScreenshotStoragePath(ctx, userID)
	if err != nil {
		return err
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	return os.RemoveAll(path)
}

func sanitizeScreenshotStorageSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(value))
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteString(strconv.FormatInt(int64(r), 36))
		}
	}

	return builder.String()
}
