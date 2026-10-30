package service_classes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func sessionSyncFixture(t *testing.T) (*SyncService, *models.Attendance, *models.User, *models.TaskRecord) {
	t.Helper()
	database, user, task := setupBrowserSyncTestDB(t)
	attendance := &models.Attendance{UserID: user.ID, TimeIn: time.Now().Add(-time.Hour), LoginLogoutID: models.StringPtr("attendance-1"), SyncStatus: syncStateSynced}
	if err := database.Create(attendance).Error; err != nil {
		t.Fatal(err)
	}
	record := &models.TaskRecord{UserID: user.ID, AttendanceID: attendance.ID, TaskID: task.TaskID, StartedAt: time.Now().Add(-time.Minute), ClientSessionID: models.StringPtr(uuid.NewString())}
	if err := database.Create(record).Error; err != nil {
		t.Fatal(err)
	}
	connectivity := NewConnectivityStateService()
	connectivity.SetOnline(true)
	s := NewSyncService(connectivity)
	s.AuthUser = &models.AuthUser{LocalID: user.ID}
	// Avoid reading workstation credentials in mocked API tests.
	s.FesClientService.sessionStore = &memoryBubbleSessionStore{values: map[string]string{}}
	if err := s.FesClientService.SetBubbleSession(user.ID, "test-token", time.Hour); err != nil {
		t.Fatalf("set test session: %v", err)
	}
	return s, attendance, user, record
}

func storedSession(t *testing.T, id uint) *models.TaskRecord {
	t.Helper()
	var row models.TaskRecord
	if err := db.DB.First(&row, id).Error; err != nil {
		t.Fatal(err)
	}
	return &row
}

func TestSessionTaskInLostResponseReusesKeyAfterRestart(t *testing.T) {
	t.Setenv("FES_TASK_SESSION_IDEMPOTENCY_ENABLED", "true")
	s, _, _, record := sessionSyncFixture(t)
	var mu sync.Mutex
	created := map[string]string{}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		key, _ := body["client_session_id"].(string)
		if key != *record.ClientSessionID {
			t.Errorf("session key = %q", key)
		}
		calls++
		if created[key] == "" {
			created[key] = "productivity-1"
		}
		if calls == 1 {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			conn.Close()
			return
		}
		fmt.Fprintf(w, `{"response":{"productivity":%q,"client_session_id":%q}}`, created[key], key)
	}))
	defer server.Close()
	s.FesClientService.baseUrl = server.URL + "/"
	if err := s.syncTaskRecordNow(context.Background(), record.ID, false); err == nil {
		t.Fatal("expected lost response")
	}
	first := storedSession(t, record.ID)
	if first.TaskInAttemptedAt == nil || !first.TaskInIdempotent || first.Productivity != nil {
		t.Fatalf("missing dispatch marker: %#v", first)
	}
	// Fresh service, same persisted database; no cached request identity.
	restarted := NewSyncService(s.Connectivity)
	restarted.AuthUser = s.AuthUser
	restarted.FesClientService.sessionStore = s.FesClientService.sessionStore
	restarted.FesClientService.SetActiveSessionUser(s.AuthUser.LocalID)
	restarted.FesClientService.baseUrl = server.URL + "/"
	if err := restarted.syncTaskRecordNow(context.Background(), record.ID, true); err != nil {
		t.Fatal(err)
	}
	stored := storedSession(t, record.ID)
	if stored.Productivity == nil || *stored.Productivity != "productivity-1" || *stored.ClientSessionID != *record.ClientSessionID {
		t.Fatalf("identity lost: %#v", stored)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 2 || len(created) != 1 {
		t.Fatalf("calls=%d creations=%d", calls, len(created))
	}
}

func TestLegacyTaskStartResyncVerifiesAbsenceBeforeReplay(t *testing.T) {
	s, _, _, record := sessionSyncFixture(t)
	now := time.Now()
	if err := db.DB.Model(record).Updates(map[string]any{
		"task_in_attempted_at": now,
		"task_in_idempotent":   false,
		"sync_status":          syncStateBlocked,
	}).Error; err != nil {
		t.Fatal(err)
	}

	var taskInCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/wf/sync_attendance_prod":
			// The authenticated employee's complete collection contains no row
			// for this session, so retrying task_in is safe.
			_, _ = w.Write([]byte(`{"response":{"employee_login_logout":[],"employee_prods":[]}}`))
		case "/wf/task_in":
			taskInCalls++
			_, _ = w.Write([]byte(fmt.Sprintf(`{"response":{"productivity":"productivity-replayed","client_session_id":%q}}`, *record.ClientSessionID)))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	s.FesClientService.baseUrl = server.URL + "/"
	s.FesClientService.client = server.Client()

	if err := s.syncTaskRecordNow(context.Background(), record.ID, true); err != nil {
		t.Fatalf("legacy task-start recovery failed: %v", err)
	}
	if taskInCalls != 1 {
		t.Fatalf("task-in calls = %d, want 1 after verified absence", taskInCalls)
	}
	stored := storedSession(t, record.ID)
	if !stored.TaskInIdempotent || stored.Productivity == nil || *stored.Productivity != "productivity-replayed" {
		t.Fatalf("legacy record was not safely recovered: %#v", stored)
	}
}

func TestLegacyTaskStartResyncDoesNotReplayAfterIncompleteBubbleVerification(t *testing.T) {
	s, _, _, record := sessionSyncFixture(t)
	now := time.Now()
	if err := db.DB.Model(record).Updates(map[string]any{
		"task_in_attempted_at": now,
		"task_in_idempotent":   false,
		"sync_status":          syncStateBlocked,
	}).Error; err != nil {
		t.Fatal(err)
	}

	var taskInCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/wf/sync_attendance_prod":
			// A malformed productivity row means the collection cannot prove
			// this session is absent.
			_, _ = w.Write([]byte(`{"response":{"employee_login_logout":[],"employee_prods":[{}]}}`))
		case "/wf/task_in":
			taskInCalls++
			_, _ = w.Write([]byte(`{"response":{}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	s.FesClientService.baseUrl = server.URL + "/"
	s.FesClientService.client = server.Client()

	err := s.syncTaskRecordNow(context.Background(), record.ID, true)
	if !errors.Is(err, errSyncBlocked) {
		t.Fatalf("expected incomplete Bubble verification to block replay, got %v", err)
	}
	if taskInCalls != 0 {
		t.Fatalf("task-in calls = %d, want 0 when verification is incomplete", taskInCalls)
	}
	stored := storedSession(t, record.ID)
	if stored.TaskInIdempotent {
		t.Fatalf("incomplete verification incorrectly authorized replay: %#v", stored)
	}
}

func TestLegacyTaskStartResyncLinksExistingBubbleSessionInsteadOfReplay(t *testing.T) {
	s, attendance, user, record := sessionSyncFixture(t)
	now := time.Now().Round(time.Millisecond)
	if err := db.DB.Model(record).Updates(map[string]any{
		"started_at":           now,
		"task_in_attempted_at": now,
		"task_in_idempotent":   false,
		"sync_status":          syncStateBlocked,
	}).Error; err != nil {
		t.Fatal(err)
	}

	var taskInCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/wf/sync_attendance_prod":
			payload := fmt.Sprintf(`{"response":{"employee_login_logout":[{"_id":%q,"EmployeeID":%q,"Modified Date":%d,"Login":%d}],"employee_prods":[{"_id":"productivity-existing","client_session_id":%q,"EmployeeID":%q,"Login - Logout":%q,"Projects - Projects":%q,"Modified Date":%d,"Actual Task In":%d}]}}`,
				*attendance.LoginLogoutID, user.EmployeeId, now.UnixMilli(), attendance.TimeIn.UnixMilli(), *record.ClientSessionID, user.EmployeeId, *attendance.LoginLogoutID, record.TaskID, now.UnixMilli(), now.UnixMilli())
			_, _ = w.Write([]byte(payload))
		case "/wf/task_in":
			taskInCalls++
			_, _ = w.Write([]byte(`{"response":{}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	s.FesClientService.baseUrl = server.URL + "/"
	s.FesClientService.client = server.Client()

	if err := s.syncTaskRecordNow(context.Background(), record.ID, true); err != nil {
		t.Fatalf("legacy Bubble-session link failed: %v", err)
	}
	if taskInCalls != 0 {
		t.Fatalf("task-in calls = %d, want 0 when Bubble session already exists", taskInCalls)
	}
	stored := storedSession(t, record.ID)
	if stored.Productivity == nil || *stored.Productivity != "productivity-existing" {
		t.Fatalf("existing Bubble session was not linked: %#v", stored)
	}
}

func TestSessionTaskInRequiresReturnedSessionIdentity(t *testing.T) {
	s, _, _, record := sessionSyncFixture(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["client_session_id"] != *record.ClientSessionID {
			t.Error("required client session identity was not sent")
		}
		w.Write([]byte(`{"response":{"productivity":"productivity-1"}}`))
	}))
	defer server.Close()
	s.FesClientService.baseUrl = server.URL + "/"
	if err := s.syncTaskRecordNow(context.Background(), record.ID, false); err == nil {
		t.Fatal("expected missing echoed identity error")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestSessionLegacyAttemptAndRemoteDeletionNeverCreateReplacement(t *testing.T) {
	t.Setenv("FES_TASK_SESSION_IDEMPOTENCY_ENABLED", "true")
	s, _, _, record := sessionSyncFixture(t)
	for _, state := range []string{syncStateFailed, syncStateRemoteMissing} {
		if err := db.DB.Model(record).Updates(map[string]any{"client_session_id": nil, "attempt_count": 1, "sync_status": state}).Error; err != nil {
			t.Fatal(err)
		}
		if err := s.syncTaskRecordNow(context.Background(), record.ID, true); !errors.Is(err, errSyncBlocked) {
			t.Fatalf("state %s: %v", state, err)
		}
		if storedSession(t, record.ID).ClientSessionID != nil {
			t.Fatal("invented identity for uncertain legacy session")
		}
	}
}

func TestSessionDelayedStartResponsePreservesLocalStop(t *testing.T) {
	t.Setenv("FES_TASK_SESSION_IDEMPOTENCY_ENABLED", "true")
	s, _, _, record := sessionSyncFixture(t)
	arrived := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	var inCalls, outCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wf/task_in" {
			inCalls++
			close(arrived)
			<-release
			fmt.Fprintf(w, `{"response":{"productivity":"productivity-1","client_session_id":%q}}`, *record.ClientSessionID)
			return
		}
		outCalls++
		w.Write([]byte(`{"response":{}}`))
	}))
	defer server.Close()
	defer unblock()
	s.FesClientService.baseUrl = server.URL + "/"
	done := make(chan error, 1)
	go func() { done <- s.syncTaskRecordNow(context.Background(), record.ID, false) }()
	select {
	case <-arrived:
	case <-time.After(5 * time.Second):
		t.Fatal("request never arrived")
	}
	end := time.Now()
	if err := db.DB.Model(record).Updates(map[string]any{"ended_at": end, "duration": 60, "notes": "saved at stop", "sync_status": syncStatePending, "synced_at": nil}).Error; err != nil {
		t.Fatal(err)
	}
	unblock()
	if err := <-done; !errors.Is(err, errTaskStateChanged) {
		t.Fatalf("expected changed snapshot, got %v", err)
	}
	stored := storedSession(t, record.ID)
	if stored.EndedAt == nil || !stored.EndedAt.Equal(end) || stored.Duration != 60 || stored.Notes != "saved at stop" || stored.SyncStatus != syncStatePending || stored.Productivity == nil {
		t.Fatalf("stop overwritten: %#v", stored)
	}
	if err := s.syncTaskRecordNow(context.Background(), record.ID, false); err != nil {
		t.Fatal(err)
	}
	if inCalls != 1 || outCalls != 1 {
		t.Fatalf("in=%d out=%d", inCalls, outCalls)
	}
}

func TestSessionTaskInForStoppedRecordLeavesTaskOutPending(t *testing.T) {
	t.Setenv("FES_TASK_SESSION_IDEMPOTENCY_ENABLED", "true")
	s, attendance, user, record := sessionSyncFixture(t)
	end := time.Now()
	if err := db.DB.Model(record).Updates(map[string]any{"ended_at": end, "duration": 60}).Error; err != nil {
		t.Fatal(err)
	}
	record = storedSession(t, record.ID)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"response":{"productivity":"productivity-1","client_session_id":%q}}`, *record.ClientSessionID)
	}))
	defer server.Close()
	s.FesClientService.baseUrl = server.URL + "/"
	if err := s.syncTaskIn(record, attendance, user); err != nil {
		t.Fatal(err)
	}
	stored := storedSession(t, record.ID)
	if stored.SyncStatus != syncStatePending || stored.Productivity == nil {
		t.Fatalf("task-out would be skipped after restart: %#v", stored)
	}
}

func TestSessionBrowserImportLinksIdentityAndPreservesPendingStop(t *testing.T) {
	_, attendance, user, record := sessionSyncFixture(t)
	end := time.Now()
	if err := db.DB.Model(record).Updates(map[string]any{"ended_at": end, "duration": 60, "notes": "keep local", "sync_status": syncStateFailed}).Error; err != nil {
		t.Fatal(err)
	}
	remote := bubbleProductivityRecord{ID: "remote-1", ClientSessionID: *record.ClientSessionID, TaskID: record.TaskID, StartedAt: time.UnixMilli(record.StartedAt.UnixMilli()), ModifiedAt: end, Notes: "old remote"}
	for attempt := 0; attempt < 2; attempt++ {
		if err := db.DB.Transaction(func(tx *gorm.DB) error {
			created, updated, _, _, err := reconcileBrowserProductivity(tx, user.ID, attendance, remote)
			if created || (attempt == 0 && !updated) {
				t.Fatalf("attempt %d: created=%t updated=%t", attempt, created, updated)
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	stored := storedSession(t, record.ID)
	if stored.Productivity == nil || *stored.Productivity != "remote-1" || stored.EndedAt == nil || !stored.EndedAt.Equal(end) || stored.Notes != "keep local" || stored.SyncStatus != syncStateConflict {
		t.Fatalf("pending stop was not preserved as a review conflict: %#v", stored)
	}
	var count int64
	db.DB.Model(&models.TaskRecord{}).Count(&count)
	if count != 1 {
		t.Fatalf("import duplicated row: %d", count)
	}
}

func TestSessionBrowserImportQuarantinesTimestampOnlyMatch(t *testing.T) {
	_, attendance, user, record := sessionSyncFixture(t)
	remote := bubbleProductivityRecord{ID: "legacy-remote", TaskID: record.TaskID, StartedAt: time.UnixMilli(record.StartedAt.UnixMilli()), ModifiedAt: time.Now()}
	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		created, _, conflict, _, err := reconcileBrowserProductivity(tx, user.ID, attendance, remote)
		if created || !conflict {
			t.Fatal("ambiguous match was imported or merged")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	stored := storedSession(t, record.ID)
	if stored.Productivity != nil || stored.SyncStatus != syncStateIdentityConflict {
		t.Fatalf("expected quarantined identity, got %#v", stored)
	}
}

func TestSessionBrowserImportRejectsConflictingTask(t *testing.T) {
	_, attendance, user, record := sessionSyncFixture(t)
	other := &models.Task{TaskID: "other-task", Title: "Other"}
	if err := db.DB.Create(other).Error; err != nil {
		t.Fatal(err)
	}
	remote := bubbleProductivityRecord{ID: "remote-conflict", ClientSessionID: *record.ClientSessionID, TaskID: other.TaskID, StartedAt: record.StartedAt, ModifiedAt: time.Now()}
	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		created, updated, conflict, _, err := reconcileBrowserProductivity(tx, user.ID, attendance, remote)
		if created || updated || !conflict {
			t.Fatal("identity collision accepted")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if storedSession(t, record.ID).Productivity != nil {
		t.Fatal("conflicting ID attached")
	}
}
