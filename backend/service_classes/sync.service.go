package service_classes

import (
	"context"
	"errors"
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

const (
	syncStatePending    = "pending"
	syncStateInProgress = "in_progress"
	syncStateSynced     = "synced"
	syncStateFailed     = "failed"
	syncStateBlocked    = "blocked"
)

var errSyncBlocked = errors.New("sync blocked")
var errSyncAlreadyQueued = errors.New("sync already queued")

type syncJob struct {
	key    string
	name   string
	fn     func(context.Context) error
	result chan error
}

type SyncService struct {
	ctx              context.Context
	isConnected      bool
	Connectivity     *ConnectivityStateService
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

func NewSyncService(connectivity ...*ConnectivityStateService) *SyncService {
	backblaze := services.NewBackblazeSync()
	var connectivityState *ConnectivityStateService
	if len(connectivity) > 0 {
		connectivityState = connectivity[0]
	}
	if connectivityState == nil {
		connectivityState = NewConnectivityStateService()
	}
	return &SyncService{
		Backblaze:        backblaze,
		FesClientService: NewFesClientService(connectivityState),
		Logger:           utils.NewLoggerService(),
		TaskRepository:   repository.NewTaskRepository(),
		AuthRepository:   repository.NewAuthRepository(),
		UserRepository:   repository.NewUserRepository(),
		Connectivity:     connectivityState,
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
	correlationID := utils.NewCorrelationID("login_autosync")
	userID := ""
	if s.AuthUser != nil {
		userID = s.AuthUser.LocalID
	}
	s.Logger.Infof(
		"op=login_autosync_start correlation_id=%s attendance_id=%d user_id=%s online=%t",
		correlationID,
		attendance.ID,
		userID,
		s.Connectivity.IsOnline(),
	)

	utils.GoSafe("sync.auto_login_kickoff", func() {
		if err := s.SyncAttendance(attendance); err != nil {
			s.Logger.Warnf(
				"op=login_autosync_failed correlation_id=%s stage=attendance attendance_id=%d user_id=%s err=%v",
				correlationID,
				attendance.ID,
				userID,
				err,
			)
			return
		}
		if err := s.SyncUnsyncedRecords(); err != nil {
			s.Logger.Warnf(
				"op=login_autosync_failed correlation_id=%s stage=task_records attendance_id=%d user_id=%s err=%v",
				correlationID,
				attendance.ID,
				userID,
				err,
			)
			return
		}
		s.Logger.Infof(
			"op=login_autosync_success correlation_id=%s attendance_id=%d user_id=%s",
			correlationID,
			attendance.ID,
			userID,
		)
	})
}

func (s *SyncService) SyncTasks() {
}

func (s *SyncService) SyncTaskRecord(taskRecordId uint) (err error) {
	correlationID := utils.NewCorrelationID("task_sync")
	s.Logger.Infof("op=task_sync_start correlation_id=%s record_id=%d", correlationID, taskRecordId)

	taskRecord, lookupErr := s.TaskRepository.GetTaskRecord(taskRecordId)
	if lookupErr != nil {
		s.Logger.Warnf("op=task_sync_failed correlation_id=%s record_id=%d err=%v", correlationID, taskRecordId, lookupErr)
		s.emit("record-synced", false)
		return lookupErr
	}

	if depErr := s.enqueueAttendanceDependencySync(taskRecord.AttendanceID); depErr != nil {
		s.Logger.Warnf("op=task_sync_failed correlation_id=%s record_id=%d err=%v", correlationID, taskRecordId, depErr)
		s.emit("record-synced", false)
		return depErr
	}

	job := &syncJob{
		key:  fmt.Sprintf("task-record:%d", taskRecordId),
		name: fmt.Sprintf("sync_task_record_%d", taskRecordId),
		fn: func(ctx context.Context) error {
			return s.syncTaskRecordNow(ctx, taskRecordId)
		},
	}

	err = s.enqueueAndWait(job)
	if err != nil {
		if errors.Is(err, errSyncAlreadyQueued) {
			s.Logger.Infof("op=task_sync_queued correlation_id=%s record_id=%d", correlationID, taskRecordId)
			return err
		}
		s.Logger.Warnf("op=task_sync_failed correlation_id=%s record_id=%d err=%v", correlationID, taskRecordId, err)
		s.Logger.Println("Failed to sync task record:", err)
		s.emit("record-synced", false)
		return err
	}

	s.Logger.Infof("op=task_sync_success correlation_id=%s record_id=%d", correlationID, taskRecordId)
	s.emit("record-synced", true)
	return nil
}

func (s *SyncService) enqueueAttendanceDependencySync(attendanceID uint) error {
	attendance, err := s.AuthRepository.GetAttendanceFromID(attendanceID)
	if err != nil {
		return err
	}

	if attendanceReadyForTaskSync(attendance) {
		return nil
	}

	key := fmt.Sprintf("attendance:%d:%t", attendance.ID, attendance.TimeOut != nil)
	if attendance.TimeOut != nil {
		key = fmt.Sprintf("%s:%d", key, attendance.TimeOut.UnixMilli())
	}

	job := &syncJob{
		key:  key,
		name: fmt.Sprintf("sync_attendance_dependency_%d", attendance.ID),
		fn: func(ctx context.Context) error {
			return s.syncAttendanceNow(attendance)
		},
	}

	return s.enqueueAndWait(job)
}

func (s *SyncService) syncTaskRecordNow(ctx context.Context, taskRecordId uint) error {
	taskRecord, err := s.TaskRepository.GetTaskRecord(taskRecordId)
	if err != nil {
		return err
	}

	if !s.Connectivity.IsOnline() {
		_ = s.updateTaskRecordSyncState(taskRecord, syncStateInProgress, nil)
		return s.updateTaskRecordSyncState(taskRecord, syncStateBlocked, fmt.Errorf("cannot sync task record while offline"))
	}

	if err := s.updateTaskRecordSyncState(taskRecord, syncStateInProgress, nil); err != nil {
		return err
	}

	attendance, err := s.ensureAttendanceReadyForTaskSync(ctx, taskRecord.AttendanceID)
	if err != nil {
		_ = s.updateTaskRecordSyncState(taskRecord, syncStateBlocked, err)
		return fmt.Errorf("%w: attendance dependency unsatisfied", errSyncBlocked)
	}

	user, err := s.UserRepository.GetUserFromId(taskRecord.UserID)
	if err != nil {
		_ = s.updateTaskRecordSyncState(taskRecord, syncStateFailed, err)
		return err
	}

	if taskRecord.Productivity == nil || strings.TrimSpace(*taskRecord.Productivity) == "" {
		if err := s.syncTaskInNow(taskRecord, attendance, user); err != nil {
			_ = s.updateTaskRecordSyncState(taskRecord, syncStateFailed, err)
			return err
		}
	}

	if taskRecord.EndedAt != nil {
		if err := s.syncTaskOutNow(taskRecord); err != nil {
			_ = s.updateTaskRecordSyncState(taskRecord, syncStateFailed, err)
			return err
		}
		return s.updateTaskRecordSyncState(taskRecord, syncStateSynced, nil)
	}

	if err := s.reconcileTaskRecord(taskRecord, attendance); err != nil {
		_ = s.updateTaskRecordSyncState(taskRecord, syncStateFailed, err)
		return err
	}
	return s.updateTaskRecordSyncState(taskRecord, syncStateSynced, nil)
}

func (s *SyncService) syncTaskInNow(taskRecord *models.TaskRecord, attendance *models.Attendance, user *models.User) error {
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
	resp, err := s.FesClientService.Request(types.Request{
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
	})
	if err != nil {
		return err
	}
	if err := rejectFailedRemoteResponse(resp); err != nil {
		return err
	}

	s.Logger.Printlnf("Sync Task Record Response: %v", resp)
	s.Logger.Printlnf("PRODUCTIVITY ID: %v", resp["productivity"])

	productivity, ok := resp["productivity"].(string)
	if !ok || strings.TrimSpace(productivity) == "" {
		return fmt.Errorf("task_in response missing productivity")
	}

	taskRecord.Productivity = &productivity
	return db.DB.Model(taskRecord).Update("productivity", productivity).Error
}

func (s *SyncService) syncTaskOutNow(taskRecord *models.TaskRecord) error {
	if taskRecord.EndedAt == nil {
		return fmt.Errorf("cannot sync task_out without ended_at")
	}
	if taskRecord.Productivity == nil || strings.TrimSpace(*taskRecord.Productivity) == "" {
		return fmt.Errorf("cannot sync task_out without remote productivity id")
	}

	s.Logger.Printlnf("Syncing TASK OUT")

	resp, err := s.FesClientService.Request(types.Request{
		Url:    "wf/task_out",
		Method: "POST",
		Body: map[string]any{
			"activity_description": strings.ReplaceAll(taskRecord.Notes, "||", "-"),
			"productivity":         taskRecord.Productivity,
			"total_task_time":      taskRecord.Duration,
			"task_out":             taskRecord.EndedAt.UnixMilli(),
		},
	})
	if err != nil {
		return err
	}
	if err := rejectFailedRemoteResponse(resp); err != nil {
		return err
	}

	s.Logger.Printlnf("Sync Task Record Response: %v", resp)
	return nil
}

func (s *SyncService) ensureAttendanceReadyForTaskSync(ctx context.Context, attendanceID uint) (*models.Attendance, error) {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	attendance, err := s.AuthRepository.GetAttendanceFromID(attendanceID)
	if err != nil {
		return nil, err
	}

	if attendanceReadyForTaskSync(attendance) {
		return attendance, nil
	}

	s.Logger.Printf(
		"task sync waiting for attendance dependency. attendance_id=%d sync_status=%q has_login_logout_id=%t",
		attendance.ID,
		attendance.SyncStatus,
		attendance.LoginLogoutID != nil && strings.TrimSpace(*attendance.LoginLogoutID) != "",
	)

	return nil, fmt.Errorf("attendance dependency still unsatisfied for attendance_id=%d", attendance.ID)
}

func attendanceReadyForTaskSync(attendance *models.Attendance) bool {
	if attendance == nil {
		return false
	}

	// Hard dependency: attendance must be fully synced first before task sync.
	if !strings.EqualFold(attendance.SyncStatus, syncStateSynced) {
		return false
	}

	return attendance.LoginLogoutID != nil && strings.TrimSpace(*attendance.LoginLogoutID) != ""
}

func (s *SyncService) SyncAttendance(attendance *models.Attendance) (err error) {
	correlationID := utils.NewCorrelationID("attendance_sync")
	attendanceID := uint(0)
	if attendance != nil {
		attendanceID = attendance.ID
	}
	s.Logger.Infof("op=attendance_sync_start correlation_id=%s attendance_id=%d", correlationID, attendanceID)
	if attendance == nil {
		s.Logger.Warnf("op=attendance_sync_failed correlation_id=%s attendance_id=0 err=nil_attendance", correlationID)
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
		if errors.Is(err, errSyncAlreadyQueued) {
			s.Logger.Infof("op=attendance_sync_queued correlation_id=%s attendance_id=%d", correlationID, attendance.ID)
			return err
		}
		s.Logger.Warnf("op=attendance_sync_failed correlation_id=%s attendance_id=%d err=%v", correlationID, attendance.ID, err)
		s.Logger.Println("Failed to sync attendance:", err)
		s.emit("attendance-synced", false)
		return err
	}

	s.Logger.Infof("op=attendance_sync_success correlation_id=%s attendance_id=%d", correlationID, attendance.ID)
	s.emit("attendance-synced", true)
	return nil
}

func (s *SyncService) syncAttendanceNow(attendance *models.Attendance) error {
	if attendance == nil {
		return fmt.Errorf("cannot sync nil attendance")
	}

	if !s.Connectivity.IsOnline() {
		_ = s.updateAttendanceSyncState(attendance, syncStateInProgress, nil)
		return s.updateAttendanceSyncState(attendance, syncStateBlocked, fmt.Errorf("cannot sync attendance while offline"))
	}

	if err := s.updateAttendanceSyncState(attendance, syncStateInProgress, nil); err != nil {
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
			_ = s.updateAttendanceSyncState(attendance, syncStateFailed, err)
			return err
		} else {
			if err := rejectFailedRemoteResponse(resp); err != nil {
				_ = s.updateAttendanceSyncState(attendance, syncStateFailed, err)
				return err
			}
			s.Logger.Printlnf("Sync Attendance Response: %v", resp)
			return s.updateAttendanceSyncState(attendance, syncStateSynced, nil)
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
		_ = s.updateAttendanceSyncState(attendance, syncStateFailed, err)
		return err
	} else {
		if err := rejectFailedRemoteResponse(resp); err != nil {
			_ = s.updateAttendanceSyncState(attendance, syncStateFailed, err)
			return err
		}
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

			return s.updateAttendanceSyncState(attendance, syncStateSynced, nil)
		}
	}

	err := fmt.Errorf("attendance sync response missing login_logout")
	_ = s.updateAttendanceSyncState(attendance, syncStateFailed, err)
	return err
}

func (s *SyncService) SyncActiveTaskRecord() (*models.TaskRecord, error) {
	if s.Attendance == nil || *s.Attendance == nil {
		s.emit("attendance-synced", false)
		return nil, s.Logger.Errorf("cannot get active task record without attendance")
	}

	if !s.Connectivity.IsOnline() || (*s.Attendance).LoginLogoutID == nil {
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
	correlationID := utils.NewCorrelationID("task_sync_batch")
	s.Logger.Infof("op=task_sync_batch_start correlation_id=%s", correlationID)
	if !s.Connectivity.IsOnline() {
		s.Logger.Warnf("op=task_sync_batch_failed correlation_id=%s err=offline", correlationID)
		return fmt.Errorf("cannot sync records while offline")
	}

	if s.AuthUser == nil {
		s.Logger.Warnf("op=task_sync_batch_failed correlation_id=%s err=auth_user_missing", correlationID)
		return fmt.Errorf("cannot sync records without authenticated user")
	}

	job := &syncJob{
		key:  fmt.Sprintf("unsynced-records:%s", s.AuthUser.LocalID),
		name: fmt.Sprintf("sync_unsynced_records_%s", s.AuthUser.LocalID),
		fn: func(ctx context.Context) error {
			return s.syncUnsyncedRecordsNow()
		},
	}

	err := s.enqueueAndWait(job)
	if err != nil {
		s.Logger.Warnf("op=task_sync_batch_failed correlation_id=%s user_id=%s err=%v", correlationID, s.AuthUser.LocalID, err)
		return err
	}
	s.Logger.Infof("op=task_sync_batch_success correlation_id=%s user_id=%s", correlationID, s.AuthUser.LocalID)
	return nil
}

func (s *SyncService) syncUnsyncedRecordsNow() error {
	syncableStates := []string{syncStatePending, syncStateFailed, syncStateBlocked, syncStateInProgress}

	var unsyncedAttendances []models.Attendance
	if err := db.DB.
		Where("user_id = ? AND sync_status IN ?", s.AuthUser.LocalID, syncableStates).
		Order("time_in ASC").
		Find(&unsyncedAttendances).Error; err != nil {
		return err
	}

	var firstErr error
	for i := range unsyncedAttendances {
		if err := s.syncAttendanceNow(&unsyncedAttendances[i]); err != nil {
			s.emit("attendance-synced", false)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		s.emit("attendance-synced", true)
	}

	var unsynced []models.TaskRecord
	if err := db.DB.
		Where("user_id = ? AND sync_status IN ?", s.AuthUser.LocalID, syncableStates).
		Order("started_at ASC").
		Find(&unsynced).Error; err != nil {
		return err
	}

	for i := range unsynced {
		if err := s.syncTaskRecordNow(s.queueCtx, unsynced[i].ID); err != nil {
			s.emit("record-synced", false)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		s.emit("record-synced", true)
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
		utils.GoSafe("sync.queue_worker", s.runQueueWorker)
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
			s.Logger.Printf("Skipping duplicate sync job key=%s", job.key)
			return errSyncAlreadyQueued
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

	select {
	case err := <-wrapped.result:
		return err
	case <-s.queueCtx.Done():
		if claimed {
			s.releasePending(job.key)
		}
		return fmt.Errorf("sync queue is unavailable: %w", s.queueCtx.Err())
	}
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
	if errors.Is(err, errSyncBlocked) {
		return false
	}
	if errors.Is(err, errSyncAlreadyQueued) {
		return false
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

func (s *SyncService) updateTaskRecordSyncState(taskRecord *models.TaskRecord, state string, opErr error) error {
	if taskRecord == nil {
		return fmt.Errorf("task record is nil")
	}

	now := time.Now()
	taskRecord.SyncStatus = state
	if state == syncStateInProgress {
		taskRecord.LastAttemptAt = &now
		taskRecord.AttemptCount++
	}
	if opErr != nil {
		taskRecord.LastError = opErr.Error()
	} else {
		taskRecord.LastError = ""
	}
	if state == syncStateSynced {
		taskRecord.SyncedAt = &now
		taskRecord.LastError = ""
	}

	if err := db.DB.Save(taskRecord).Error; err != nil {
		return err
	}
	s.Logger.Infof("op=task_sync_state_saved user_id=%s attendance_id=%d task_id=%s record_id=%d sync_status=%s attempt_count=%d last_error=%q", taskRecord.UserID, taskRecord.AttendanceID, taskRecord.TaskID, taskRecord.ID, taskRecord.SyncStatus, taskRecord.AttemptCount, taskRecord.LastError)
	return nil
}

func (s *SyncService) updateAttendanceSyncState(attendance *models.Attendance, state string, opErr error) error {
	if attendance == nil {
		return fmt.Errorf("attendance is nil")
	}

	now := time.Now()
	attendance.SyncStatus = state
	if state == syncStateInProgress {
		attendance.LastAttemptAt = &now
		attendance.AttemptCount++
	}
	if opErr != nil {
		attendance.LastError = opErr.Error()
	} else {
		attendance.LastError = ""
	}
	if state == syncStateSynced {
		attendance.SyncedAt = &now
		attendance.LastError = ""
	}

	if err := db.DB.Save(attendance).Error; err != nil {
		return err
	}
	s.Logger.Infof("op=attendance_sync_state_saved user_id=%s attendance_id=%d sync_status=%s attempt_count=%d last_error=%q", attendance.UserID, attendance.ID, attendance.SyncStatus, attendance.AttemptCount, attendance.LastError)
	return nil
}

func (s *SyncService) reconcileTaskRecord(taskRecord *models.TaskRecord, attendance *models.Attendance) error {
	if taskRecord == nil {
		return fmt.Errorf("cannot reconcile nil task record")
	}

	if attendance == nil || !attendanceReadyForTaskSync(attendance) {
		return fmt.Errorf("cannot reconcile task record without synced attendance dependency")
	}

	// For task-in, require a remote productivity ID before marking record as synced.
	if taskRecord.EndedAt == nil {
		if taskRecord.Productivity == nil || strings.TrimSpace(*taskRecord.Productivity) == "" {
			return fmt.Errorf("remote reconciliation failed: missing productivity id")
		}
	}

	return nil
}

func rejectFailedRemoteResponse(resp map[string]any) error {
	if resp == nil {
		return fmt.Errorf("empty remote response")
	}

	raw, ok := resp["error"]
	if !ok {
		return nil
	}

	message, ok := raw.(string)
	if !ok {
		return fmt.Errorf("remote response error: %v", raw)
	}

	trimmed := strings.TrimSpace(message)
	if trimmed == "" || strings.EqualFold(trimmed, "Successful") {
		return nil
	}

	return fmt.Errorf("remote response error: %s", trimmed)
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
