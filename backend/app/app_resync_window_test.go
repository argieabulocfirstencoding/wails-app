package app

import (
	"fesapp/backend/db/models"
	"fesapp/backend/services"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestCanResyncAttendanceByTime(t *testing.T) {
	now := time.Date(2026, time.May, 29, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		inputTime time.Time
		want      bool
	}{
		{
			name:      "under 2 days allowed",
			inputTime: now.Add(-47 * time.Hour),
			want:      true,
		},
		{
			name:      "exactly 2 days blocked",
			inputTime: now.Add(-48 * time.Hour),
			want:      false,
		},
		{
			name:      "older than 2 days blocked",
			inputTime: now.Add(-(48*time.Hour + time.Minute)),
			want:      false,
		},
		{
			name:      "future time allowed",
			inputTime: now.Add(3 * time.Hour),
			want:      true,
		},
		{
			name:      "zero time blocked",
			inputTime: time.Time{},
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := canResyncAttendanceByTime(tc.inputTime, now)
			if got != tc.want {
				t.Fatalf("canResyncAttendanceByTime() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAttendanceResyncReferenceTime(t *testing.T) {
	timeIn := time.Date(2026, time.May, 27, 8, 0, 0, 0, time.UTC)
	timeOut := time.Date(2026, time.May, 27, 17, 0, 0, 0, time.UTC)

	withoutTimeout := &models.Attendance{TimeIn: timeIn}
	withTimeout := &models.Attendance{TimeIn: timeIn, TimeOut: &timeOut}

	if got := attendanceResyncReferenceTime(nil); !got.IsZero() {
		t.Fatalf("attendanceResyncReferenceTime(nil) should be zero time")
	}

	if got := attendanceResyncReferenceTime(withoutTimeout); !got.Equal(timeIn) {
		t.Fatalf("expected time_in reference, got %v", got)
	}

	if got := attendanceResyncReferenceTime(withTimeout); !got.Equal(timeOut) {
		t.Fatalf("expected time_out reference, got %v", got)
	}
}

func TestSyncAttendanceByIDGuards(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		a := &App{}
		err := a.SyncAttendanceByID(1)
		if err == nil {
			t.Fatalf("expected error when auth user is missing")
		}
	})

	t.Run("blocks syncing another users attendance", func(t *testing.T) {
		original := services.GetAttendanceFromID
		defer func() { services.GetAttendanceFromID = original }()

		services.GetAttendanceFromID = func(attendanceID uint) (*models.Attendance, error) {
			return &models.Attendance{
				Model:  gorm.Model{ID: attendanceID},
				UserID: "other-user",
				TimeIn: time.Now(),
			}, nil
		}

		a := &App{
			AuthUser: &models.AuthUser{LocalID: "current-user"},
		}

		err := a.SyncAttendanceByID(1)
		if err == nil || err.Error() != "cannot resync attendance owned by another user" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("blocks old attendance over 2 days", func(t *testing.T) {
		original := services.GetAttendanceFromID
		defer func() { services.GetAttendanceFromID = original }()

		services.GetAttendanceFromID = func(attendanceID uint) (*models.Attendance, error) {
			old := time.Now().Add(-(49 * time.Hour))
			return &models.Attendance{
				Model:  gorm.Model{ID: attendanceID},
				UserID: "current-user",
				TimeIn: old,
			}, nil
		}

		a := &App{
			AuthUser: &models.AuthUser{LocalID: "current-user"},
		}

		err := a.SyncAttendanceByID(1)
		if err == nil || err.Error() != "cannot resync login/logout older than 2 days" {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
