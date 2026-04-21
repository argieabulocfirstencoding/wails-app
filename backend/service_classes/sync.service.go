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

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type SyncService struct {
	ctx              context.Context
	isConnected      bool
	AuthUser         *models.AuthUser
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

func (s *SyncService) SyncTaskRecord(taskRecord *models.TaskRecord) (err error) {

	defer func() {
		if err != nil {
			s.Logger.Println("Failed to sync task record:", err)
			runtime.EventsEmit(s.ctx, "record-synced", false)
		} else {
			runtime.EventsEmit(s.ctx, "record-synced", true)
		}
	}()

	Save := func() error {
		if err := db.DB.Save(taskRecord).Error; err != nil {
			s.Logger.Println("Failed to save user:", err)
			return err
		}
		return nil
	}

	SaveStatus := func() error {
		now := time.Now()
		taskRecord.SyncedAt = &now
		taskRecord.SyncStatus = "synced"
		taskRecord.Duration = int32(now.Sub(taskRecord.StartedAt).Seconds())

		return Save()
	}

	taskRecord.SyncStatus = "pending"
	Save()

	if taskRecord.EndedAt != nil {
		s.Logger.Printlnf("Syncing TASK OUT")
		if resp, err := s.FesClientService.Request(types.Request{
			Url:    "wf/task_out",
			Method: "POST",
			Body: map[string]any{
				"productivity":    taskRecord.Productivity,
				"total_task_time": taskRecord.Duration,
				"task_out":        taskRecord.EndedAt.UnixMilli(),
			},
		}); err != nil {
			return err
		} else {
			s.Logger.Printlnf("Sync Task Record Response: %v", resp)

			return SaveStatus()
		}
	}

	padagan := "Not Padagan"
	if taskRecord.Padagan {
		padagan = "Padagan"
	}

	s.Logger.Printlnf("Syncing TASK IN")
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
			"task_time":         taskRecord.Duration,
			"login_logout":      *taskRecord.Attendance.LoginLogoutID,
		},
	}); err != nil {
		return err
	} else {
		s.Logger.Printlnf("Sync Task Record Response: %v", resp)

		s.Logger.Printlnf("PRODUCTIVITY ID: %v", resp["productivity"])

		if productivity, ok := resp["productivity"].(string); ok {
			// jsonData, err := json.Marshal(productivity)
			// if err != nil {
			// 	return err
			// }
			// // Cast to RawMessage so it stays as valid JSON
			// p := json.RawMessage(jsonData)
			// taskRecord.Productivity = &p

			taskRecord.Productivity = &productivity
		}

		return SaveStatus()

	}
}

func (s *SyncService) SyncAttendance(attendance *models.Attendance) (err error) {

	defer func() {
		if err != nil {
			s.Logger.Println("Failed to sync task record:", err)
			runtime.EventsEmit(s.ctx, "attendance-synced", false)
		} else {
			runtime.EventsEmit(s.ctx, "attendance-synced", true)
		}
	}()

	Save := func() {
		if err := db.DB.Save(attendance).Error; err != nil {
			s.Logger.Println("Failed to save user:", err)
		}
	}

	SaveStatus := func() {
		now := time.Now()
		attendance.SyncedAt = &now
		attendance.SyncStatus = "synced"
		Save()
	}

	attendance.SyncStatus = "pending"
	Save()

	if attendance.TimeOut != nil && attendance.LoginLogoutID != nil {
		if resp, err := s.FesClientService.Request(types.Request{
			Url:    "wf/logout",
			Method: "POST",
			Body: map[string]any{
				"email":        s.AuthUser.Email,
				"login_logout": *attendance.LoginLogoutID,
				"date_logout":  attendance.TimeOut.UnixMilli(),
				"duration":     int(attendance.TimeOut.Sub(attendance.TimeIn).Seconds()),
			},
		}); err != nil {
			return err
		} else {
			s.Logger.Printlnf("Sync Attendance Response: %v", resp)
			SaveStatus()
			return nil
		}

	}

	body := map[string]any{
		"login": attendance.TimeIn.UnixMilli()}

	if attendance.LoginLogoutID != nil {
		body["login_logout_id"] = *attendance.LoginLogoutID
	}

	if resp, err := s.FesClientService.Request(types.Request{
		Url:    "wf/login_attendance",
		Method: "POST",
		Body:   body,
	}); err != nil {
		s.Logger.Println("Attempt sync attendance error:", err)
		return err
	} else {
		s.Logger.Println("Sync Attendance Response:", resp)
		if loginLogout, ok := resp["login_logout"].(string); ok {
			s.Logger.Println("LOGIN ID: ", loginLogout)
			attendance.LoginLogoutID = &loginLogout
			SaveStatus()
		}
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
