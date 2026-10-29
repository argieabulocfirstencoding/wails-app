package service_classes

import (
	"reflect"
	"testing"
	"time"

	"fesapp/backend/db/models"
)

func TestConflictResolutionRequestUsesFlatAttendanceDates(t *testing.T) {
	expected := time.Date(2026, time.October, 30, 8, 0, 0, 0, time.UTC)
	endpoint, body, err := conflictResolutionRequest(models.SyncConflict{
		RecordType: "attendance", RemoteID: "attendance-1", ExpectedRemoteModifiedAt: expected,
	}, map[string]any{
		"time_in": float64(expected.UnixMilli()), "time_out": nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if endpoint != bubbleResolveAttendanceConflictEndpoint {
		t.Fatalf("unexpected endpoint: %s", endpoint)
	}
	if body["record_id"] != "attendance-1" || body["expected_modified_at"] != "2026-10-30T08:00:00Z" || body["time_in"] != "2026-10-30T08:00:00Z" || body["time_out"] != nil {
		t.Fatalf("unexpected request body: %#v", body)
	}
}

func TestConflictResolutionRequestUsesFlatTaskFields(t *testing.T) {
	started := time.Date(2026, time.October, 30, 8, 0, 0, 0, time.UTC)
	ended := started.Add(15 * time.Minute)
	endpoint, body, err := conflictResolutionRequest(models.SyncConflict{
		RecordType: "task", RemoteID: "task-1", ExpectedRemoteModifiedAt: started,
	}, map[string]any{
		"started_at": float64(started.UnixMilli()), "ended_at": float64(ended.UnixMilli()), "duration": float64(900),
		"notes": "desktop note", "padagan": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if endpoint != bubbleResolveTaskConflictEndpoint {
		t.Fatalf("unexpected endpoint: %s", endpoint)
	}
	if body["record_id"] != "task-1" || body["started_at"] != "2026-10-30T08:00:00Z" || body["ended_at"] != "2026-10-30T08:15:00Z" || body["duration"] != int32(900) || body["notes"] != "desktop note" || body["padagan"] != true {
		t.Fatalf("unexpected request body: %#v", body)
	}
}

func TestConflictTimestampAcceptsBubbleDate(t *testing.T) {
	fallback := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	got := conflictTimestamp(map[string]any{"modified_at": "2026-10-30T08:15:00Z"}, fallback)
	want := time.Date(2026, time.October, 30, 8, 15, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestChooseConflictValuesUsesEachSelectedSource(t *testing.T) {
	local := map[string]any{"notes": "desktop note", "duration": float64(60)}
	bubble := map[string]any{"notes": "Bubble note", "duration": float64(45)}
	fields := []models.SyncConflictField{
		{Field: "duration", LocalValue: local["duration"], BubbleValue: bubble["duration"]},
		{Field: "notes", LocalValue: local["notes"], BubbleValue: bubble["notes"]},
	}

	merged, err := chooseConflictValues(local, bubble, fields, map[string]string{
		"duration": "bubble",
		"notes":    "local",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(merged, map[string]any{"notes": "desktop note", "duration": float64(45)}) {
		t.Fatalf("unexpected merged values: %#v", merged)
	}
}

func TestChooseConflictValuesRequiresEveryField(t *testing.T) {
	_, err := chooseConflictValues(
		map[string]any{"notes": "desktop"},
		map[string]any{"notes": "Bubble"},
		[]models.SyncConflictField{{Field: "notes"}},
		map[string]string{},
	)
	if err == nil {
		t.Fatal("expected missing selection to be rejected")
	}
}
