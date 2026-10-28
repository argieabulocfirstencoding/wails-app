package models

import "time"

// SessionRecovery is the non-secret state the frontend needs in order to
// restore an interrupted local shift. The Bubble bearer token is deliberately
// never exposed to the renderer.
type SessionRecovery struct {
	Recovered       bool        `json:"recovered"`
	RequiresReview  bool        `json:"requiresReview"`
	User            *AuthUser   `json:"user,omitempty"`
	Attendance      *Attendance `json:"attendance,omitempty"`
	ActiveTaskID    string      `json:"activeTaskId,omitempty"`
	ActiveTaskTitle string      `json:"activeTaskTitle,omitempty"`
}

type InterruptedShiftSummary struct {
	AttendanceID uint                    `json:"attendanceId"`
	TimeIn       time.Time               `json:"timeIn"`
	Records      []InterruptedTaskRecord `json:"records"`
	OpenRecordID uint                    `json:"openRecordId,omitempty"`
	SuggestedEnd *time.Time              `json:"suggestedEnd,omitempty"`
}

type InterruptedTaskRecord struct {
	ID         uint       `json:"id"`
	TaskID     string     `json:"taskId"`
	TaskTitle  string     `json:"taskTitle"`
	StartedAt  time.Time  `json:"startedAt"`
	EndedAt    *time.Time `json:"endedAt,omitempty"`
	Duration   int32      `json:"duration"`
	SyncStatus string     `json:"syncStatus"`
	Open       bool       `json:"open"`
}
