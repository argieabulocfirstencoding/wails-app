package service_classes

import (
	"context"
	"encoding/json"
	"errors"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/repository"
	"fesapp/backend/services"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	sqlite "github.com/Tryanks/gorm-sqlite"
	"gorm.io/gorm"
)

func setupSyncServiceTestDB(t *testing.T) {
	t.Helper()

	originalDB := db.DB
	database, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "sync-test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := database.AutoMigrate(
		&models.User{},
		&models.Client{},
		&models.Task{},
		&models.Attendance{},
		&models.TaskRecord{},
		&models.Screenshot{},
	); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	db.DB = database
	t.Cleanup(func() {
		if sqlDB, sqlErr := database.DB(); sqlErr == nil {
			_ = sqlDB.Close()
		}
		db.DB = originalDB
	})
}

func newTestSyncService(online bool) *SyncService {
	connectivity := NewConnectivityStateService()
	connectivity.SetOnline(online)

	service := NewSyncService(connectivity)
	service.AuthUser = &models.AuthUser{
		LocalID:    "user-1",
		Email:      "user@example.com",
		EmployeeId: "employee-1",
	}
	service.FesClientService.apiToken = "test-token"
	return service
}

func seedSyncUserTaskAttendance(t *testing.T, attendanceStatus string, loginLogoutID *string) (*models.User, *models.Attendance, *models.Task) {
	t.Helper()

	user := &models.User{
		ID:         "user-1",
		Email:      "user@example.com",
		EmployeeId: "employee-1",
	}
	if err := db.DB.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	client := &models.Client{
		ID:          "client-1",
		Fullname:    "Client One",
		CompanyName: "Client Company",
	}
	if err := db.DB.Create(client).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}

	task := &models.Task{
		TaskID:      "task-1",
		Title:       "Tracker Task",
		ProjectName: "Project One",
		ClientID:    client.ID,
	}
	if err := db.DB.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	attendance := &models.Attendance{
		UserID:        user.ID,
		TimeIn:        time.Now().Add(-2 * time.Hour),
		LoginLogoutID: loginLogoutID,
		SyncStatus:    attendanceStatus,
	}
	if err := db.DB.Create(attendance).Error; err != nil {
		t.Fatalf("create attendance: %v", err)
	}

	return user, attendance, task
}

func seedTaskRecord(t *testing.T, user *models.User, attendance *models.Attendance, task *models.Task, ended bool, productivity *string) *models.TaskRecord {
	t.Helper()

	startedAt := attendance.TimeIn.Add(15 * time.Minute)
	record := &models.TaskRecord{
		UserID:       user.ID,
		AttendanceID: attendance.ID,
		TaskID:       task.TaskID,
		StartedAt:    startedAt,
		Duration:     1800,
		Notes:        "test notes",
		Productivity: productivity,
		SyncStatus:   syncStatePending,
	}
	if ended {
		endedAt := startedAt.Add(30 * time.Minute)
		record.EndedAt = &endedAt
	}

	if err := db.DB.Create(record).Error; err != nil {
		t.Fatalf("create task record: %v", err)
	}

	return record
}

func attachFesTestServer(t *testing.T, service *SyncService, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(handler)
	service.FesClientService.baseUrl = server.URL + "/"
	service.FesClientService.client = server.Client()
	service.FesClientService.apiToken = "test-token"
	t.Cleanup(server.Close)
	return server
}

func writeFesResponse(t *testing.T, writer http.ResponseWriter, status int, response map[string]any) {
	t.Helper()

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(map[string]any{"response": response}); err != nil {
		t.Fatalf("write fes response: %v", err)
	}
}

func TestEnqueueAndWaitDuplicateJobDoesNotReportSuccess(t *testing.T) {
	service := newTestSyncService(true)
	if err := service.Connect(context.Background()); err != nil {
		t.Fatalf("connect sync service: %v", err)
	}
	defer service.Shutdown()

	started := make(chan struct{})
	release := make(chan struct{})
	firstResult := make(chan error, 1)

	go func() {
		firstResult <- service.enqueueAndWait(&syncJob{
			key:  "duplicate-key",
			name: "first_duplicate_test_job",
			fn: func(ctx context.Context) error {
				close(started)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-release:
					return nil
				}
			},
		})
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first sync job did not start")
	}

	err := service.enqueueAndWait(&syncJob{
		key:  "duplicate-key",
		name: "second_duplicate_test_job",
		fn: func(ctx context.Context) error {
			t.Fatal("duplicate sync job should not execute")
			return nil
		},
	})
	if !errors.Is(err, errSyncAlreadyQueued) {
		t.Fatalf("expected errSyncAlreadyQueued, got %v", err)
	}

	close(release)
	if err := <-firstResult; err != nil {
		t.Fatalf("first job failed: %v", err)
	}

	if err := service.enqueueAndWait(&syncJob{
		key:  "duplicate-key",
		name: "third_duplicate_test_job",
		fn: func(ctx context.Context) error {
			return nil
		},
	}); err != nil {
		t.Fatalf("expected key to be reusable after completion, got %v", err)
	}
}

func TestSyncTaskRecordOfflineBlocksButKeepsLocalRecord(t *testing.T) {
	setupSyncServiceTestDB(t)

	service := newTestSyncService(false)
	loginLogoutID := "remote-login-1"
	user, attendance, task := seedSyncUserTaskAttendance(t, syncStateSynced, &loginLogoutID)
	record := seedTaskRecord(t, user, attendance, task, true, nil)

	err := service.syncTaskRecordNow(context.Background(), record.ID)
	if err == nil {
		t.Fatal("expected offline sync to fail")
	}

	var saved models.TaskRecord
	if dbErr := db.DB.First(&saved, record.ID).Error; dbErr != nil {
		t.Fatalf("expected local task record to remain saved: %v", dbErr)
	}
	if saved.SyncStatus != syncStateBlocked {
		t.Fatalf("expected sync status %q, got %q", syncStateBlocked, saved.SyncStatus)
	}
	if saved.EndedAt == nil {
		t.Fatal("expected ended_at to remain saved locally")
	}
	if saved.AttemptCount != 1 {
		t.Fatalf("expected one local sync attempt, got %d", saved.AttemptCount)
	}
	if !strings.Contains(saved.LastError, "offline") {
		t.Fatalf("expected offline last_error, got %q", saved.LastError)
	}
}

func TestSyncAttendanceOfflineBlocksButKeepsLocalTimestamp(t *testing.T) {
	setupSyncServiceTestDB(t)

	service := newTestSyncService(false)
	_, attendance, _ := seedSyncUserTaskAttendance(t, syncStatePending, nil)
	originalTimeIn := attendance.TimeIn

	err := service.syncAttendanceNow(attendance)
	if err == nil {
		t.Fatal("expected offline attendance sync to fail")
	}

	var saved models.Attendance
	if dbErr := db.DB.First(&saved, attendance.ID).Error; dbErr != nil {
		t.Fatalf("expected local attendance to remain saved: %v", dbErr)
	}
	if saved.SyncStatus != syncStateBlocked {
		t.Fatalf("expected sync status %q, got %q", syncStateBlocked, saved.SyncStatus)
	}
	if !saved.TimeIn.Equal(originalTimeIn) {
		t.Fatalf("expected local time_in to remain %s, got %s", originalTimeIn, saved.TimeIn)
	}
	if saved.AttemptCount != 1 {
		t.Fatalf("expected one local sync attempt, got %d", saved.AttemptCount)
	}
	if !strings.Contains(saved.LastError, "offline") {
		t.Fatalf("expected offline last_error, got %q", saved.LastError)
	}
}

func TestLogoutAttendancePreservesOriginalTimeOutOnRetry(t *testing.T) {
	setupSyncServiceTestDB(t)

	user, attendance, _ := seedSyncUserTaskAttendance(t, syncStatePending, nil)
	firstTimeOut := time.Now().Add(-45 * time.Minute).Truncate(time.Millisecond)
	attendance.TimeOut = &firstTimeOut
	if err := db.DB.Save(attendance).Error; err != nil {
		t.Fatalf("save timed-out attendance: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	if err := services.LogoutAttendance(attendance); err != nil {
		t.Fatalf("service logout retry: %v", err)
	}

	var saved models.Attendance
	if err := db.DB.First(&saved, attendance.ID).Error; err != nil {
		t.Fatalf("load service logout attendance: %v", err)
	}
	if !saved.TimeOut.Equal(firstTimeOut) {
		t.Fatalf("service logout retry changed time_out from %s to %s", firstTimeOut, *saved.TimeOut)
	}

	attendanceRepo := repository.NewAttendanceRepository()
	repoAttendance := &models.Attendance{
		UserID:     user.ID,
		TimeIn:     time.Now().Add(-2 * time.Hour),
		TimeOut:    &firstTimeOut,
		SyncStatus: syncStatePending,
	}
	if err := db.DB.Create(repoAttendance).Error; err != nil {
		t.Fatalf("create repo attendance: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	if err := attendanceRepo.LogoutAttendance(repoAttendance); err != nil {
		t.Fatalf("repository logout retry: %v", err)
	}

	var repoSaved models.Attendance
	if err := db.DB.First(&repoSaved, repoAttendance.ID).Error; err != nil {
		t.Fatalf("load repository logout attendance: %v", err)
	}
	if !repoSaved.TimeOut.Equal(firstTimeOut) {
		t.Fatalf("repository logout retry changed time_out from %s to %s", firstTimeOut, *repoSaved.TimeOut)
	}
}

func TestSyncTaskRecordEndedWithoutProductivitySyncsInBeforeOut(t *testing.T) {
	setupSyncServiceTestDB(t)

	service := newTestSyncService(true)
	loginLogoutID := "remote-login-1"
	user, attendance, task := seedSyncUserTaskAttendance(t, syncStateSynced, &loginLogoutID)
	record := seedTaskRecord(t, user, attendance, task, true, nil)

	var mu sync.Mutex
	var calls []string
	attachFesTestServer(t, service, func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		calls = append(calls, request.URL.Path)
		mu.Unlock()

		switch request.URL.Path {
		case "/wf/task_in":
			writeFesResponse(t, writer, http.StatusOK, map[string]any{"productivity": "prod-1"})
		case "/wf/task_out":
			writeFesResponse(t, writer, http.StatusOK, map[string]any{"error": "Successful"})
		default:
			t.Fatalf("unexpected FES path: %s", request.URL.Path)
		}
	})

	if err := service.syncTaskRecordNow(context.Background(), record.ID); err != nil {
		t.Fatalf("sync task record: %v", err)
	}

	mu.Lock()
	gotCalls := append([]string(nil), calls...)
	mu.Unlock()
	expectedCalls := []string{"/wf/task_in", "/wf/task_out"}
	if len(gotCalls) != len(expectedCalls) {
		t.Fatalf("expected calls %v, got %v", expectedCalls, gotCalls)
	}
	for i := range expectedCalls {
		if gotCalls[i] != expectedCalls[i] {
			t.Fatalf("expected calls %v, got %v", expectedCalls, gotCalls)
		}
	}

	var saved models.TaskRecord
	if err := db.DB.First(&saved, record.ID).Error; err != nil {
		t.Fatalf("load saved task record: %v", err)
	}
	if saved.SyncStatus != syncStateSynced {
		t.Fatalf("expected task record synced, got %q", saved.SyncStatus)
	}
	if saved.Productivity == nil || *saved.Productivity != "prod-1" {
		t.Fatalf("expected productivity prod-1, got %v", saved.Productivity)
	}
	if saved.SyncedAt == nil {
		t.Fatal("expected synced_at to be set")
	}
}

func TestSyncUnsyncedRecordsSyncsAttendanceBeforeTasks(t *testing.T) {
	setupSyncServiceTestDB(t)

	service := newTestSyncService(true)
	user, attendance, task := seedSyncUserTaskAttendance(t, syncStatePending, nil)
	record := seedTaskRecord(t, user, attendance, task, false, nil)

	var mu sync.Mutex
	var calls []string
	attachFesTestServer(t, service, func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		calls = append(calls, request.URL.Path)
		mu.Unlock()

		switch request.URL.Path {
		case "/wf/login_attendance":
			writeFesResponse(t, writer, http.StatusOK, map[string]any{
				"login_logout": "remote-login-1",
				"login":        float64(attendance.TimeIn.UnixMilli()),
			})
		case "/wf/task_in":
			writeFesResponse(t, writer, http.StatusOK, map[string]any{"productivity": "prod-1"})
		default:
			t.Fatalf("unexpected FES path: %s", request.URL.Path)
		}
	})

	if err := service.syncUnsyncedRecordsNow(); err != nil {
		t.Fatalf("sync unsynced records: %v", err)
	}

	mu.Lock()
	gotCalls := append([]string(nil), calls...)
	mu.Unlock()
	expectedCalls := []string{"/wf/login_attendance", "/wf/task_in"}
	if len(gotCalls) != len(expectedCalls) {
		t.Fatalf("expected calls %v, got %v", expectedCalls, gotCalls)
	}
	for i := range expectedCalls {
		if gotCalls[i] != expectedCalls[i] {
			t.Fatalf("expected calls %v, got %v", expectedCalls, gotCalls)
		}
	}

	var savedAttendance models.Attendance
	if err := db.DB.First(&savedAttendance, attendance.ID).Error; err != nil {
		t.Fatalf("load saved attendance: %v", err)
	}
	if savedAttendance.SyncStatus != syncStateSynced {
		t.Fatalf("expected attendance synced, got %q", savedAttendance.SyncStatus)
	}
	if savedAttendance.LoginLogoutID == nil || *savedAttendance.LoginLogoutID != "remote-login-1" {
		t.Fatalf("expected login_logout id saved, got %v", savedAttendance.LoginLogoutID)
	}

	var savedRecord models.TaskRecord
	if err := db.DB.First(&savedRecord, record.ID).Error; err != nil {
		t.Fatalf("load saved task record: %v", err)
	}
	if savedRecord.SyncStatus != syncStateSynced {
		t.Fatalf("expected task record synced, got %q", savedRecord.SyncStatus)
	}
}

func TestSyncUnsyncedRecordsSyncsClosedAttendanceBeforeTaskOut(t *testing.T) {
	setupSyncServiceTestDB(t)

	service := newTestSyncService(true)
	user, attendance, task := seedSyncUserTaskAttendance(t, syncStatePending, nil)
	timeOut := attendance.TimeIn.Add(90 * time.Minute)
	attendance.TimeOut = &timeOut
	if err := db.DB.Save(attendance).Error; err != nil {
		t.Fatalf("save attendance timeout: %v", err)
	}
	record := seedTaskRecord(t, user, attendance, task, true, nil)

	var mu sync.Mutex
	var calls []string
	attachFesTestServer(t, service, func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		calls = append(calls, request.URL.Path)
		mu.Unlock()

		switch request.URL.Path {
		case "/wf/login_attendance":
			writeFesResponse(t, writer, http.StatusOK, map[string]any{
				"login_logout": "remote-login-1",
				"login":        float64(attendance.TimeIn.UnixMilli()),
			})
		case "/wf/logout":
			writeFesResponse(t, writer, http.StatusOK, map[string]any{"error": "Successful"})
		case "/wf/task_in":
			writeFesResponse(t, writer, http.StatusOK, map[string]any{"productivity": "prod-1"})
		case "/wf/task_out":
			writeFesResponse(t, writer, http.StatusOK, map[string]any{"error": "Successful"})
		default:
			t.Fatalf("unexpected FES path: %s", request.URL.Path)
		}
	})

	if err := service.syncUnsyncedRecordsNow(); err != nil {
		t.Fatalf("sync unsynced closed attendance records: %v", err)
	}

	mu.Lock()
	gotCalls := append([]string(nil), calls...)
	mu.Unlock()
	expectedCalls := []string{"/wf/login_attendance", "/wf/logout", "/wf/task_in", "/wf/task_out"}
	if len(gotCalls) != len(expectedCalls) {
		t.Fatalf("expected calls %v, got %v", expectedCalls, gotCalls)
	}
	for i := range expectedCalls {
		if gotCalls[i] != expectedCalls[i] {
			t.Fatalf("expected calls %v, got %v", expectedCalls, gotCalls)
		}
	}

	var savedAttendance models.Attendance
	if err := db.DB.First(&savedAttendance, attendance.ID).Error; err != nil {
		t.Fatalf("load saved attendance: %v", err)
	}
	if savedAttendance.SyncStatus != syncStateSynced {
		t.Fatalf("expected attendance synced, got %q", savedAttendance.SyncStatus)
	}
	if savedAttendance.TimeOut == nil || !savedAttendance.TimeOut.Equal(timeOut) {
		t.Fatalf("expected local time_out preserved as %s, got %v", timeOut, savedAttendance.TimeOut)
	}

	var savedRecord models.TaskRecord
	if err := db.DB.First(&savedRecord, record.ID).Error; err != nil {
		t.Fatalf("load saved task record: %v", err)
	}
	if savedRecord.SyncStatus != syncStateSynced {
		t.Fatalf("expected task record synced, got %q", savedRecord.SyncStatus)
	}
}

func TestSyncAttendanceFailureStates(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		response       map[string]any
		wantStatus     string
		wantLastErrSub string
	}{
		{
			name:           "remote error is failed",
			status:         http.StatusOK,
			response:       map[string]any{"error": "Bubble validation failed"},
			wantStatus:     syncStateFailed,
			wantLastErrSub: "Bubble validation failed",
		},
		{
			name:           "missing login logout is failed",
			status:         http.StatusOK,
			response:       map[string]any{},
			wantStatus:     syncStateFailed,
			wantLastErrSub: "missing login_logout",
		},
		{
			name:           "http failure is failed",
			status:         http.StatusInternalServerError,
			response:       map[string]any{"message": "server down"},
			wantStatus:     syncStateFailed,
			wantLastErrSub: "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupSyncServiceTestDB(t)

			service := newTestSyncService(true)
			_, attendance, _ := seedSyncUserTaskAttendance(t, syncStatePending, nil)
			attachFesTestServer(t, service, func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/wf/login_attendance" {
					t.Fatalf("unexpected FES path: %s", request.URL.Path)
				}
				writeFesResponse(t, writer, tt.status, tt.response)
			})

			err := service.syncAttendanceNow(attendance)
			if err == nil {
				t.Fatal("expected attendance sync failure")
			}

			var saved models.Attendance
			if dbErr := db.DB.First(&saved, attendance.ID).Error; dbErr != nil {
				t.Fatalf("load saved attendance: %v", dbErr)
			}
			if saved.SyncStatus != tt.wantStatus {
				t.Fatalf("expected attendance status %q, got %q", tt.wantStatus, saved.SyncStatus)
			}
			if !strings.Contains(saved.LastError, tt.wantLastErrSub) {
				t.Fatalf("expected last_error to contain %q, got %q", tt.wantLastErrSub, saved.LastError)
			}
			if saved.AttemptCount != 1 {
				t.Fatalf("expected one local sync attempt, got %d", saved.AttemptCount)
			}
		})
	}
}

func TestSyncAttendanceLogoutEmailResolutionFailureMarksFailed(t *testing.T) {
	setupSyncServiceTestDB(t)

	user := &models.User{ID: "user-1"}
	if err := db.DB.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	loginLogoutID := "remote-login-1"
	timeIn := time.Now().Add(-2 * time.Hour)
	timeOut := timeIn.Add(time.Hour)
	attendance := &models.Attendance{
		UserID:        user.ID,
		TimeIn:        timeIn,
		TimeOut:       &timeOut,
		LoginLogoutID: &loginLogoutID,
		SyncStatus:    syncStatePending,
	}
	if err := db.DB.Create(attendance).Error; err != nil {
		t.Fatalf("create attendance: %v", err)
	}

	service := newTestSyncService(true)
	service.AuthUser = &models.AuthUser{LocalID: user.ID}

	err := service.syncAttendanceNow(attendance)
	if err == nil {
		t.Fatal("expected email resolution failure")
	}

	var saved models.Attendance
	if dbErr := db.DB.First(&saved, attendance.ID).Error; dbErr != nil {
		t.Fatalf("load saved attendance: %v", dbErr)
	}
	if saved.SyncStatus != syncStateFailed {
		t.Fatalf("expected attendance status %q, got %q", syncStateFailed, saved.SyncStatus)
	}
	if !strings.Contains(saved.LastError, "unable to resolve user email") {
		t.Fatalf("expected email resolution last_error, got %q", saved.LastError)
	}
}

func TestSyncTaskRecordFailureStates(t *testing.T) {
	tests := []struct {
		name           string
		handler        http.HandlerFunc
		wantLastErrSub string
	}{
		{
			name: "task in remote error",
			handler: func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/wf/task_in" {
					t.Fatalf("unexpected FES path: %s", request.URL.Path)
				}
				writeFesResponse(t, writer, http.StatusOK, map[string]any{"error": "remote task_in rejected"})
			},
			wantLastErrSub: "remote task_in rejected",
		},
		{
			name: "task in missing productivity",
			handler: func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/wf/task_in" {
					t.Fatalf("unexpected FES path: %s", request.URL.Path)
				}
				writeFesResponse(t, writer, http.StatusOK, map[string]any{})
			},
			wantLastErrSub: "missing productivity",
		},
		{
			name: "task out http failure",
			handler: func(writer http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case "/wf/task_in":
					writeFesResponse(t, writer, http.StatusOK, map[string]any{"productivity": "prod-1"})
				case "/wf/task_out":
					writeFesResponse(t, writer, http.StatusBadGateway, map[string]any{"message": "gateway unavailable"})
				default:
					t.Fatalf("unexpected FES path: %s", request.URL.Path)
				}
			},
			wantLastErrSub: "502",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupSyncServiceTestDB(t)

			service := newTestSyncService(true)
			loginLogoutID := "remote-login-1"
			user, attendance, task := seedSyncUserTaskAttendance(t, syncStateSynced, &loginLogoutID)
			record := seedTaskRecord(t, user, attendance, task, true, nil)
			attachFesTestServer(t, service, tt.handler)

			err := service.syncTaskRecordNow(context.Background(), record.ID)
			if err == nil {
				t.Fatal("expected task record sync failure")
			}

			var saved models.TaskRecord
			if dbErr := db.DB.First(&saved, record.ID).Error; dbErr != nil {
				t.Fatalf("load saved task record: %v", dbErr)
			}
			if saved.SyncStatus != syncStateFailed {
				t.Fatalf("expected task record status %q, got %q", syncStateFailed, saved.SyncStatus)
			}
			if !strings.Contains(saved.LastError, tt.wantLastErrSub) {
				t.Fatalf("expected last_error to contain %q, got %q", tt.wantLastErrSub, saved.LastError)
			}
			if saved.AttemptCount != 1 {
				t.Fatalf("expected one local sync attempt, got %d", saved.AttemptCount)
			}
			if saved.EndedAt == nil {
				t.Fatal("expected ended_at to remain saved locally")
			}
		})
	}
}

func TestRejectFailedRemoteResponse(t *testing.T) {
	tests := []struct {
		name    string
		resp    map[string]any
		wantErr bool
	}{
		{name: "nil response", resp: nil, wantErr: true},
		{name: "empty response", resp: map[string]any{}, wantErr: false},
		{name: "successful error marker", resp: map[string]any{"error": "Successful"}, wantErr: false},
		{name: "blank error marker", resp: map[string]any{"error": " "}, wantErr: false},
		{name: "remote failure marker", resp: map[string]any{"error": "Denied"}, wantErr: true},
		{name: "non string error marker", resp: map[string]any{"error": false}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rejectFailedRemoteResponse(tt.resp)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
