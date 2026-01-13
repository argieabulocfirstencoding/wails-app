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
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/chai2010/webp"

	"github.com/kbinani/screenshot"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

func screenshotNotify(message string) {
	// NOTE this was preferred to be silent and only for dev purposes

	// notification := toast.Notification{
	// 	AppID: "FESAPP.Tracker",
	// 	Title: "Screenshot",
	// 	Body:  message,
	// 	Icon:  "icon.ico",
	// }

	// notification.Push()
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
	Running      bool
	Memo         string
	Notes        string
	TaskRecordID *uint
	TaskID       *string
	SyncService  *SyncService
}

func NewScreenshotService(sync *SyncService) *ScreenshotService {
	return &ScreenshotService{
		stopChan:    make(chan struct{}),
		SyncService: sync,
		Memo:        "",
		Notes:       "",
	}
}

func (s *ScreenshotService) AddMemo(memo, notes string) {
	s.Memo = memo
	s.Notes = notes
}

func (s *ScreenshotService) Start(ctx context.Context) {
	s.ctx = ctx
	taskRecordIDCopy := *s.TaskRecordID
	env := runtime.Environment(s.ctx)

	var TIME_TO_SCREENSHOT_IN_MINUTES time.Duration
	if env.BuildType == "dev" {
		TIME_TO_SCREENSHOT_IN_MINUTES = 1
	} else {
		TIME_TO_SCREENSHOT_IN_MINUTES = 5
	}

	if s.Running {
		return
	}
	s.Running = true

	fmt.Println("Started Screenshot Tracking")
	go func() {
		ticker := time.NewTicker(time.Minute * TIME_TO_SCREENSHOT_IN_MINUTES)
		defer ticker.Stop()
		defer func() { s.Running = false }()

		if _, err := s.Screenshot(); err != nil {
			fmt.Println("screenshot error:", err) // don't kill app
		}
		// SYNCING PROCESS
		s.syncScreenshot(taskRecordIDCopy)

		for {
			select {
			case <-ticker.C:
				if _, err := s.Screenshot(); err != nil {
					fmt.Println("screenshot error:", err)
				}
				// SYNCING PROCESS
				s.syncScreenshot(taskRecordIDCopy)
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
	if s.Running {
		select {
		case s.stopChan <- struct{}{}:
		default: // avoid blocking if already stopped
		}
	}
}

func (s *ScreenshotService) Screenshot() ([]string, error) {
	taskRecordIDCopy := *s.TaskRecordID
	taskIDCopy := *s.TaskID
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
		img, err := screenshot.CaptureRect(bounds) // img is *image.RGBA
		if err != nil {
			return nil, err
		}

		// --- ENCODING TO WEBP ---
		var buf bytes.Buffer

		// Use EncodeRGBA since img is a *image.RGBA, which satisfies image.Image
		// We use 90 as a high-quality lossy compression factor (0-100).
		// Use 100 for true lossless.
		const WEBP_QUALITY_FACTOR = 90

		// EncodeRGBA returns []byte, so we write it directly to the buffer
		encodedData, err := webp.EncodeRGBA(img, WEBP_QUALITY_FACTOR)
		if err != nil {
			return nil, err
		}
		buf.Write(encodedData)

		// 1. Write the buffer content to the file
		fileName := filepath.Join(outputDir, fmt.Sprintf("screenshot_%s_display%d.webp", pictureName, i))
		if err := os.WriteFile(fileName, buf.Bytes(), 0644); err != nil {
			return nil, err
		}

		// 2. Create Base64 from the buffer content
		base64Img := "data:image/webp;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

		fmt.Printf("Saved %s\n", fileName)
		pictures = append(pictures, base64Img)
		paths = append(paths, fileName)
	}

	// JSON Paths to store array of screenshots in local db
	jsonPaths, err := json.Marshal(paths)
	if err != nil {
		return nil, err
	}

	taskScreenshot := &models.Screenshot{
		TaskRecordID: taskRecordIDCopy,
		TaskID:       taskIDCopy,
		Memo:         s.Memo,
		Notes:        s.Notes,
		FilePath:     string(jsonPaths),
		TakenAt:      time.Now().Local(),
	}

	if err := db.DB.Create(taskScreenshot).Error; err != nil {
		return nil, err
	}

	runtime.EventsEmit(s.ctx, "screenshot", pictures)
	screenshotNotify("Your work progress have been captured")

	go func() {
		log.Printf("Upload called — isConnected=%v", s.SyncService.Backblaze.isConnected)
		if config.OnlineMode {
			taskScreenshotModel := taskScreenshot
			remotePaths, err := s.SyncService.Backblaze.AttemptUpload(paths)
			if err != nil {
				log.Print(err)
			}
			if remotePaths != nil {
				jsonRemotePaths, _ := json.Marshal(remotePaths)
				if jsonRemotePaths != nil {
					stringifiedJsonPath := string(jsonRemotePaths)
					db.DB.Model(&taskScreenshotModel).Update("cloud_url", stringifiedJsonPath)
				}
			}
		}
	}()

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

func (s *ScreenshotService) syncScreenshot(taskRecordIDCopy uint) {
	if config.OnlineMode {
		// Retrieving TaskRecord from TaskRecordID
		log.Print("Attempting to sync record")
		taskRecord, err := GetTaskRecordFromID(taskRecordIDCopy)
		if err != nil {
			log.Printf("error retrieving task record: %v\n", err)
		}

		s.SyncRecord(taskRecord)
	}
}

func (s *ScreenshotService) SyncRecord(taskRecord *models.TaskRecord) {
	attendance, _ := GetUserAttendance(s.SyncService.AuthUser.LocalID)
	db.DB.Model(&attendance).Update("sync_status", "pending")
	db.DB.Model(&taskRecord).Update("sync_status", "pending")

	go func() {
		log.Printf("Syncing record: %v\n", taskRecord)
		// Syncing TaskRecord to Firebase
		if err := s.SyncService.SyncTaskRecord(taskRecord); err != nil {
			return
		}

	}()
}
