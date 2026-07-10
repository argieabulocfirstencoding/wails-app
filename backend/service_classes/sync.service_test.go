package service_classes

import (
	"fesapp/backend/db/models"
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
