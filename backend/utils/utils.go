package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var Logger *log.Logger
var loggerMu sync.Mutex
var loggerFile *os.File

func init() {
	// Default to stdout. Startup will route logs to file based on build type.
	Logger = log.New(os.Stdout, "FES_LOG: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func ConfigureLogger(buildType string) error {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	logPath := "log.txt"
	if buildType != "dev" {
		exePath, err := os.Executable()
		if err != nil {
			return err
		}
		logPath = filepath.Join(filepath.Dir(exePath), "logs.txt")
	}

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	if loggerFile != nil {
		_ = loggerFile.Close()
	}
	loggerFile = file

	Logger.SetOutput(file)
	Logger.Printf("Logger initialized. buildType=%s path=%s", buildType, logPath)
	return nil
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
