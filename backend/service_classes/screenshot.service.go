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
	"fmt"
	"os"
	"path/filepath"
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

	mu          sync.Mutex
	cancel      context.CancelFunc
	workerDone  chan struct{}
	uploadRetry int
	syncRetry   int
}

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
		uploadRetry:  3,
		syncRetry:    3,
	}
}

func (s *ScreenshotService) Screenshot() ([]string, error) {
	s.mu.Lock()
	ctx := s.ctx
	folderName := s.FolderName
	taskRecordID := s.TaskRecordID
	taskID := s.TaskID
	memo := s.Memo
	notes := s.Notes
	s.mu.Unlock()

	if taskRecordID == nil {
		return nil, fmt.Errorf("screenshot capture aborted: task record ID is nil")
	}
	if taskID == nil || *taskID == "" {
		return nil, fmt.Errorf("screenshot capture aborted: task ID is nil")
	}

	outputDir := filepath.Join("backend", "screenshots", folderName)
	if folderName == "" {
		outputDir = filepath.Join("backend", "screenshots", fmt.Sprintf("%d", time.Now().UnixMilli()))
	}
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return nil, err
	}

	displayCount := screenshot.NumActiveDisplays()
	if displayCount < 1 {
		return nil, fmt.Errorf("no active display detected")
	}

	pictures := make([]string, 0, displayCount)
	paths := make([]string, 0, displayCount)
	pictureName := time.Now().Format("20060102_150405")

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

		fileName := filepath.Join(outputDir, fmt.Sprintf("screenshot_%s_display%d.webp", pictureName, i))
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
		Memo:         memo,
		Notes:        notes,
		FilePath:     string(jsonPaths),
		TakenAt:      time.Now().Local(),
	}

	if err := db.DB.Create(taskScreenshot).Error; err != nil {
		return nil, err
	}

	if ctx != nil {
		runtime.EventsEmit(ctx, "screenshot", pictures)
	}

	s.trySync(taskScreenshot, paths)
	return pictures, nil
}

func (s *ScreenshotService) Start(ctx context.Context) {
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

	interval := 5 * time.Minute
	if s.ctx != nil {
		env := runtime.Environment(s.ctx)
		if env.BuildType == "dev" {
			interval = time.Minute
		}
	}

	go func() {
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

		_, _ = s.Screenshot()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
				_, _ = s.Screenshot()
			}
		}
	}()
}

func (s *ScreenshotService) Stop() {
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

func (s *ScreenshotService) AddMemo(memo, notes string) {
	s.mu.Lock()
	s.Memo = memo
	s.Notes = notes
	s.mu.Unlock()
}

func (s *ScreenshotService) GetLatestRecordsWithScreenshots(userId, taskId string) (*models.TaskRecordWithScreenshots, error) {
	trws := &models.TaskRecordWithScreenshots{}

	taskRecord, err := s.TaskRepo.GetLastTaskRecord(userId, taskId)
	if taskRecord == nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	expTaskRecord := taskRecord.ToExp()
	trws.Record = &expTaskRecord

	var screenshot models.Screenshot
	if err := db.DB.Where("task_id = ?", expTaskRecord.TaskID).Last(&screenshot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// ✅ No screenshot found, not an error for your flow
			fmt.Println("No screenshot found for task:", expTaskRecord.TaskID)
		} else {
			// ❌ Real DB error
			fmt.Println("DB error:", err)
			return trws, err
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
				return trws, err
			}
			base64Img := "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
			pictures = append(pictures, base64Img)
		}
	}

	trws.Screenshots = pictures
	trws.Date = screenshot.TakenAt
	return trws, nil
}

func (s *ScreenshotService) trySync(taskScreenshot *models.Screenshot, paths []string) {
	if taskScreenshot == nil || s.SyncService == nil {
		return
	}

	if s.Connectivity.IsOnline() && s.SyncService.Backblaze != nil && len(paths) > 0 {
		backoff := 800 * time.Millisecond
		for attempt := 1; attempt <= s.uploadRetry; attempt++ {
			remotePaths, err := s.SyncService.Backblaze.AttemptUpload(paths)
			if err == nil {
				if len(remotePaths) > 0 {
					if jsonRemotePaths, marshalErr := json.Marshal(remotePaths); marshalErr == nil {
						remoteString := string(jsonRemotePaths)
						if err := db.DB.Model(taskScreenshot).Update("cloud_url", remoteString).Error; err != nil {
							fmt.Printf("screenshot sync update cloud_url failed task_record_id=%d err=%v\n", taskScreenshot.TaskRecordID, err)
						}
					} else {
						fmt.Printf("screenshot sync marshal remote paths failed task_record_id=%d err=%v\n", taskScreenshot.TaskRecordID, marshalErr)
					}
				}
				break
			}

			if attempt == s.uploadRetry {
				break
			}
			time.Sleep(backoff)
			backoff *= 2
		}
	}

	backoff := 800 * time.Millisecond
	for attempt := 1; attempt <= s.syncRetry; attempt++ {
		err := s.SyncService.SyncTaskRecord(taskScreenshot.TaskRecordID)
		if err == nil {
			return
		}

		if attempt == s.syncRetry {
			return
		}
		time.Sleep(backoff)
		backoff *= 2
	}
}
