package service_classes

import (
	"context"
	"errors"
	config "fesapp/backend"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/repository"
	"fesapp/backend/services"
	"fesapp/backend/types"
	"fesapp/backend/utils"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type syncJob struct {
	key    string
	name   string
	fn     func(context.Context) error
	result chan error
}

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
	UserRepository *repository.UserRepository

	queue           chan *syncJob
	queueCtx        context.Context
	queueCancel     context.CancelFunc
	queueWorkerOnce sync.Once

	lifecycleMu sync.RWMutex
	stopped     bool

	pendingMu  sync.Mutex
	pendingSet map[string]struct{}
}

func NewSyncService() *SyncService {
	backblaze := services.NewBackblazeSync()
	return &SyncService{
		Backblaze:        backblaze,
		FesClientService: NewFesClientService(),
		Logger:           utils.NewLoggerService(),
		TaskRepository:   repository.NewTaskRepository(),
		AuthRepository:   repository.NewAuthRepository(),
		UserRepository:   repository.NewUserRepository(),
		pendingSet:       make(map[string]struct{}),
	}
}

func (s *SyncService) Connect(ctx context.Context) error {
	if ctx != nil {
		s.ctx = ctx
	}
	s.startQueueWorker()
	return nil
}

func (s *SyncService) Shutdown() {
	s.lifecycleMu.Lock()
	if s.stopped {
		s.lifecycleMu.Unlock()
		return
	}
	s.stopped = true
	cancel := s.queueCancel
	s.lifecycleMu.Unlock()

	if cancel != nil {
		cancel()
	}
}

func (s *SyncService) KickoffAutoSyncOnLogin(attendance *models.Attendance) {
	if attendance == nil {
		return
	}

	go func() {
		if err := s.SyncAttendance(attendance); err != nil {
			s.Logger.Printf("auto login attendance sync failed: %v", err)
		}
		if err := s.SyncUnsyncedRecords(); err != nil {
			s.Logger.Printf("auto login unsynced records sync failed: %v", err)
		}
	}()
}

func (s *SyncService) SyncTasks() {
}

func (s *SyncService) SyncTaskRecord(taskRecordId uint) (err error) {
	job := &syncJob{
		key:  fmt.Sprintf("task-record:%d", taskRecordId),
		name: fmt.Sprintf("sync_task_record_%d", taskRecordId),
		fn: func(ctx context.Context) error {
			return s.syncTaskRecordNow(taskRecordId)
		},
	}

	err = s.enqueueAndWait(job)
	if err != nil {
		s.Logger.Println("Failed to sync task record:", err)
		s.emit("record-synced", false)
		return err
	}

	s.emit("record-synced", true)
	return nil
}

func (s *SyncService) syncTaskRecordNow(taskRecordId uint) error {
	if !config.OnlineMode {
		return fmt.Errorf("not currently synced")
	}

	taskRecord, err := s.TaskRepository.GetTaskRecord(taskRecordId)
	if err != nil {
		return err
	}

	attendance, err := s.AuthRepository.GetAttendanceFromID(taskRecord.AttendanceID)
	if err != nil {
		return err
	}

	user, err := s.UserRepository.GetUserFromId(taskRecord.UserID)
	if err != nil {
		return err
	}

	save := func() error {
		if err := db.DB.Save(taskRecord).Error; err != nil {
			s.Logger.Println("Failed to save user:", err)
			return err
		}
		return nil
	}

	saveStatus := func() error {
		now := time.Now()
		taskRecord.SyncedAt = &now
		taskRecord.SyncStatus = "synced"
		return save()
	}

	taskRecord.SyncStatus = "pending"
	if err := save(); err != nil {
		return err
	}

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
			return saveStatus()
		}
	}

	padagan := "No"
	if taskRecord.Padagan {
		padagan = "Yes"
	}

	task, err := s.TaskRepository.GetTaskFromTaskID(taskRecord.TaskID)
	if err != nil {
		s.Logger.Printlnf("Task not found on syncing task record, important for determining of upwork account. err: %v", err)
		return err
	}

	s.Logger.Printlnf("Syncing TASK IN")
	if resp, err := s.FesClientService.Request(types.Request{
		Url:    "wf/task_in",
		Method: "POST",
		Body: map[string]any{
			"task_in":              taskRecord.StartedAt.UnixMilli(),
			"date":                 attendance.TimeIn.UnixMilli(),
			"edited_by":            taskRecord.UserID,
			"employee_ID":          user.EmployeeId,
			"padaganan":            padagan,
			"projects_projects":    taskRecord.TaskID,
			"tracker_used":         task.Title,
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

		return saveStatus()
	}
}

func (s *SyncService) SyncAttendance(attendance *models.Attendance) (err error) {
	if attendance == nil {
		s.emit("attendance-synced", false)
		return fmt.Errorf("cannot sync nil attendance")
	}

	key := fmt.Sprintf("attendance:%d:%t", attendance.ID, attendance.TimeOut != nil)
	if attendance.TimeOut != nil {
		key = fmt.Sprintf("%s:%d", key, attendance.TimeOut.UnixMilli())
	}

	job := &syncJob{
		key:  key,
		name: fmt.Sprintf("sync_attendance_%d", attendance.ID),
		fn: func(ctx context.Context) error {
			return s.syncAttendanceNow(attendance)
		},
	}

	err = s.enqueueAndWait(job)
	if err != nil {
		s.Logger.Println("Failed to sync attendance:", err)
		s.emit("attendance-synced", false)
		return err
	}

	s.emit("attendance-synced", true)
	return nil
}

func (s *SyncService) syncAttendanceNow(attendance *models.Attendance) error {
	if attendance == nil {
		return fmt.Errorf("cannot sync nil attendance")
	}

	if !config.OnlineMode {
		return fmt.Errorf("cannot sync attendance while offline")
	}

	save := func() error {
		if err := db.DB.Save(attendance).Error; err != nil {
			s.Logger.Println("Failed to save user:", err)
			return err
		}
		return nil
	}

	saveStatus := func() error {
		now := time.Now()
		attendance.SyncedAt = &now
		attendance.SyncStatus = "synced"
		return save()
	}

	attendance.SyncStatus = "pending"
	if err := save(); err != nil {
		return err
	}

	if attendance.TimeOut != nil && attendance.LoginLogoutID != nil {
		email := ""
		if s.AuthUser != nil {
			email = s.AuthUser.Email
		}

		if email == "" {
			user, userErr := s.UserRepository.GetUserFromId(attendance.UserID)
			if userErr != nil {
				return fmt.Errorf("unable to resolve user email for logout sync: %w", userErr)
			}
			email = user.Email
		}

		if email == "" {
			return fmt.Errorf("unable to resolve user email for logout sync")
		}

		if resp, err := s.FesClientService.Request(types.Request{
			Url:    "wf/logout",
			Method: "POST",
			Body: map[string]any{
				"email":        email,
				"login_logout": *attendance.LoginLogoutID,
				"date_logout":  attendance.TimeOut.UnixMilli(),
				"duration":     int(attendance.TimeOut.Sub(attendance.TimeIn).Seconds()),
			},
		}); err != nil {
			return err
		} else {
			s.Logger.Printlnf("Sync Attendance Response: %v", resp)
			return saveStatus()
		}
	}

	body := map[string]any{
		"login": attendance.TimeIn.UnixMilli(),
	}

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
				milli := int64(timeInRaw)
				timeIn := time.UnixMilli(milli)
				s.Logger.Println("TIME IN: ", timeIn)
				attendance.TimeIn = timeIn
			}

			return saveStatus()
		}
	}

	return nil
}

func (s *SyncService) SyncActiveTaskRecord() (*models.TaskRecord, error) {
	if !config.OnlineMode && (*s.Attendance).LoginLogoutID == nil {
		s.emit("attendance-synced", false)
		return nil, s.Logger.Errorf("cannot get active task record if not synced")
	}

	body := map[string]any{
		"login_logout": *(*s.Attendance).LoginLogoutID,
	}

	if resp, err := s.FesClientService.Request(types.Request{
		Url:    "wf/active_task",
		Method: "GET",
		Body:   body,
	}); err != nil {
		s.Logger.Println("Attempt sync attendance error:", err)
		return nil, err
	} else {
		productivity, ok := resp["productivity"].(map[string]any)
		if !ok {
			return nil, s.Logger.Error("productivity not found on syncing active task record")
		}

		s.Logger.Printf("productivity returned on sync: %v", productivity)
	}

	return nil, nil
}

func (s *SyncService) SyncAllAttendance() error {
	return nil
}

func (s *SyncService) SyncUnsyncedRecords() error {
	if !config.OnlineMode {
		return fmt.Errorf("cannot sync records while offline")
	}

	if s.AuthUser == nil {
		return fmt.Errorf("cannot sync records without authenticated user")
	}

	job := &syncJob{
		key:  fmt.Sprintf("unsynced-records:%s", s.AuthUser.LocalID),
		name: fmt.Sprintf("sync_unsynced_records_%s", s.AuthUser.LocalID),
		fn: func(ctx context.Context) error {
			return s.syncUnsyncedRecordsNow()
		},
	}

	return s.enqueueAndWait(job)
}

func (s *SyncService) syncUnsyncedRecordsNow() error {
	var unsynced []models.TaskRecord
	if err := db.DB.
		Where("user_id = ? AND sync_status = ?", s.AuthUser.LocalID, "pending").
		Order("started_at ASC").
		Find(&unsynced).Error; err != nil {
		return err
	}

	var firstErr error
	for i := range unsynced {
		if err := s.syncTaskRecordNow(unsynced[i].ID); err != nil {
			s.emit("record-synced", false)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		s.emit("record-synced", true)
	}

	if s.Attendance != nil && *s.Attendance != nil {
		if err := s.syncAttendanceNow(*s.Attendance); err != nil {
			s.emit("attendance-synced", false)
			if firstErr == nil {
				firstErr = err
			}
		} else {
			s.emit("attendance-synced", true)
		}
	}

	return firstErr
}

func (s *SyncService) SyncUser(user models.AuthUser) error {
	return nil
}

func (s *SyncService) TestSync() bool {
	return true
}

func (s *SyncService) RefreshAccessToken() {
}

func (s *SyncService) startQueueWorker() {
	s.queueWorkerOnce.Do(func() {
		workerCtx := s.ctx
		if workerCtx == nil {
			workerCtx = context.Background()
		}

		s.queueCtx, s.queueCancel = context.WithCancel(workerCtx)
		s.queue = make(chan *syncJob, 256)
		go s.runQueueWorker()
	})
}

func (s *SyncService) runQueueWorker() {
	for {
		select {
		case <-s.queueCtx.Done():
			return
		case job := <-s.queue:
			if job == nil {
				continue
			}

			err := s.executeQueuedJob(job)
			if job.result != nil {
				job.result <- err
				close(job.result)
			}
		}
	}
}

func (s *SyncService) enqueueAndWait(job *syncJob) error {
	if job == nil || job.fn == nil {
		return fmt.Errorf("invalid sync job")
	}

	s.startQueueWorker()

	s.lifecycleMu.RLock()
	stopped := s.stopped
	s.lifecycleMu.RUnlock()
	if stopped {
		return fmt.Errorf("sync queue is shutting down")
	}

	claimed := false
	if job.key != "" {
		if !s.claimPending(job.key) {
			return nil
		}
		claimed = true
	}

	wrapped := *job
	wrapped.result = make(chan error, 1)
	wrapped.fn = func(ctx context.Context) error {
		if claimed {
			defer s.releasePending(job.key)
		}
		return job.fn(ctx)
	}

	select {
	case <-s.queueCtx.Done():
		if claimed {
			s.releasePending(job.key)
		}
		return fmt.Errorf("sync queue is unavailable")
	case s.queue <- &wrapped:
	}

	return <-wrapped.result
}

func (s *SyncService) executeQueuedJob(job *syncJob) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in sync job %s: %v", job.name, r)
			s.Logger.Println(err)
		}
	}()

	return s.executeWithRetry(job.name, job.fn)
}

func (s *SyncService) executeWithRetry(jobName string, fn func(context.Context) error) error {
	const maxAttempts = 4
	const baseDelay = 750 * time.Millisecond

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if s.queueCtx.Err() != nil {
			return s.queueCtx.Err()
		}

		err := fn(s.queueCtx)
		if err == nil {
			return nil
		}

		lastErr = err
		if !isRetryableSyncError(err) || attempt == maxAttempts {
			break
		}

		delay := baseDelay * time.Duration(1<<(attempt-1))
		if delay > 10*time.Second {
			delay = 10 * time.Second
		}

		s.Logger.Printf("Retrying sync job %s (%d/%d): %v", jobName, attempt, maxAttempts, err)

		select {
		case <-s.queueCtx.Done():
			return s.queueCtx.Err()
		case <-time.After(delay):
		}
	}

	return fmt.Errorf("%s failed: %w", jobName, lastErr)
}

func isRetryableSyncError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return true
	}

	msg := strings.ToLower(err.Error())
	retryableTokens := []string{
		"timeout",
		"temporary",
		"temporarily unavailable",
		"connection reset",
		"connection refused",
		"no such host",
		"tls handshake timeout",
		"unexpected eof",
		"broken pipe",
		"bad gateway",
		"service unavailable",
		"too many requests",
		" 429",
		" 500",
		" 502",
		" 503",
		" 504",
	}

	for _, token := range retryableTokens {
		if strings.Contains(msg, token) {
			return true
		}
	}

	return false
}

func (s *SyncService) emit(eventName string, payload any) {
	if s.ctx == nil {
		return
	}
	runtime.EventsEmit(s.ctx, eventName, payload)
}

func (s *SyncService) claimPending(key string) bool {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	if _, exists := s.pendingSet[key]; exists {
		return false
	}

	s.pendingSet[key] = struct{}{}
	return true
}

func (s *SyncService) releasePending(key string) {
	s.pendingMu.Lock()
	delete(s.pendingSet, key)
	s.pendingMu.Unlock()
}
