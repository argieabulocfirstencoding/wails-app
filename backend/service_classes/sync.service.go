package service_classes

import (
	"context"
	"encoding/json"
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
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

const (
	syncStatePending          = "pending"
	syncStateInProgress       = "in_progress"
	syncStateSynced           = "synced"
	syncStateFailed           = "failed"
	syncStateBlocked          = "blocked"
	syncStateRemoteMissing    = "remote_missing"
	syncStateIdentityConflict = "identity_conflict"
	syncStateConflict         = "conflict"
)

var errSyncBlocked = errors.New("sync blocked")

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
	workerDone      chan struct{}

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
	if ctx == nil {
		return nil
	}

	s.ctx = ctx
	if s.Backblaze != nil {
		s.Backblaze.SetContext(ctx)
	}
	s.startQueueWorker()
	return nil
}

func (s *SyncService) SetAuthUser(authUser *models.AuthUser) {
	s.AuthUser = authUser
	if s.FesClientService != nil {
		if authUser == nil {
			s.FesClientService.ClearActiveBubbleSession(true)
		} else {
			s.FesClientService.SetActiveSessionUser(authUser.LocalID)
		}
	}
	if s.Backblaze != nil {
		s.Backblaze.SetAuthUser(authUser)
	}
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

	if done := s.workerDone; done != nil {
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()

		select {
		case <-done:
		case <-timer.C:
		}
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
	err = s.syncTaskRecord(taskRecordId, false)
	if err != nil {
		s.Logger.Warnf("op=task_sync_failed correlation_id=%s record_id=%d err=%v", correlationID, taskRecordId, err)
		s.Logger.Println("Failed to sync task record:", err)
		s.emit("record-synced", false)
		return err
	}

	s.Logger.Infof("op=task_sync_success correlation_id=%s record_id=%d", correlationID, taskRecordId)
	s.emit("record-synced", true)
	return nil
}

func (s *SyncService) ResyncTaskRecord(taskRecordId uint) (err error) {
	correlationID := utils.NewCorrelationID("task_resync")
	s.Logger.Infof("op=task_resync_start correlation_id=%s record_id=%d", correlationID, taskRecordId)
	err = s.syncTaskRecord(taskRecordId, true)
	if err != nil {
		s.Logger.Warnf("op=task_resync_failed correlation_id=%s record_id=%d err=%v", correlationID, taskRecordId, err)
		s.Logger.Println("Failed to resync task record:", err)
		s.emit("record-synced", false)
		return err
	}

	s.Logger.Infof("op=task_resync_success correlation_id=%s record_id=%d", correlationID, taskRecordId)
	s.emit("record-synced", true)
	return nil
}

func (s *SyncService) syncTaskRecord(taskRecordId uint, force bool) error {
	job := &syncJob{
		key:  fmt.Sprintf("task-record:%d", taskRecordId),
		name: fmt.Sprintf("sync_task_record_%d", taskRecordId),
		fn: func(ctx context.Context) error {
			return s.syncTaskRecordNow(ctx, taskRecordId, force)
		},
	}

	return s.enqueueAndWait(job)
}

func (s *SyncService) syncTaskRecordNow(ctx context.Context, taskRecordId uint, force bool) error {
	taskRecord, err := s.TaskRepository.GetTaskRecord(taskRecordId)
	if err != nil {
		return err
	}
	if err := s.ensureTaskRecordOwnership(taskRecord); err != nil {
		return err
	}
	if taskRecord.SyncStatus == syncStateIdentityConflict {
		return fmt.Errorf("%w: task session identity is unresolved; review required", errSyncBlocked)
	}

	if !s.Connectivity.IsOnline() {
		_ = s.updateTaskRecordSyncState(taskRecord, syncStateInProgress, nil)
		return s.updateTaskRecordSyncState(taskRecord, syncStateBlocked, fmt.Errorf("cannot sync task record while offline"))
	}

	remoteMissing := taskRecordNeedsRemoteRepair(taskRecord)
	if remoteMissing {
		return fmt.Errorf("%w: task record is missing in Bubble; resolve the original session before resyncing", errSyncBlocked)
	}

	if !remoteMissing && taskRecord.EndedAt == nil && taskRecordHasRemoteProductivity(taskRecord) {
		if err := s.updateTaskRecordSyncState(taskRecord, syncStateSynced, nil); err != nil {
			return err
		}
		s.drainPendingScreenshotsBestEffort(taskRecord)
		return nil
	}

	if shouldSkipCompletedTaskRecordSync(taskRecord, force) {
		s.drainPendingScreenshotsBestEffort(taskRecord)
		return nil
	}
	if !taskRecordHasRemoteProductivity(taskRecord) {
		if err := prepareTaskSessionIdentity(taskRecord); err != nil {
			_ = s.updateTaskRecordSyncState(taskRecord, syncStateBlocked, err)
			return err
		}
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

	if taskRecord.EndedAt != nil {
		if !taskRecordHasRemoteProductivity(taskRecord) || remoteMissing {
			if err := s.syncTaskIn(taskRecord, attendance, user); err != nil {
				_ = s.updateTaskRecordSyncState(taskRecord, syncFailureState(remoteMissing), err)
				return err
			}
			taskRecord, err = s.TaskRepository.GetTaskRecord(taskRecordId)
			if err != nil {
				return err
			}
		}

		if err := s.syncTaskOut(taskRecord); err != nil {
			// A repaired task-in has already produced a fresh Bubble record. A
			// task-out failure is therefore a normal retryable sync failure, not a
			// remote-missing record.
			_ = s.updateTaskRecordSyncState(taskRecord, syncStateFailed, err)
			return err
		}
		taskRecord, err = s.TaskRepository.GetTaskRecord(taskRecordId)
		if err != nil {
			return err
		}
		s.drainPendingScreenshotsBestEffort(taskRecord)
		return nil
	}

	if err := s.syncTaskIn(taskRecord, attendance, user); err != nil {
		_ = s.updateTaskRecordSyncState(taskRecord, syncFailureState(remoteMissing), err)
		return err
	}

	taskRecord, err = s.TaskRepository.GetTaskRecord(taskRecordId)
	if err != nil {
		return err
	}
	s.drainPendingScreenshotsBestEffort(taskRecord)
	return nil
}

func (s *SyncService) syncTaskIn(taskRecord *models.TaskRecord, attendance *models.Attendance, user *models.User) error {
	if user == nil || strings.TrimSpace(user.EmployeeId) == "" {
		return fmt.Errorf("cannot sync task-in without an employee ID")
	}
	if taskRecord.ClientSessionID == nil || strings.TrimSpace(*taskRecord.ClientSessionID) == "" {
		return fmt.Errorf("%w: task start has no persisted session identity", errSyncBlocked)
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

	if err := recordTaskInDispatch(taskRecord); err != nil {
		return err
	}
	body := map[string]any{
		"task_in":              taskRecord.StartedAt.UnixMilli(),
		"date":                 attendance.TimeIn.UnixMilli(),
		"edited_by":            taskRecord.UserID,
		"employee_ID":          strings.TrimSpace(user.EmployeeId),
		"padaganan":            padagan,
		"projects_projects":    taskRecord.TaskID,
		"tracker_used":         task.Title,
		"task_time":            taskRecord.Duration,
		"login_logout":         attendance.LoginLogoutID,
		"activity_description": taskRecord.Notes,
	}
	if taskRecord.TaskInIdempotent {
		body["client_session_id"] = *taskRecord.ClientSessionID
	}
	s.Logger.Printlnf("Syncing TASK IN record_id=%d session_id=%s", taskRecord.ID, *taskRecord.ClientSessionID)
	if resp, err := s.FesClientService.Request(types.Request{
		Url:    "wf/task_in",
		Method: "POST",
		Body:   body,
	}); err != nil {
		return err
	} else {
		s.Logger.Printlnf("Sync Task Record Response: %v", resp)
		s.Logger.Printlnf("PRODUCTIVITY ID: %v", resp["productivity"])

		productivity, ok := resp["productivity"].(string)
		if !ok || strings.TrimSpace(productivity) == "" {
			return fmt.Errorf("task-in response missing productivity identifier")
		}
		if err := persistTaskProductivity(taskRecord, productivity); err != nil {
			return err
		}

		if err := s.reconcileTaskRecord(taskRecord, attendance); err != nil {
			return err
		}
		// Task-in does not acknowledge task-out. A crash before the subsequent
		// task-out must leave a completed record eligible for synchronization.
		if taskRecord.EndedAt != nil {
			return s.updateTaskRecordSyncState(taskRecord, syncStatePending, nil)
		}
		return s.updateTaskRecordSyncState(taskRecord, syncStateSynced, nil)
	}
}

func (s *SyncService) syncTaskOut(taskRecord *models.TaskRecord) error {
	if !taskRecordHasRemoteProductivity(taskRecord) {
		return fmt.Errorf("cannot sync task out without a remote productivity identifier")
	}

	s.Logger.Printlnf("Syncing TASK OUT")
	resp, err := s.FesClientService.Request(types.Request{
		Url:    "wf/task_out",
		Method: "POST",
		Body: map[string]any{
			"activity_description": taskRecord.Notes,
			"productivity":         taskRecord.Productivity,
			"total_task_time":      taskRecord.Duration,
			"task_out":             taskRecord.EndedAt.UnixMilli(),
		},
	})
	if err != nil {
		return err
	}

	s.Logger.Printlnf("Sync Task Record Response: %v", resp)
	return s.updateTaskRecordSyncState(taskRecord, syncStateSynced, nil)
}

func (s *SyncService) ensureAttendanceReadyForTaskSync(ctx context.Context, attendanceID uint) (*models.Attendance, error) {
	const waitInterval = 3 * time.Second
	const maxAttempts = 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
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

		if err := s.syncAttendanceNow(attendance); err != nil {
			s.Logger.Printf("task sync dependency attendance sync attempt failed. attendance_id=%d err=%v", attendance.ID, err)
		}

		if attempt == maxAttempts {
			return nil, fmt.Errorf("attendance dependency still unsatisfied for attendance_id=%d", attendance.ID)
		}

		if ctx == nil {
			time.Sleep(waitInterval)
			continue
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(waitInterval):
		}
	}

	return nil, fmt.Errorf("attendance dependency still unsatisfied for attendance_id=%d", attendanceID)
}

func attendanceReadyForTaskSync(attendance *models.Attendance) bool {
	if attendance == nil {
		return false
	}

	if attendance.LoginLogoutID != nil && strings.TrimSpace(*attendance.LoginLogoutID) != "" {
		return true
	}

	return strings.EqualFold(attendance.SyncStatus, "synced")
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
		s.Logger.Warnf("op=attendance_sync_failed correlation_id=%s attendance_id=%d err=%v", correlationID, attendance.ID, err)
		s.Logger.Println("Failed to sync attendance:", err)
		s.emit("attendance-synced", false)
		return err
	}

	s.Logger.Infof("op=attendance_sync_success correlation_id=%s attendance_id=%d", correlationID, attendance.ID)
	s.emit("attendance-synced", true)
	return nil
}

// ForceSyncAttendance deliberately re-sends a locally saved attendance record
// to Bubble, including records that are already marked synced. Callers must
// have already obtained explicit user confirmation for this operation.
func (s *SyncService) ForceSyncAttendance(attendance *models.Attendance) (err error) {
	if attendance == nil {
		return fmt.Errorf("cannot force sync nil attendance")
	}
	if s.AuthUser == nil || strings.TrimSpace(s.AuthUser.LocalID) == "" {
		return fmt.Errorf("cannot force sync attendance without authenticated user")
	}
	if attendance.UserID != s.AuthUser.LocalID {
		return fmt.Errorf("cannot force sync attendance owned by another user")
	}

	correlationID := utils.NewCorrelationID("attendance_force_sync")
	s.Logger.Infof("op=attendance_force_sync_start correlation_id=%s attendance_id=%d", correlationID, attendance.ID)

	key := fmt.Sprintf("attendance:%d:%t", attendance.ID, attendance.TimeOut != nil)
	if attendance.TimeOut != nil {
		key = fmt.Sprintf("%s:%d", key, attendance.TimeOut.UnixMilli())
	}

	job := &syncJob{
		key:  key,
		name: fmt.Sprintf("force_sync_attendance_%d", attendance.ID),
		fn: func(ctx context.Context) error {
			return s.syncAttendanceNowWithForce(attendance, true)
		},
	}

	err = s.enqueueAndWait(job)
	if err != nil {
		s.Logger.Warnf("op=attendance_force_sync_failed correlation_id=%s attendance_id=%d err=%v", correlationID, attendance.ID, err)
		s.emit("attendance-synced", false)
		return err
	}

	s.Logger.Infof("op=attendance_force_sync_success correlation_id=%s attendance_id=%d", correlationID, attendance.ID)
	s.emit("attendance-synced", true)
	return nil
}

func (s *SyncService) syncAttendanceNow(attendance *models.Attendance) error {
	return s.syncAttendanceNowWithForce(attendance, false)
}

func (s *SyncService) syncAttendanceNowWithForce(attendance *models.Attendance, force bool) error {
	if attendance == nil {
		return fmt.Errorf("cannot sync nil attendance")
	}

	if !s.Connectivity.IsOnline() {
		_ = s.updateAttendanceSyncState(attendance, syncStateInProgress, nil)
		return s.updateAttendanceSyncState(attendance, syncStateBlocked, fmt.Errorf("cannot sync attendance while offline"))
	}

	if !force && attendance.TimeOut != nil && attendanceLogoutAlreadySynced(attendance) {
		return nil
	}

	if !force && attendance.TimeOut == nil && attendanceLoginAlreadySynced(attendance) {
		return nil
	}

	if err := s.updateAttendanceSyncState(attendance, syncStateInProgress, nil); err != nil {
		return err
	}

	if attendance.TimeOut != nil {
		if !attendanceHasRemoteLoginLogout(attendance) {
			if err := s.syncAttendanceLogin(attendance); err != nil {
				return err
			}
		}

		return s.syncAttendanceLogout(attendance)
	}

	return s.syncAttendanceLogin(attendance)
}

func (s *SyncService) syncAttendanceLogout(attendance *models.Attendance) error {
	if attendance == nil || attendance.TimeOut == nil {
		return fmt.Errorf("cannot sync logout without a completed attendance")
	}
	if !attendanceHasRemoteLoginLogout(attendance) {
		return fmt.Errorf("cannot sync logout without login_logout")
	}

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
		s.Logger.Printlnf("Sync Attendance Response: %v", resp)
		return s.updateAttendanceSyncState(attendance, syncStateSynced, nil)
	}
}

func (s *SyncService) syncAttendanceLogin(attendance *models.Attendance) error {
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
	var unsynced []models.TaskRecord
	if err := db.DB.
		Where("user_id = ? AND sync_status IN ?", s.AuthUser.LocalID, []string{syncStatePending, syncStateFailed, syncStateBlocked}).
		Order("started_at ASC").
		Find(&unsynced).Error; err != nil {
		return err
	}

	var firstErr error
	handled := make(map[uint]struct{}, len(unsynced))
	for i := range unsynced {
		handled[unsynced[i].ID] = struct{}{}
		if err := s.syncTaskRecordNow(s.queueCtx, unsynced[i].ID, false); err != nil {
			s.emit("record-synced", false)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		s.emit("record-synced", true)
	}

	pendingScreenshotRecordIDs, err := s.pendingTaskRecordIDsWithPendingScreenshots()
	if err != nil && firstErr == nil {
		firstErr = err
	}

	for _, recordID := range pendingScreenshotRecordIDs {
		if _, exists := handled[recordID]; exists {
			continue
		}
		handled[recordID] = struct{}{}
		if err := s.syncTaskRecordNow(s.queueCtx, recordID, false); err != nil {
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
	s.lifecycleMu.RLock()
	stopped := s.stopped
	s.lifecycleMu.RUnlock()
	if stopped {
		return
	}
	if s.ctx == nil {
		return
	}

	s.queueWorkerOnce.Do(func() {
		s.queueCtx, s.queueCancel = context.WithCancel(s.ctx)
		s.queue = make(chan *syncJob, 256)
		s.workerDone = make(chan struct{})
		utils.GoSafe("sync.queue_worker", s.runQueueWorker)
	})
}

func (s *SyncService) runQueueWorker() {
	defer func() {
		if s.workerDone != nil {
			close(s.workerDone)
		}
	}()

	for {
		select {
		case <-s.queueCtx.Done():
			return
		case job := <-s.queue:
			if job == nil {
				continue
			}
			if s.queueCtx.Err() != nil {
				if job.result != nil {
					job.result <- s.queueCtx.Err()
					close(job.result)
				}
				return
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
	if s.queueCtx == nil || s.queue == nil {
		return fmt.Errorf("sync queue is unavailable")
	}

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
			return nil
		}
		claimed = true
		defer s.releasePending(job.key)
	}

	wrapped := *job
	wrapped.result = make(chan error, 1)
	wrapped.fn = job.fn

	select {
	case <-s.queueCtx.Done():
		if claimed {
			s.releasePending(job.key)
		}
		return fmt.Errorf("sync queue is unavailable")
	case s.queue <- &wrapped:
	}

	select {
	case result := <-wrapped.result:
		return result
	case <-s.queueCtx.Done():
		return s.queueCtx.Err()
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
	if errors.Is(err, errSyncBlocked) {
		return false
	}
	if errors.Is(err, errTaskStateChanged) {
		return true
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

	updates := map[string]any{
		"sync_status": state,
		"last_error":  taskRecord.LastError,
	}
	if state == syncStateInProgress {
		updates["last_attempt_at"] = now
		updates["attempt_count"] = gorm.Expr("attempt_count + 1")
	}
	if state == syncStateSynced {
		updates["synced_at"] = now
	}
	// Never save the stale lifecycle snapshot or associations. Also do not
	// acknowledge a newer stop/notes edit that this network request did not send.
	return db.DB.Transaction(func(tx *gorm.DB) error {
		var current models.TaskRecord
		if err := tx.First(&current, taskRecord.ID).Error; err != nil {
			return err
		}
		// Compare instants in Go: SQLite may store equivalent timestamps using
		// different timezone text, so SQL string equality is not sufficient.
		if current.UserID != taskRecord.UserID || current.AttendanceID != taskRecord.AttendanceID ||
			current.TaskID != taskRecord.TaskID || !current.StartedAt.Equal(taskRecord.StartedAt) ||
			current.Duration != taskRecord.Duration || current.Notes != taskRecord.Notes || current.Padagan != taskRecord.Padagan ||
			!sameOptionalTime(current.EndedAt, taskRecord.EndedAt) {
			return errTaskStateChanged
		}
		return tx.Model(&models.TaskRecord{}).Where("id = ?", taskRecord.ID).Updates(updates).Error
	})
}

func sameOptionalTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
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
	return nil
}

func (s *SyncService) reconcileTaskRecord(taskRecord *models.TaskRecord, attendance *models.Attendance) error {
	if taskRecord == nil {
		return fmt.Errorf("cannot reconcile nil task record")
	}

	if attendance == nil || !attendanceReadyForTaskSync(attendance) {
		return fmt.Errorf("cannot reconcile task record without synced attendance dependency")
	}

	// Task-in must always supply a remote productivity ID before local success is recorded.
	if taskRecord.Productivity == nil || strings.TrimSpace(*taskRecord.Productivity) == "" {
		return fmt.Errorf("remote reconciliation failed: missing productivity id")
	}

	return nil
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

func attendanceHasRemoteLoginLogout(attendance *models.Attendance) bool {
	if attendance == nil || attendance.LoginLogoutID == nil {
		return false
	}

	return strings.TrimSpace(*attendance.LoginLogoutID) != ""
}

func attendanceLoginAlreadySynced(attendance *models.Attendance) bool {
	return attendanceHasRemoteLoginLogout(attendance) && strings.EqualFold(attendance.SyncStatus, syncStateSynced)
}

func attendanceLogoutAlreadySynced(attendance *models.Attendance) bool {
	if attendance == nil || attendance.TimeOut == nil {
		return false
	}
	if !strings.EqualFold(attendance.SyncStatus, syncStateSynced) {
		return false
	}
	if attendance.SyncedAt == nil {
		return false
	}
	return !attendance.SyncedAt.Before(*attendance.TimeOut)
}

func taskRecordHasRemoteProductivity(taskRecord *models.TaskRecord) bool {
	if taskRecord == nil || taskRecord.Productivity == nil {
		return false
	}
	return strings.TrimSpace(*taskRecord.Productivity) != ""
}

func taskRecordNeedsRemoteRepair(taskRecord *models.TaskRecord) bool {
	return taskRecord != nil && strings.EqualFold(taskRecord.SyncStatus, syncStateRemoteMissing)
}

func syncFailureState(remoteMissing bool) string {
	if remoteMissing {
		return syncStateRemoteMissing
	}
	return syncStateFailed
}

func shouldSkipCompletedTaskRecordSync(taskRecord *models.TaskRecord, force bool) bool {
	if taskRecord == nil || taskRecord.EndedAt == nil {
		return false
	}

	if force {
		return false
	}

	return strings.EqualFold(taskRecord.SyncStatus, syncStateSynced)
}

func (s *SyncService) ensureTaskRecordOwnership(taskRecord *models.TaskRecord) error {
	if taskRecord == nil {
		return fmt.Errorf("task record is nil")
	}
	if s.AuthUser == nil || strings.TrimSpace(s.AuthUser.LocalID) == "" {
		return nil
	}
	if taskRecord.UserID != s.AuthUser.LocalID {
		return fmt.Errorf("cannot sync task record owned by another user")
	}
	return nil
}

func (s *SyncService) drainPendingScreenshotsBestEffort(taskRecord *models.TaskRecord) {
	if err := s.drainPendingScreenshotsNow(taskRecord); err != nil {
		recordID := uint(0)
		if taskRecord != nil {
			recordID = taskRecord.ID
		}
		s.Logger.Warnf("op=screenshot_backlog_drain_failed task_record_id=%d err=%v", recordID, err)
	}
}

func (s *SyncService) drainPendingScreenshotsNow(taskRecord *models.TaskRecord) error {
	if taskRecord == nil {
		return nil
	}
	if !taskRecordHasRemoteProductivity(taskRecord) {
		return nil
	}
	if s.Backblaze == nil || !s.Backblaze.Enabled() {
		return nil
	}
	if !s.Connectivity.IsOnline() {
		return nil
	}
	if err := s.ensureTaskRecordOwnership(taskRecord); err != nil {
		return err
	}

	folderName := sanitizeScreenshotFolderName(strings.TrimSpace(*taskRecord.Productivity))
	if folderName == "" {
		return fmt.Errorf("cannot upload screenshots without a valid productivity folder")
	}

	const maxPasses = 3
	for pass := 1; pass <= maxPasses; pass++ {
		var pending []models.Screenshot
		if err := db.DB.
			Where("task_record_id = ? AND (cloud_url IS NULL OR TRIM(cloud_url) = '')", taskRecord.ID).
			Order("taken_at ASC").
			Find(&pending).Error; err != nil {
			return err
		}

		if len(pending) == 0 {
			return nil
		}

		uploadedAny := false
		for i := range pending {
			paths := existingScreenshotPathsForUpload(pending[i].FilePath)
			if len(paths) == 0 {
				s.Logger.Warnf("op=screenshot_backlog_missing_local_files task_record_id=%d screenshot_id=%d", taskRecord.ID, pending[i].ID)
				continue
			}

			remotePaths, err := s.Backblaze.AttemptUploadToFolder(paths, folderName, pending[i].ID)
			if err != nil {
				s.Logger.Warnf("op=screenshot_backlog_upload_failed task_record_id=%d screenshot_id=%d err=%v", taskRecord.ID, pending[i].ID, err)
				continue
			}
			if len(remotePaths) == 0 {
				continue
			}

			jsonRemotePaths, err := json.Marshal(remotePaths)
			if err != nil {
				s.Logger.Warnf("op=screenshot_backlog_marshal_failed task_record_id=%d screenshot_id=%d err=%v", taskRecord.ID, pending[i].ID, err)
				continue
			}

			remoteString := string(jsonRemotePaths)
			if err := db.DB.Model(&pending[i]).Update("cloud_url", remoteString).Error; err != nil {
				s.Logger.Warnf("op=screenshot_backlog_persist_failed task_record_id=%d screenshot_id=%d err=%v", taskRecord.ID, pending[i].ID, err)
				continue
			}

			uploadedAny = true
		}

		if !uploadedAny {
			return nil
		}
	}

	return nil
}

func existingScreenshotPathsForUpload(raw string) []string {
	paths := decodeScreenshotPaths(raw)
	if len(paths) == 0 {
		return nil
	}

	existing := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			continue
		}
		existing = append(existing, path)
	}

	return existing
}

func (s *SyncService) pendingTaskRecordIDsWithPendingScreenshots() ([]uint, error) {
	if s.Backblaze == nil || !s.Backblaze.Enabled() {
		return nil, nil
	}
	if s.AuthUser == nil || strings.TrimSpace(s.AuthUser.LocalID) == "" {
		return nil, nil
	}

	var recordIDs []uint
	err := db.DB.
		Model(&models.Screenshot{}).
		Joins("JOIN task_records ON task_records.id = screenshots.task_record_id").
		Where("task_records.user_id = ? AND (screenshots.cloud_url IS NULL OR TRIM(screenshots.cloud_url) = '')", s.AuthUser.LocalID).
		Distinct("screenshots.task_record_id").
		Pluck("screenshots.task_record_id", &recordIDs).Error
	if err != nil {
		return nil, err
	}

	return recordIDs, nil
}
