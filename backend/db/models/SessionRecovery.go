package models

// SessionRecovery is the non-secret state the frontend needs in order to
// restore an interrupted local shift. The Bubble bearer token is deliberately
// never exposed to the renderer.
type SessionRecovery struct {
	Recovered       bool        `json:"recovered"`
	User            *AuthUser   `json:"user,omitempty"`
	Attendance      *Attendance `json:"attendance,omitempty"`
	ActiveTaskID    string      `json:"activeTaskId,omitempty"`
	ActiveTaskTitle string      `json:"activeTaskTitle,omitempty"`
}
