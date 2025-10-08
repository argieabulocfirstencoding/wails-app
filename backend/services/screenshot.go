package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	config "fesapp/backend"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fmt"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/kbinani/screenshot"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

func screenshotNotify(message string) {
	exePath, _ := os.Executable()
	root := filepath.Dir(exePath)
	iconPath := filepath.Join(root, "icon.ico")

	//NOTE Icon is duplicated and placed in build/bin/ and build/windows/
	beeep.Notify("Screenshot", message, iconPath)
}

func GetMonitorCount() int {
	n := screenshot.NumActiveDisplays()
	fmt.Printf("You have %d displays\n", n)

	return n
}

type ScreenshotService struct {
	ctx          context.Context
	FolderName   string
	stopChan     chan struct{}
	running      bool
	TaskRecordID *uint
	TaskID       *string
	SyncService  *SyncService
}

func NewScreenshotService(sync *SyncService) *ScreenshotService {
	return &ScreenshotService{
		stopChan:    make(chan struct{}),
		SyncService: sync,
	}
}

func (s *ScreenshotService) Start(ctx context.Context) {
	s.ctx = ctx

	const TIME_TO_SCREENSHOT_IN_MINUTES time.Duration = 5
	if s.running {
		return
	}
	s.running = true

	fmt.Println("Started Screenshot Tracking")
	go func() {
		ticker := time.NewTicker(time.Minute * TIME_TO_SCREENSHOT_IN_MINUTES)
		defer ticker.Stop()
		defer func() { s.running = false }()

		go func() {
			if _, err := s.Screenshot(); err != nil {
				fmt.Println("screenshot error:", err) // don't kill app
			}

			// SYNCING PROCESS
			if config.OnlineMode {
				// Retrieving TaskRecord from TaskRecordID
				log.Print("Attempting to sync record")
				taskRecord, err := GetTaskRecordFormID(*s.TaskRecordID)
				if err != nil {
					log.Printf("error retrieving task record: %v\n", err)
				}

				log.Printf("Syncing record: %v\n", taskRecord)
				// Syncing TaskRecord to Firebase
				if err := s.SyncService.SyncTaskRecord(taskRecord); err != nil {
					log.Printf("error syncing task_record: %v", err)
				}
			}

		}()

		for {
			select {
			case <-ticker.C:
				if _, err := s.Screenshot(); err != nil {
					fmt.Println("screenshot error:", err)
				}
			case <-s.stopChan:
				fmt.Println("Stopping screenshot service...")
				return
			case <-ctx.Done():
				fmt.Println("Context canceled, stopping screenshot service...")
				return
			}
		}
	}()

}

func (s *ScreenshotService) Stop() {
	fmt.Println("Stopped Screenshot Tracking")
	// Retrieving TaskRecord from TaskRecordID
	if s.running {
		select {
		case s.stopChan <- struct{}{}:
		default: // avoid blocking if already stopped
		}
	}
}

func (s *ScreenshotService) Screenshot() ([]string, error) {
	n := GetMonitorCount()
	fmt.Println("📸 Taking screenshot...")
	// File path where screenshots are saved
	outputDir, err := filepath.Abs(filepath.Join("backend", "screenshots", s.FolderName))
	if err != nil {
		panic(err)
	}

	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		panic(err)
	}

	pictures := []string{}
	paths := []string{}

	// Timestamp name of the screenshots
	now := time.Now()
	pictureName := now.Format("20060102_150405")
	// Loop through all displays and save screenshot
	for i := 0; i < n; i++ {

		bounds := screenshot.GetDisplayBounds(i)

		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			return nil, err
		}

		fileName := filepath.Join(outputDir, fmt.Sprintf("screenshot_%s_display%d.png", pictureName, i))
		file, err := os.Create(fileName)
		if err != nil {
			return nil, err
		}

		if err := png.Encode(file, img); err != nil {
			file.Close()
			return nil, err
		}
		file.Close()

		// Convert to base64 for preview
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
		base64Img := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

		fmt.Printf("Saved %s\n", fileName)
		pictures = append(pictures, base64Img)
		paths = append(paths, fileName)
	}

	// JSON Paths to store array of screenshots in local db
	jsonPaths, err := json.Marshal(paths)
	if err != nil {
		return nil, err
	}

	if err := db.DB.Create(&models.Screenshot{
		TaskRecordID: *s.TaskRecordID,
		TaskID:       *s.TaskID,
		FilePath:     string(jsonPaths),
		TakenAt:      time.Now().Local(),
	}).Error; err != nil {
		return nil, err
	}

	runtime.EventsEmit(s.ctx, "screenshot", pictures)
	screenshotNotify("Your work progress have been captured")

	return pictures, nil
}

func (s *ScreenshotService) GetLatestRecordsWithScreenshots(userId, taskId string) (*models.TaskRecordWithScreenshots, error) {

	trws := &models.TaskRecordWithScreenshots{}

	expTaskRecord, err := GetLastTaskRecord(userId, taskId)
	if err != nil {
		return nil, err
	}

	var screenshot models.Screenshot
	if err := db.DB.Where("task_id = ?", expTaskRecord.TaskID).Last(&screenshot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// ✅ No screenshot found, not an error for your flow
			fmt.Println("No screenshot found for task:", expTaskRecord.TaskID)
		} else {
			// ❌ Real DB error
			fmt.Println("DB error:", err)
			return nil, err
		}
	}

	// ✅ Screenshot found, now you can unmarshal
	// example: if FilePath is JSON array of paths
	var pictures []string
	var paths []string
	if err := json.Unmarshal([]byte(screenshot.FilePath), &paths); err == nil {
		for _, path := range paths {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			base64Img := "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
			pictures = append(pictures, base64Img)
		}
	}

	trws.Record = expTaskRecord
	trws.Screenshots = pictures
	trws.Date = screenshot.TakenAt
	return trws, nil
}
