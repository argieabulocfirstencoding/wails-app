package service_classes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fesapp/backend/db"
	"fesapp/backend/db/models"
)

func legacyRecoveryFixture(t *testing.T) (*SyncService, *models.Attendance, *models.User, *models.TaskRecord) {
	t.Helper()
	s, attendance, user, record := sessionSyncFixture(t)
	now := time.Now().Round(time.Millisecond)
	if err := db.DB.Model(record).Updates(map[string]any{"client_session_id": nil, "task_in_attempted_at": nil, "task_in_idempotent": false, "attempt_count": 1, "last_attempt_at": now, "sync_status": syncStateBlocked}).Error; err != nil {
		t.Fatal(err)
	}
	return s, attendance, user, storedSession(t, record.ID)
}

func TestLegacyTaskRecoveryLinksUserSelectedBubbleSession(t *testing.T) {
	s, attendance, user, record := legacyRecoveryFixture(t)
	now := time.Now().Round(time.Millisecond)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wf/sync_attendance_prod" {
			t.Fatalf("unexpected request %s", r.URL.Path)
		}
		fmt.Fprintf(w, `{"response":{"employee_prods":[{"_id":"remote-1","EmployeeID":%q,"Login - Logout":%q,"Projects - Projects":%q,"Modified Date":%d,"Actual Task In":%d,"Actual Task Out":%d,"Total Task Time":60,"ActivityDescription":"","Operate from TopTracker (Padagan)":"no"}]}}`, user.EmployeeId, *attendance.LoginLogoutID, record.TaskID, now.UnixMilli(), record.StartedAt.UnixMilli(), now.Add(time.Minute).UnixMilli())
	}))
	defer server.Close()
	s.FesClientService.baseUrl = server.URL + "/"
	s.FesClientService.client = server.Client()

	candidates, err := s.GetLegacyTaskRecoveryCandidates(record.ID)
	if err != nil || len(candidates) != 1 || candidates[0].ProductivityID != "remote-1" {
		t.Fatalf("candidates=%#v err=%v", candidates, err)
	}
	if err := s.LinkLegacyTaskRecovery(record.ID, "remote-1"); err != nil {
		t.Fatal(err)
	}
	stored := storedSession(t, record.ID)
	if stored.Productivity == nil || *stored.Productivity != "remote-1" || stored.SyncStatus != syncStateConflict {
		t.Fatalf("unexpected linked record: %#v", stored)
	}
	var audit models.TaskRecoveryAudit
	if err := db.DB.Where("task_record_id = ? AND action = ?", record.ID, "linked").First(&audit).Error; err != nil {
		t.Fatal(err)
	}
}

func TestLegacyTaskRecoveryCreateUsesNewIdempotentSession(t *testing.T) {
	s, attendance, user, record := legacyRecoveryFixture(t)
	if err := s.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Shutdown)
	created := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wf/sync_attendance_prod":
			fmt.Fprintf(w, `{"response":{"employee_login_logout":[{"_id":%q,"EmployeeID":%q,"Modified Date":%d,"Login":%d}],"employee_prods":[]}}`, *attendance.LoginLogoutID, user.EmployeeId, time.Now().UnixMilli(), attendance.TimeIn.UnixMilli())
		case "/wf/task_in":
			created++
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			key, _ := body["client_session_id"].(string)
			if key == "" {
				t.Fatal("missing recovery client session ID")
			}
			fmt.Fprintf(w, `{"response":{"productivity":"created-1","client_session_id":%q}}`, key)
		case "/wf/task_out":
			w.Write([]byte(`{"response":{}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	s.FesClientService.baseUrl = server.URL + "/"
	s.FesClientService.client = server.Client()
	if err := s.CreateLegacyTaskRecovery(record.ID); err != nil {
		t.Fatal(err)
	}
	stored := storedSession(t, record.ID)
	if created != 1 || stored.ClientSessionID == nil || !stored.TaskInIdempotent || stored.Productivity == nil || *stored.Productivity != "created-1" {
		t.Fatalf("unexpected recovery result created=%d record=%#v", created, stored)
	}
}

func TestLegacyTaskRecoveryRejectsCandidateFromOtherAttendance(t *testing.T) {
	s, attendance, user, record := legacyRecoveryFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"response":{"employee_prods":[{"_id":"wrong-attendance","EmployeeID":%q,"Login - Logout":"other-attendance","Projects - Projects":%q,"Modified Date":%d,"Actual Task In":%d}]}}`, user.EmployeeId, record.TaskID, time.Now().UnixMilli(), record.StartedAt.UnixMilli())
	}))
	defer server.Close()
	s.FesClientService.baseUrl = server.URL + "/"
	s.FesClientService.client = server.Client()
	candidates, err := s.GetLegacyTaskRecoveryCandidates(record.ID)
	if err != nil || len(candidates) != 0 {
		t.Fatalf("candidates=%#v err=%v", candidates, err)
	}
	if err := s.LinkLegacyTaskRecovery(record.ID, "wrong-attendance"); err == nil {
		t.Fatal("expected invalid candidate rejection")
	}
	stored := storedSession(t, record.ID)
	if stored.Productivity != nil {
		t.Fatalf("linked invalid candidate: %#v", stored)
	}
	_ = attendance
}
