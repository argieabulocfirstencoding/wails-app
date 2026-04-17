package service_classes

import (
	"context"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/services"
	"fesapp/backend/types"
	"fesapp/backend/utils"
	"sync"
	"time"
)

type SyncService struct {
	ctx              context.Context
	isConnected      bool
	AuthUser         *models.FirebaseLoginResponse
	IdToken          *string
	IssuedAt         *time.Time
	Backblaze        *services.BackblazeSync
	FesClientService *FesClientService
	Logger           *utils.LoggerService

	connecting bool
	queue      []func()
	mu         sync.Mutex
}

func NewSyncService() *SyncService {
	backblaze := services.NewBackblazeSync()
	return &SyncService{
		Backblaze:        backblaze,
		FesClientService: NewFesClientService(),
		Logger:           utils.NewLoggerService(),
	}
}

func (s *SyncService) Connect(ctx context.Context) error {
	s.ctx = ctx

	return nil
}

func (s *SyncService) SyncCredentials() {

}

func (s *SyncService) SyncTasks() {

}

func (s *SyncService) SyncTaskRecord(taskRecord *models.TaskRecord) error {

	Save := func(taskRecord *models.TaskRecord) {
		if err := db.DB.Save(&taskRecord).Error; err != nil {
			s.Logger.Println("Failed to save user:", err)
		}
	}

	if taskRecord.EndedAt != nil && taskRecord.ProductivityID != nil {

		if resp, err := s.FesClientService.Request(types.Request{
			Url:    "wf/task_out",
			Method: "POST",
			Body: map[string]any{
				"productivity":    taskRecord.ProductivityID,
				"total_task_time": taskRecord.Duration,
				"task_out":        taskRecord.EndedAt.UnixMilli(),
			},
		}); err != nil {
			return err
		} else {
			s.Logger.Printlnf("Sync Task Record Response: %v", resp)

			now := time.Now()
			taskRecord.SyncedAt = &now
			Save(taskRecord)
			return nil
		}
	}

	padagan := "Not Padagan"
	if taskRecord.Padagan {
		padagan = "Padagan"
	}

	if resp, err := s.FesClientService.Request(types.Request{
		Url:    "wf/task_in",
		Method: "POST",
		Body: map[string]any{
			"task_in":           taskRecord.StartedAt.UnixMilli(),
			"date":              taskRecord.Attendance.TimeIn.UnixMilli(),
			"edited_by":         taskRecord.UserID,
			"employee_ID":       taskRecord.User.EmployeeId,
			"padagan":           padagan,
			"projects_projects": taskRecord.TaskID,
		},
	}); err != nil {
		return err
	} else {
		s.Logger.Printlnf("Sync Task Record Response: %v", resp)
		productivity_id, ok := resp["Productivty_ID"].(uint)
		if ok {
			now := time.Now()
			taskRecord.ProductivityID = &productivity_id
			taskRecord.SyncedAt = &now
			Save(taskRecord)
		}

	}

	return nil
}

func (s *SyncService) SyncAttendance(attendance *models.Attendance) error {

	resp, err := s.FesClientService.Request(types.Request{
		Url:    "wf/login_attendance",
		Method: "POST",
		Body: map[string]any{
			"login": attendance.TimeIn.UnixMilli()},
	})

	if err != nil {
		s.Logger.Println("Attempt sync attendance error:", err)
		return err
	}

	s.Logger.Println("Sync Attendance Response:", resp)

	_, ok := resp["user_id"].(string)
	if !ok {
		return s.Logger.Error("Something went wrong when syncing attendance!")
	}

	return nil
}

func (s *SyncService) SyncAllAttendance() error {
	return nil
}

func (s *SyncService) SyncUnsyncedRecords() error {
	return nil
}

func (s *SyncService) SyncUser(user models.AuthUser) error {
	return nil
}

func (s *SyncService) TestSync() bool {
	return true
}

func (s *SyncService) RefreshAccessToken() {

}
