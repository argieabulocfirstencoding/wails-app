package service_classes

import (
	"context"
	config "fesapp/backend"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/repository"
	"fesapp/backend/services"
	"fesapp/backend/types"
	"fesapp/backend/utils"
	"fmt"
	"strings"
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

	Attendance **models.Attendance
	TaskRecord **models.TaskRecord

	TaskRepository *repository.TaskRepository
	AuthRepository *repository.AuthRepository
}

type SyncQueue struct {
	ctx        context.Context
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


func (s *SyncService) SyncTasks() {

}

func (s *SyncService) SyncTaskRecord(taskRecordId uint) (err error) {

	defer func() {
		if err != nil {
			s.Logger.Println("Failed to sync task record:", err)
			runtime.EventsEmit(s.ctx, "record-synced", false)
		} else {
			runtime.EventsEmit(s.ctx, "record-synced", true)
		}
	}()

	if !config.OnlineMode {
		return fmt.Errorf("not currently synced")
	}

	taskRecord, err := s.TaskRepository.GetTaskRecord(taskRecordId)
	if err != nil {
		return err
	}

	attendance, err := s.AuthRepository.GetUserAttendance(taskRecord.UserID)
	if err != nil {
		return err
	}

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
				"activity_description": strings.ReplaceAll(taskRecord.Notes, "||", "-"),
				"productivity":         taskRecord.Productivity,
				"total_task_time":      taskRecord.Duration,
				"task_out":             taskRecord.EndedAt.UnixMilli(),
			},
		}); err != nil {
			return err
		} else {
			s.Logger.Printlnf("Sync Task Record Response: %v", resp)

			return SaveStatus()
		}
	}

	padagan := "No"
	if taskRecord.Padagan {
		padagan = "Yes"
	}

	s.Logger.Printlnf("Syncing TASK IN")
	if resp, err := s.FesClientService.Request(types.Request{
		Url:    "wf/task_in",
		Method: "POST",
		Body: map[string]any{
			"task_in":              taskRecord.StartedAt.UnixMilli(),
			"date":                 attendance.TimeIn.UnixMilli(),
			"edited_by":            taskRecord.UserID,
			"employee_ID":          (*s.Attendance).User.EmployeeId,
			"padaganan":            padagan,
			"projects_projects":    taskRecord.TaskID,
			"task_time":            taskRecord.Duration,
			"login_logout":         attendance.LoginLogoutID,
			"activity_description": taskRecord.Notes,
		},
	}); err != nil {
		return err
	} else {
		s.Logger.Printlnf("Sync Task Record Response: %v", resp)

		s.Logger.Printlnf("PRODUCTIVITY ID: %v", resp["productivity"])

		if productivity, ok := resp["productivity"].(string); ok {
			taskRecord.Productivity = &productivity
		}

		return SaveStatus()

	}
}

func (s *SyncService) SyncAttendance(attendance *models.Attendance) (err error) {
	if !config.OnlineMode {
		runtime.EventsEmit(s.ctx, "attendance-synced", false)
		return nil
	}

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

			if timeInRaw, ok := resp["login"].(float64); ok {
				// 1. Convert the float64 to an int64 (milliseconds)
				milli := int64(timeInRaw)

				// 2. Create the time.Time object using UnixMilli
				timeIn := time.UnixMilli(milli)

				s.Logger.Println("TIME IN: ", timeIn)
				attendance.TimeIn = timeIn
			}

			SaveStatus()
		}
	}

	return nil
}

func (s *SyncService) SyncActiveTaskRecord() (*models.TaskRecord, error) {
	if !config.OnlineMode && (*s.Attendance).LoginLogoutID == nil {
		runtime.EventsEmit(s.ctx, "attendance-synced", false)
		return nil, s.Logger.Errorf("cannot get active task record if not synced")
	}

	body := map[string]any{
		"login_logout_id": (*s.Attendance).LoginLogoutID,
	}
	// TODO continue this to fetch current active record
	if _, err := s.FesClientService.Request(types.Request{
		Url:    "wf/active_record",
		Method: "GET",
		Body:   body,
	}); err != nil {
		s.Logger.Println("Attempt sync attendance error:", err)
		return nil, err
	} else {

	}

	// FIX do not return nil on both return params
	return nil, nil
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
