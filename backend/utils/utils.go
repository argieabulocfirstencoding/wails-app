package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var Logger *log.Logger
var loggerMu sync.Mutex
var loggerFile *os.File
var loggerPath string

const (
	maxLogFileSizeBytes = 10 * 1024 * 1024
	maxRetainedLogFiles = 7
)

func init() {
	// Default to stdout. Startup will route logs to file based on build type.
	Logger = log.New(os.Stdout, "FES_LOG: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func ConfigureLogger(buildType string) error {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	candidates, err := candidateLogPaths(buildType)
	if err != nil {
		return err
	}

	var file *os.File
	var selectedPath string
	var openErrs []string

	for _, path := range candidates {
		if mkErr := os.MkdirAll(filepath.Dir(path), 0755); mkErr != nil {
			openErrs = append(openErrs, fmt.Sprintf("%s (mkdir: %v)", path, mkErr))
			continue
		}
		if rotateErr := rotateLogFile(path); rotateErr != nil {
			openErrs = append(openErrs, fmt.Sprintf("%s (rotate: %v)", path, rotateErr))
			continue
		}

		f, openErr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if openErr != nil {
			openErrs = append(openErrs, fmt.Sprintf("%s (open: %v)", path, openErr))
			continue
		}

		file = f
		selectedPath = path
		break
	}

	if file == nil {
		return fmt.Errorf("failed to open any log file path: %s", strings.Join(openErrs, " | "))
	}

	if loggerFile != nil {
		_ = loggerFile.Close()
	}
	loggerFile = file
	loggerPath = selectedPath

	Logger.SetOutput(io.MultiWriter(os.Stdout, file))
	Logger.Printf("Logger initialized. buildType=%s path=%s", buildType, selectedPath)
	return nil
}

func CurrentLogPath() string {
	loggerMu.Lock()
	defer loggerMu.Unlock()
	return loggerPath
}

func rotateLogFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if info.Size() < maxLogFileSizeBytes {
		return pruneRotatedLogs(path)
	}

	rotatedPath := fmt.Sprintf("%s.%s", path, time.Now().UTC().Format("20060102_150405"))
	if err := os.Rename(path, rotatedPath); err != nil {
		return err
	}

	return pruneRotatedLogs(path)
}

func pruneRotatedLogs(path string) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	type candidate struct {
		path    string
		modTime time.Time
	}

	var rotated []candidate
	prefix := base + "."
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}

		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		rotated = append(rotated, candidate{
			path:    filepath.Join(dir, name),
			modTime: info.ModTime(),
		})
	}

	if len(rotated) <= maxRetainedLogFiles {
		return nil
	}

	sort.Slice(rotated, func(i, j int) bool {
		return rotated[i].modTime.After(rotated[j].modTime)
	})

	for i := maxRetainedLogFiles; i < len(rotated); i++ {
		_ = os.Remove(rotated[i].path)
	}

	return nil
}

func candidateLogPaths(buildType string) ([]string, error) {
	paths := make([]string, 0, 5)

	if buildType == "dev" {
		paths = append(paths, "log.txt")
	} else {
		exePath, err := os.Executable()
		if err != nil {
			return nil, err
		}
		paths = append(paths, filepath.Join(filepath.Dir(exePath), "logs.txt"))
	}

	if appData, err := os.UserConfigDir(); err == nil && appData != "" {
		paths = append(paths, filepath.Join(appData, "FESApp", "logs.txt"))
	}

	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		paths = append(paths, filepath.Join(localAppData, "FESApp", "logs.txt"))
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		paths = append(paths, filepath.Join(home, "FESApp", "logs.txt"))
	}

	paths = append(paths, "log.txt")
	return uniqueNonEmpty(paths), nil
}

func uniqueNonEmpty(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))

	for _, path := range paths {
		normalized := strings.TrimSpace(path)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	return result
}

func GetStringValue(field interface{}) string {
	if field == nil {
		return ""
	}
	if m, ok := field.(map[string]interface{}); ok {
		if v, ok := m["stringValue"].(string); ok {
			return v
		}
	}
	return ""
}

// helper to extract integerValue
func GetIntValue(field interface{}) int {
	if field == nil {
		return 0
	}
	if m, ok := field.(map[string]interface{}); ok {
		if v, ok := m["integerValue"].(string); ok {
			var num int
			fmt.Sscanf(v, "%d", &num)
			return num
		}
	}
	return 0
}
