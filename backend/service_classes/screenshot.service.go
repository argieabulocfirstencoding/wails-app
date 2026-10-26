package service_classes

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/repository"
	"fesapp/backend/services"
	"fesapp/backend/utils"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chai2010/webp"
	"github.com/kbinani/screenshot"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

type ScreenshotService struct {
	FolderName   string
	TaskID       *string
	TaskRecordID *uint
	TaskRepo     *repository.TaskRepository
	ctx          context.Context
	Running      bool
	Memo         string
	Notes        string
	SyncService  *SyncService
	Connectivity *ConnectivityStateService

	mu         sync.Mutex
	cancel     context.CancelFunc
	workerDone chan struct{}
}

const screenshotsEnabled = false
const PRODUCTION_SCREENSHOT_INTERVAL = 10
const DEVELOPMENT_SCREENSHOT_INTERVAL = 1

func NewScreenshotService(connectivity ...*ConnectivityStateService) *ScreenshotService {
	var connectivityState *ConnectivityStateService
	if len(connectivity) > 0 {
		connectivityState = connectivity[0]
	}
	if connectivityState == nil {
		connectivityState = NewConnectivityStateService()
	}
	return &ScreenshotService{
		TaskRepo:     repository.NewTaskRepository(),
		Connectivity: connectivityState,
	}
}

func (s *ScreenshotService) Screenshot() ([]string, error) {
	if !screenshotsEnabled {
		return nil, nil
	}

	s.mu.Lock()
	ctx := s.ctx
	folderName := fmt.Sprintf("%d", *s.TaskRecordID)
	taskRecordID := s.TaskRecordID
	taskID := s.TaskID
	notes := s.Notes
	s.mu.Unlock()

	if taskRecordID == nil {
		return nil, fmt.Errorf("screenshot capture aborted: task record ID is nil")
	}
	if taskID == nil || *taskID == "" {
		return nil, fmt.Errorf("screenshot capture aborted: task ID is nil")
	}

	storageRoot, err := s.screenshotStorageRoot(ctx, *taskRecordID)
	if err != nil {
		return nil, err
	}

	folderName = sanitizeScreenshotFolderName(folderName)
	if folderName == "" {
		folderName = fmt.Sprintf("%d", time.Now().UnixMilli())
	}
	s.mu.Lock()
	s.FolderName = folderName
	s.mu.Unlock()

	outputDir := filepath.Join(storageRoot, folderName, fmt.Sprintf("%s", *taskID))
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return nil, err
	}

	displayCount := screenshot.NumActiveDisplays()
	if displayCount < 1 {
		return nil, fmt.Errorf("no active display detected")
	}

	pictures := make([]string, 0, displayCount)
	paths := make([]string, 0, displayCount)
	// pictureName := time.Now().Format("20060102_150405")
	pictureName := time.Now().UnixMilli()

	for i := 0; i < displayCount; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			return nil, err
		}

		encodedData, err := webp.EncodeRGBA(img, 90)
		if err != nil {
			return nil, err
		}

		var buf bytes.Buffer
		buf.Write(encodedData)

		fileName := filepath.Join(outputDir, fmt.Sprintf("screenshot_%d_display%d.webp", pictureName, i))
		if err := os.WriteFile(fileName, buf.Bytes(), 0644); err != nil {
			return nil, err
		}

		base64Img := "data:image/webp;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
		pictures = append(pictures, base64Img)
		paths = append(paths, fileName)
	}

	jsonPaths, err := json.Marshal(paths)
	if err != nil {
		return nil, err
	}

	taskScreenshot := &models.Screenshot{
		TaskRecordID: *taskRecordID,
		TaskID:       *taskID,
		Notes:        notes,
		FilePath:     string(jsonPaths),
		TakenAt:      time.Now().Local(),
	}

	if err := db.DB.Create(taskScreenshot).Error; err != nil {
		return nil, err
	}

	emitRuntimeEvent(ctx, "screenshot", pictures)
	utils.GoSafe("screenshot.try_sync", func() {
		s.trySync(taskScreenshot, paths)
	})
	return pictures, nil
}

func (s *ScreenshotService) screenshotStorageRoot(ctx context.Context, taskRecordID uint) (string, error) {
	if taskRecordID == 0 {
		return "", fmt.Errorf("screenshot capture aborted: task record ID is invalid")
	}

	taskRecord, err := s.TaskRepo.GetTaskRecord(taskRecordID)
	if err != nil {
		return "", err
	}
	if taskRecord == nil {
		return "", fmt.Errorf("screenshot capture aborted: task record not found")
	}

	userID := strings.TrimSpace(taskRecord.UserID)
	if userID == "" {
		return "", fmt.Errorf("screenshot capture aborted: task record user ID is empty")
	}

	return services.EnsureUserScreenshotStoragePath(ctx, userID)
}

func (s *ScreenshotService) Start(ctx context.Context) {
	if !screenshotsEnabled {
		return
	}

	s.mu.Lock()
	if s.Running {
		s.mu.Unlock()
		return
	}

	if s.TaskRecordID == nil || s.TaskID == nil || *s.TaskID == "" {
		s.mu.Unlock()
		return
	}

	if ctx != nil {
		s.ctx = ctx
	}

	runCtx := s.ctx
	if runCtx == nil {
		runCtx = context.Background()
	}

	workerCtx, cancel := context.WithCancel(runCtx)
	s.cancel = cancel
	s.workerDone = make(chan struct{})
	s.Running = true
	s.mu.Unlock()

	interval := screenshotInterval(s.ctx)

	utils.GoSafe("screenshot.worker", func() {
		defer func() {
			s.mu.Lock()
			s.Running = false
			if s.workerDone != nil {
				close(s.workerDone)
				s.workerDone = nil
			}
			s.cancel = nil
			s.mu.Unlock()
		}()

		if _, err := s.Screenshot(); err != nil {
			utils.Logger.Printf("screenshot capture failed: %v", err)
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				if _, err := s.Screenshot(); err != nil {
					utils.Logger.Printf("screenshot capture failed: %v", err)
				}
			}
		}
	})
}

func (s *ScreenshotService) Stop() {
	if !screenshotsEnabled {
		return
	}

	s.mu.Lock()
	cancel := s.cancel
	done := s.workerDone
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (s *ScreenshotService) AddMemo(notes string) {
	s.mu.Lock()
	s.Notes = notes
	s.mu.Unlock()
}

func (s *ScreenshotService) GetLatestRecordsWithScreenshots(userId, taskId string, attendanceID uint) (*models.TaskRecordWithScreenshots, error) {
	trws := &models.TaskRecordWithScreenshots{}

	taskRecord, err := s.TaskRepo.GetTaskRecordForDisplay(userId, taskId, attendanceID)
	if taskRecord == nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	expTaskRecord := taskRecord.ToExp()
	trws.Record = &expTaskRecord

	var screenshot models.Screenshot
	if err := db.DB.Where("task_record_id = ?", expTaskRecord.ID).Order("taken_at DESC").First(&screenshot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// ✅ No screenshot found, not an error for your flow
			trws.Screenshots = []string{}
			return trws, nil
		}
		// ❌ Real DB error
		fmt.Println("DB error:", err)
		return trws, err
	}

	// ✅ Screenshot found, now you can unmarshal
	// example: if FilePath is JSON array of paths
	var pictures []string
	for _, path := range decodeScreenshotPaths(screenshot.FilePath) {
		if strings.TrimSpace(path) == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return trws, err
		}
		base64Img := "data:image/webp;base64," + base64.StdEncoding.EncodeToString(data)
		pictures = append(pictures, base64Img)
	}

	trws.Screenshots = pictures
	trws.Date = screenshot.TakenAt
	return trws, nil
}

func (s *ScreenshotService) trySync(taskScreenshot *models.Screenshot, paths []string) {
	if !screenshotsEnabled {
		return
	}

	if taskScreenshot == nil || s.SyncService == nil {
		return
	}
	correlationID := utils.NewCorrelationID("screenshot_batch")
	s.SyncService.Logger.Infof("op=screenshot_batch_sync_start correlation_id=%s task_record_id=%d item_count=%d", correlationID, taskScreenshot.TaskRecordID, len(paths))

	if s.Connectivity == nil || !s.Connectivity.IsOnline() {
		s.SyncService.Logger.Infof("op=screenshot_batch_sync_skipped correlation_id=%s task_record_id=%d reason=offline", correlationID, taskScreenshot.TaskRecordID)
		return
	}

	if err := s.SyncService.SyncTaskRecord(taskScreenshot.TaskRecordID); err != nil {
		s.SyncService.Logger.Warnf("op=screenshot_batch_sync_failed correlation_id=%s task_record_id=%d err=%v", correlationID, taskScreenshot.TaskRecordID, err)
		return
	}

	s.SyncService.Logger.Infof("op=screenshot_batch_sync_success correlation_id=%s task_record_id=%d", correlationID, taskScreenshot.TaskRecordID)
}

func decodeScreenshotPaths(raw string) []string {
	var paths []string
	if err := json.Unmarshal([]byte(raw), &paths); err == nil {
		return paths
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	return []string{raw}
}

func sanitizeScreenshotFolderName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	var builder strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_':
			builder.WriteRune(r)
		}
	}

	return builder.String()
}

func screenshotInterval(ctx context.Context) time.Duration {
	if isDevelopmentRuntime(ctx) {
		return DEVELOPMENT_SCREENSHOT_INTERVAL * time.Minute
	}

	return PRODUCTION_SCREENSHOT_INTERVAL * time.Minute
}

func isDevelopmentRuntime(ctx context.Context) bool {
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

func emitRuntimeEvent(ctx context.Context, eventName string, payload any) {
	if ctx == nil {
		return
	}

	defer func() {
		_ = recover()
	}()
	runtime.EventsEmit(ctx, eventName, payload)
}
