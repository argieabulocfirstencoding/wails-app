package service_classes

import (
<<<<<<< HEAD
	"fesapp/backend/db/models"
=======
	"context"
	"errors"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/types"
	"net/http"
	"net/http/httptest"
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
	"testing"
	"time"
)

func TestAttendanceLoginAlreadySynced(t *testing.T) {
	loginID := "login-1"
	attendance := &models.Attendance{
		LoginLogoutID: &loginID,
		SyncStatus:    syncStateSynced,
	}

	if !attendanceLoginAlreadySynced(attendance) {
		t.Fatalf("expected attendance login to be treated as synced")
	}
}

func TestAttendanceLogoutAlreadySynced(t *testing.T) {
	loginID := "login-1"
	timeOut := time.Now()
	syncedAfterLogout := timeOut.Add(time.Second)

	attendance := &models.Attendance{
		LoginLogoutID: &loginID,
		TimeOut:       &timeOut,
		SyncedAt:      &syncedAfterLogout,
		SyncStatus:    syncStateSynced,
	}

	if !attendanceLogoutAlreadySynced(attendance) {
		t.Fatalf("expected attendance logout to be treated as synced")
	}

	syncedBeforeLogout := timeOut.Add(-time.Second)
	attendance.SyncedAt = &syncedBeforeLogout
	if attendanceLogoutAlreadySynced(attendance) {
		t.Fatalf("expected stale synced_at before logout to require another logout sync")
	}
}

func TestTaskRecordHasRemoteProductivity(t *testing.T) {
	productivity := "productivity-1"
	record := &models.TaskRecord{
		Productivity: &productivity,
	}

	if !taskRecordHasRemoteProductivity(record) {
		t.Fatalf("expected task record with productivity id to be treated as remote-synced")
	}

	record.Productivity = nil
	if taskRecordHasRemoteProductivity(record) {
		t.Fatalf("expected task record without productivity id to require task-in sync")
	}
}
<<<<<<< HEAD
=======

func TestTaskInRejectsMissingEmployeeID(t *testing.T) {
	syncService := NewSyncService()
	err := syncService.syncTaskIn(&models.TaskRecord{}, &models.Attendance{}, &models.User{})
	if err == nil || err.Error() != "cannot sync task-in without an employee ID" {
		t.Fatalf("expected missing employee ID error, got %v", err)
	}
}

func TestShouldSkipCompletedTaskRecordSync(t *testing.T) {
	timeOut := time.Now()
	record := &models.TaskRecord{
		EndedAt:    &timeOut,
		SyncStatus: syncStateSynced,
	}

	if !shouldSkipCompletedTaskRecordSync(record, false) {
		t.Fatalf("expected normal sync to skip completed synced task records")
	}

	if shouldSkipCompletedTaskRecordSync(record, true) {
		t.Fatalf("expected manual resync to force completed synced task records through remote sync")
	}

	record.EndedAt = nil
	if shouldSkipCompletedTaskRecordSync(record, false) {
		t.Fatalf("expected active task records to remain eligible for sync")
	}
}

func TestRemoteMissingTaskRecordRequiresExplicitRepair(t *testing.T) {
	record := &models.TaskRecord{SyncStatus: syncStateRemoteMissing}
	if !taskRecordNeedsRemoteRepair(record) {
		t.Fatal("expected remote-missing record to require repair")
	}
	if syncFailureState(true) != syncStateRemoteMissing {
		t.Fatal("expected repair failure to preserve remote-missing state")
	}
	if syncFailureState(false) != syncStateFailed {
		t.Fatal("expected normal sync failure to use failed state")
	}
}

func TestTaskSyncFailureCode(t *testing.T) {
	if got := taskSyncFailureCode(types.NewStableError("SESSION_REAUTH_REQUIRED", "sign in again")); got != "session_expired" {
		t.Fatalf("expected session_expired for a reauthentication error, got %q", got)
	}

	unauthorized := types.NewRemoteAPIError("FES API", 401, "Invalid or expired token")
	if got := taskSyncFailureCode(unauthorized); got != "session_expired" {
		t.Fatalf("expected session_expired for 401, got %q", got)
	}

	if got := taskSyncFailureCode(errors.New("network interrupted")); got != "sync_failed" {
		t.Fatalf("expected sync_failed for a generic error, got %q", got)
	}
}

func TestCompletedRecordWithProductivityOnlySendsTaskOut(t *testing.T) {
	testDB, err := db.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}
	originalDB := db.DB
	db.DB = testDB
	t.Cleanup(func() { db.DB = originalDB })

	user := &models.User{ID: "user-1", Email: "user@example.com"}
	if err := testDB.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	task := &models.Task{TaskID: "task-1", Title: "Task", ProjectName: "Project"}
	if err := testDB.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	attendanceID := "attendance-remote-1"
	attendance := &models.Attendance{
		UserID:        user.ID,
		TimeIn:        time.Now().Add(-time.Hour),
		LoginLogoutID: &attendanceID,
		SyncStatus:    syncStateSynced,
	}
	if err := testDB.Create(attendance).Error; err != nil {
		t.Fatal(err)
	}
	productivityID := "productivity-remote-1"
	endedAt := time.Now()
	record := &models.TaskRecord{
		UserID:       user.ID,
		AttendanceID: attendance.ID,
		TaskID:       task.TaskID,
		StartedAt:    endedAt.Add(-5 * time.Minute),
		EndedAt:      &endedAt,
		Duration:     300,
		Productivity: &productivityID,
		SyncStatus:   syncStatePending,
	}
	if err := testDB.Create(record).Error; err != nil {
		t.Fatal(err)
	}

	var taskInCalls, taskOutCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wf/task_in":
			taskInCalls++
		case "/wf/task_out":
			taskOutCalls++
		default:
			t.Errorf("unexpected sync endpoint %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":{}}`))
	}))
	defer server.Close()

	connectivity := NewConnectivityStateService()
	connectivity.SetOnline(true)
	syncService := NewSyncService(connectivity)
	fesClient, _ := newTestFesClient(t, server)
	syncService.FesClientService = fesClient
	syncService.AuthUser = &models.AuthUser{LocalID: user.ID}

	if err := syncService.syncTaskRecordNow(context.Background(), record.ID, false); err != nil {
		t.Fatalf("syncTaskRecordNow returned error: %v", err)
	}
	if taskInCalls != 0 || taskOutCalls != 1 {
		t.Fatalf("expected one task-out and no task-in, got task-in=%d task-out=%d", taskInCalls, taskOutCalls)
	}

	var stored models.TaskRecord
	if err := testDB.First(&stored, record.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.SyncStatus != syncStateSynced {
		t.Fatalf("expected synced record, got %q", stored.SyncStatus)
	}
}

func TestForceSyncAttendanceResendsAnAlreadySyncedLogout(t *testing.T) {
	testDB, err := db.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}
	originalDB := db.DB
	db.DB = testDB
	t.Cleanup(func() { db.DB = originalDB })

	user := &models.User{ID: "user-1", Email: "user@example.com"}
	if err := testDB.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	remoteAttendanceID := "attendance-remote-1"
	timeOut := time.Now()
	syncedAt := timeOut.Add(time.Second)
	attendance := &models.Attendance{
		UserID:        user.ID,
		TimeIn:        timeOut.Add(-time.Hour),
		TimeOut:       &timeOut,
		LoginLogoutID: &remoteAttendanceID,
		SyncedAt:      &syncedAt,
		SyncStatus:    syncStateSynced,
	}
	if err := testDB.Create(attendance).Error; err != nil {
		t.Fatal(err)
	}

	logoutCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wf/logout" {
			t.Errorf("unexpected sync endpoint %q", r.URL.Path)
		}
		logoutCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":{}}`))
	}))
	defer server.Close()

	connectivity := NewConnectivityStateService()
	connectivity.SetOnline(true)
	syncService := NewSyncService(connectivity)
	fesClient, _ := newTestFesClient(t, server)
	syncService.FesClientService = fesClient
	syncService.AuthUser = &models.AuthUser{LocalID: user.ID, Email: user.Email}

	if err := syncService.syncAttendanceNowWithForce(attendance, false); err != nil {
		t.Fatalf("normal synced attendance check returned error: %v", err)
	}
	if logoutCalls != 0 {
		t.Fatalf("expected normal sync to skip already-synced logout, got %d calls", logoutCalls)
	}

	if err := syncService.syncAttendanceNowWithForce(attendance, true); err != nil {
		t.Fatalf("force sync returned error: %v", err)
	}
	if logoutCalls != 1 {
		t.Fatalf("expected force sync to resend logout once, got %d calls", logoutCalls)
	}
}
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
