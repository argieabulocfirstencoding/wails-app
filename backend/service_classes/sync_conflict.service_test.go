package service_classes

import (
	"reflect"
	"testing"

	"fesapp/backend/db/models"
)

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
