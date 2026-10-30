package service_classes

import (
	"fmt"
	"strings"
	"time"

	"fesapp/backend/db"
	"fesapp/backend/db/models"
)

// LoginAttendanceCandidate is a Bubble attendance that may safely be resumed
// during desktop login. RemoteID is the Bubble Login / Logout thing ID.
type LoginAttendanceCandidate struct {
	RemoteID        string    `json:"remoteId"`
	AttendanceID    uint      `json:"attendanceId"`
	TimeIn          time.Time `json:"timeIn"`
	ActiveTaskLabel string    `json:"activeTaskLabel"`
}

// ReconcileLoginAttendances imports the configured Bubble history first, then
// returns only remote-backed, open sessions within the existing 16-hour resume
// rule. It deliberately never selects the newest session on behalf of a user.
func (s *SyncService) ReconcileLoginAttendances() ([]LoginAttendanceCandidate, error) {
	if s == nil || s.AuthUser == nil || strings.TrimSpace(s.AuthUser.LocalID) == "" {
		return nil, fmt.Errorf("cannot reconcile Bubble attendance without an authenticated user")
	}
	if _, err := s.SyncBrowserRecords(); err != nil {
		return nil, err
	}

	return findLoginAttendanceCandidates(s.AuthUser.LocalID, time.Now())
}

func findLoginAttendanceCandidates(userID string, now time.Time) ([]LoginAttendanceCandidate, error) {
	cutoff := now.Add(-16 * time.Hour)
	var attendances []models.Attendance
	if err := db.DB.Where("user_id = ? AND time_out IS NULL AND login_logout_id IS NOT NULL AND TRIM(login_logout_id) <> '' AND time_in >= ?", userID, cutoff).
		Order("time_in DESC, id DESC").Find(&attendances).Error; err != nil {
		return nil, fmt.Errorf("find resumable Bubble attendances: %w", err)
	}

	candidates := make([]LoginAttendanceCandidate, 0, len(attendances))
	for _, attendance := range attendances {
		remoteID := strings.TrimSpace(*attendance.LoginLogoutID)
		candidate := LoginAttendanceCandidate{RemoteID: remoteID, AttendanceID: attendance.ID, TimeIn: attendance.TimeIn}
		var active models.TaskRecord
		if err := db.DB.Preload("Task").Where("attendance_id = ? AND ended_at IS NULL", attendance.ID).Order("started_at DESC, id DESC").First(&active).Error; err == nil {
			active.SetTaskLabel()
			candidate.ActiveTaskLabel = active.TaskLabel
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}
