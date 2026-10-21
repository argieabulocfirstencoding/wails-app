package app

import (
	"archive/zip"
	"encoding/json"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/utils"
	"fmt"
	"io"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"
)

type diagnosticBundleManifest struct {
	GeneratedAt string `json:"generated_at"`
	AppVersion  string `json:"app_version"`
	Platform    string `json:"platform"`
	Arch        string `json:"arch"`
}

type diagnosticAttendanceState struct {
	ID            uint    `json:"id"`
	UserID        string  `json:"user_id"`
	SyncStatus    string  `json:"sync_status"`
	AttemptCount  int32   `json:"attempt_count"`
	LastError     string  `json:"last_error"`
	LastAttemptAt *string `json:"last_attempt_at,omitempty"`
}

type diagnosticTaskRecordState struct {
<<<<<<< HEAD
	ID            uint    `json:"id"`
	UserID        string  `json:"user_id"`
	AttendanceID  uint    `json:"attendance_id"`
	TaskID        string  `json:"task_id"`
	SyncStatus    string  `json:"sync_status"`
	AttemptCount  int32   `json:"attempt_count"`
	LastError     string  `json:"last_error"`
	LastAttemptAt *string `json:"last_attempt_at,omitempty"`
=======
	ClientSessionID   *string `json:"client_session_id,omitempty"`
	Productivity      *string `json:"productivity,omitempty"`
	TaskInAttemptedAt *string `json:"task_in_attempted_at,omitempty"`
	TaskInIdempotent  bool    `json:"task_in_idempotent"`
	ID                uint    `json:"id"`
	UserID            string  `json:"user_id"`
	AttendanceID      uint    `json:"attendance_id"`
	TaskID            string  `json:"task_id"`
	SyncStatus        string  `json:"sync_status"`
	AttemptCount      int32   `json:"attempt_count"`
	LastError         string  `json:"last_error"`
	LastAttemptAt     *string `json:"last_attempt_at,omitempty"`
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
}

type diagnosticSyncStates struct {
	Attendance []diagnosticAttendanceState `json:"attendance"`
	TaskRecord []diagnosticTaskRecordState `json:"task_records"`
}

func (a *App) ExportDiagnosticBundle() (string, error) {
	correlationID := utils.NewCorrelationID("diag")
	a.Logger.Infof("op=diagnostic_export_start correlation_id=%s", correlationID)

	manifest := diagnosticBundleManifest{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		AppVersion:  CurrentVersion,
		Platform:    goruntime.GOOS,
		Arch:        goruntime.GOARCH,
	}

	states, err := loadDiagnosticSyncStates()
	if err != nil {
		a.Logger.Warnf("op=diagnostic_export_failed correlation_id=%s err=%v", correlationID, err)
		return "", a.stableClientError("diagnostic_export", err)
	}

	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("fes-diagnostic-%s.zip", time.Now().UTC().Format("20060102_150405")))
	file, err := os.Create(tmpPath)
	if err != nil {
		a.Logger.Warnf("op=diagnostic_export_failed correlation_id=%s err=%v", correlationID, err)
		return "", a.stableClientError("diagnostic_export", err)
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	if err := writeJSONEntry(zw, "manifest.json", manifest); err != nil {
		_ = zw.Close()
		return "", a.stableClientError("diagnostic_export", err)
	}
	if err := writeJSONEntry(zw, "sync_states.json", states); err != nil {
		_ = zw.Close()
		return "", a.stableClientError("diagnostic_export", err)
	}
	if err := addLogEntry(zw); err != nil {
		_ = zw.Close()
		return "", a.stableClientError("diagnostic_export", err)
	}

	if err := zw.Close(); err != nil {
		return "", a.stableClientError("diagnostic_export", err)
	}

	a.Logger.Infof("op=diagnostic_export_success correlation_id=%s bundle=%s", correlationID, tmpPath)
	return tmpPath, nil
}

func loadDiagnosticSyncStates() (*diagnosticSyncStates, error) {
	var attendances []models.Attendance
	if err := db.DB.Order("id DESC").Limit(200).Find(&attendances).Error; err != nil {
		return nil, err
	}

	var taskRecords []models.TaskRecord
	if err := db.DB.Order("id DESC").Limit(500).Find(&taskRecords).Error; err != nil {
		return nil, err
	}

	out := &diagnosticSyncStates{
		Attendance: make([]diagnosticAttendanceState, 0, len(attendances)),
		TaskRecord: make([]diagnosticTaskRecordState, 0, len(taskRecords)),
	}

	for i := range attendances {
		out.Attendance = append(out.Attendance, diagnosticAttendanceState{
			ID:            attendances[i].ID,
			UserID:        attendances[i].UserID,
			SyncStatus:    attendances[i].SyncStatus,
			AttemptCount:  attendances[i].AttemptCount,
			LastError:     sanitizeDiagnosticError(attendances[i].LastError),
			LastAttemptAt: formatOptionalTime(attendances[i].LastAttemptAt),
		})
	}

	for i := range taskRecords {
		out.TaskRecord = append(out.TaskRecord, diagnosticTaskRecordState{
<<<<<<< HEAD
			ID:            taskRecords[i].ID,
			UserID:        taskRecords[i].UserID,
			AttendanceID:  taskRecords[i].AttendanceID,
			TaskID:        taskRecords[i].TaskID,
			SyncStatus:    taskRecords[i].SyncStatus,
			AttemptCount:  taskRecords[i].AttemptCount,
			LastError:     sanitizeDiagnosticError(taskRecords[i].LastError),
			LastAttemptAt: formatOptionalTime(taskRecords[i].LastAttemptAt),
=======
			ClientSessionID:   taskRecords[i].ClientSessionID,
			Productivity:      taskRecords[i].Productivity,
			TaskInAttemptedAt: formatOptionalTime(taskRecords[i].TaskInAttemptedAt),
			TaskInIdempotent:  taskRecords[i].TaskInIdempotent,
			ID:                taskRecords[i].ID,
			UserID:            taskRecords[i].UserID,
			AttendanceID:      taskRecords[i].AttendanceID,
			TaskID:            taskRecords[i].TaskID,
			SyncStatus:        taskRecords[i].SyncStatus,
			AttemptCount:      taskRecords[i].AttemptCount,
			LastError:         sanitizeDiagnosticError(taskRecords[i].LastError),
			LastAttemptAt:     formatOptionalTime(taskRecords[i].LastAttemptAt),
>>>>>>> bdad11dd336beb05d8c751de525843d9cfa9deef
		})
	}

	return out, nil
}

func addLogEntry(zw *zip.Writer) error {
	logPath := strings.TrimSpace(utils.CurrentLogPath())
	if logPath == "" {
		return nil
	}

	f, err := os.Open(logPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	writer, err := zw.Create("logs/current.log")
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, f)
	return err
}

func writeJSONEntry(zw *zip.Writer, name string, payload any) error {
	writer, err := zw.Create(name)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(writer)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func sanitizeDiagnosticError(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, "\n", " ")
	raw = strings.ReplaceAll(raw, "\r", " ")
	if len(raw) > 180 {
		raw = raw[:180]
	}
	return raw
}

func formatOptionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	v := t.UTC().Format(time.RFC3339)
	return &v
}
