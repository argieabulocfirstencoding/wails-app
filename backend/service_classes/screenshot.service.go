package service_classes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/repository"
	"fmt"
	"os"

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
}

func NewScreenshotService() *ScreenshotService {
	return &ScreenshotService{
		TaskRepo: repository.NewTaskRepository(),
	}
}

func (s *ScreenshotService) Screenshot() ([]string, error) {
	return nil, nil
}

func (s *ScreenshotService) Start(ctx context.Context) {
	s.ctx = ctx
}

func (s *ScreenshotService) Stop() {

}

func (s *ScreenshotService) AddMemo(memo, notes string) {

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
