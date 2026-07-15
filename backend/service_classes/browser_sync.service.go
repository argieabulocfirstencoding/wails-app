package service_classes

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/types"

	"gorm.io/gorm"
)

const bubbleAttendanceProductivitySyncEndpoint = "wf/sync_attendance_prod"

type BrowserSyncResult struct {
	AttendancesCreated int `json:"attendancesCreated"`
	AttendancesUpdated int `json:"attendancesUpdated"`
	RecordsCreated     int `json:"recordsCreated"`
	RecordsUpdated     int `json:"recordsUpdated"`
	Conflicts          int `json:"conflicts"`
	Skipped            int `json:"skipped"`
}

type bubbleAttendanceRecord struct {
	ID         string
	EmployeeID string
	ModifiedAt time.Time
	TimeIn     time.Time
	TimeOut    *time.Time
}

type bubbleProductivityRecord struct {
	ID           string
	EmployeeID   string
	ModifiedAt   time.Time
	AttendanceID string
	TaskID       string
	StartedAt    time.Time
	EndedAt      *time.Time
	Duration     int32
	Notes        string
	Padagan      bool
}

func (s *SyncService) SyncBrowserRecords() (*BrowserSyncResult, error) {
	if s == nil || s.FesClientService == nil {
		return nil, fmt.Errorf("browser sync service is unavailable")
	}
	if !s.Connectivity.IsOnline() {
		return nil, fmt.Errorf("cannot check browser updates while offline")
	}
	if s.AuthUser == nil || strings.TrimSpace(s.AuthUser.LocalID) == "" {
		return nil, fmt.Errorf("cannot check browser updates without an authenticated user")
	}

	user, err := s.UserRepository.GetUserFromId(s.AuthUser.LocalID)
	if err != nil {
		return nil, fmt.Errorf("load authenticated employee: %w", err)
	}
	employeeID := strings.TrimSpace(user.EmployeeId)
	if employeeID == "" {
		return nil, fmt.Errorf("cannot check browser updates without an employee id")
	}

	response, err := s.FesClientService.Request(types.Request{
		Url:    bubbleAttendanceProductivitySyncEndpoint,
		Method: "POST",
		Body:   map[string]any{},
	})
	if err != nil {
		return nil, fmt.Errorf("fetch browser updates: %w", err)
	}

	attendances, err := parseBubbleAttendances(response["employee_login_logout"])
	if err != nil {
		return nil, fmt.Errorf("parse browser attendance updates: %w", err)
	}
	records, err := parseBubbleProductivities(response["employee_prods"])
	if err != nil {
		return nil, fmt.Errorf("parse browser productivity updates: %w", err)
	}

	result := &BrowserSyncResult{}
	err = db.DB.Transaction(func(tx *gorm.DB) error {
		attendanceByRemoteID := make(map[string]*models.Attendance)
		for _, remote := range attendances {
			if remote.EmployeeID != employeeID {
				result.Skipped++
				continue
			}

			local, created, updated, conflict, err := reconcileBrowserAttendance(tx, s.AuthUser.LocalID, remote)
			if err != nil {
				return err
			}
			if created {
				result.AttendancesCreated++
			}
			if updated {
				result.AttendancesUpdated++
			}
			if conflict {
				result.Conflicts++
			}
			if local != nil {
				attendanceByRemoteID[remote.ID] = local
			}
		}

		for _, remote := range records {
			if remote.EmployeeID != employeeID {
				result.Skipped++
				continue
			}
			attendance := attendanceByRemoteID[remote.AttendanceID]
			if attendance == nil {
				var err error
				attendance, err = findAttendanceByRemoteID(tx, s.AuthUser.LocalID, remote.AttendanceID)
				if err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						result.Skipped++
						continue
					}
					return err
				}
			}

			created, updated, conflict, skipped, err := reconcileBrowserProductivity(tx, s.AuthUser.LocalID, attendance, remote)
			if err != nil {
				return err
			}
			if created {
				result.RecordsCreated++
			}
			if updated {
				result.RecordsUpdated++
			}
			if conflict {
				result.Conflicts++
			}
			if skipped {
				result.Skipped++
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reconcile browser updates: %w", err)
	}

	s.emit("browser-records-synced", result)
	return result, nil
}

func reconcileBrowserAttendance(tx *gorm.DB, userID string, remote bubbleAttendanceRecord) (*models.Attendance, bool, bool, bool, error) {
	local, err := findAttendanceByRemoteID(tx, userID, remote.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		now := time.Now()
		remoteID := remote.ID
		local = &models.Attendance{
			UserID:           userID,
			ClientTime:       uint32(remote.TimeIn.Unix()),
			TimeIn:           remote.TimeIn,
			TimeOut:          remote.TimeOut,
			LoginLogoutID:    &remoteID,
			RemoteModifiedAt: &remote.ModifiedAt,
			SyncedAt:         &now,
			SyncStatus:       syncStateSynced,
		}
		if err := tx.Create(local).Error; err != nil {
			return nil, false, false, false, err
		}
		return local, true, false, false, nil
	}
	if err != nil {
		return nil, false, false, false, err
	}
	if hasProtectedLocalSyncState(local.SyncStatus) {
		return local, false, false, true, nil
	}
	if !remoteIsNewer(remote.ModifiedAt, local.RemoteModifiedAt) {
		return local, false, false, false, nil
	}

	now := time.Now()
	local.TimeIn = remote.TimeIn
	local.TimeOut = remote.TimeOut
	local.RemoteModifiedAt = &remote.ModifiedAt
	local.SyncedAt = &now
	local.SyncStatus = syncStateSynced
	local.LastError = ""
	if err := tx.Save(local).Error; err != nil {
		return nil, false, false, false, err
	}
	return local, false, true, false, nil
}

func reconcileBrowserProductivity(tx *gorm.DB, userID string, attendance *models.Attendance, remote bubbleProductivityRecord) (bool, bool, bool, bool, error) {
	if attendance == nil || strings.TrimSpace(remote.TaskID) == "" {
		return false, false, false, true, nil
	}

	var task models.Task
	if err := tx.Where("task_id = ?", remote.TaskID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, false, false, true, nil
		}
		return false, false, false, false, err
	}

	var local models.TaskRecord
	err := tx.Where("user_id = ? AND productivity = ?", userID, remote.ID).First(&local).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		now := time.Now()
		remoteID := remote.ID
		local = models.TaskRecord{
			UserID:           userID,
			AttendanceID:     attendance.ID,
			TaskID:           remote.TaskID,
			StartedAt:        remote.StartedAt,
			EndedAt:          remote.EndedAt,
			Duration:         remote.Duration,
			Padagan:          remote.Padagan,
			Notes:            remote.Notes,
			Productivity:     &remoteID,
			RemoteModifiedAt: &remote.ModifiedAt,
			SyncedAt:         &now,
			SyncStatus:       syncStateSynced,
		}
		if err := tx.Create(&local).Error; err != nil {
			return false, false, false, false, err
		}
		return true, false, false, false, nil
	}
	if err != nil {
		return false, false, false, false, err
	}
	if hasProtectedLocalSyncState(local.SyncStatus) {
		return false, false, true, false, nil
	}
	if !remoteIsNewer(remote.ModifiedAt, local.RemoteModifiedAt) {
		return false, false, false, false, nil
	}

	now := time.Now()
	local.AttendanceID = attendance.ID
	local.TaskID = remote.TaskID
	local.StartedAt = remote.StartedAt
	local.EndedAt = remote.EndedAt
	local.Duration = remote.Duration
	local.Padagan = remote.Padagan
	local.Notes = remote.Notes
	local.RemoteModifiedAt = &remote.ModifiedAt
	local.SyncedAt = &now
	local.SyncStatus = syncStateSynced
	local.LastError = ""
	if err := tx.Save(&local).Error; err != nil {
		return false, false, false, false, err
	}
	return false, true, false, false, nil
}

func findAttendanceByRemoteID(tx *gorm.DB, userID, remoteID string) (*models.Attendance, error) {
	var attendance models.Attendance
	err := tx.Where("user_id = ? AND login_logout_id = ?", userID, remoteID).First(&attendance).Error
	if err != nil {
		return nil, err
	}
	return &attendance, nil
}

func hasProtectedLocalSyncState(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case syncStatePending, syncStateFailed, syncStateBlocked, syncStateInProgress:
		return true
	default:
		return false
	}
}

func remoteIsNewer(remote time.Time, local *time.Time) bool {
	return local == nil || remote.After(*local)
}

func parseBubbleAttendances(value any) ([]bubbleAttendanceRecord, error) {
	objects, err := bubbleObjectList(value)
	if err != nil {
		return nil, err
	}
	out := make([]bubbleAttendanceRecord, 0, len(objects))
	for _, item := range objects {
		id := bubbleString(item, "_id")
		employeeID := bubbleString(item, "EmployeeID")
		modifiedAt, ok := bubbleTime(item, "Modified Date")
		if id == "" || employeeID == "" || !ok {
			continue
		}
		timeIn, ok := bubbleTime(item, "Login")
		if !ok {
			continue
		}
		var timeOut *time.Time
		if value, ok := bubbleTime(item, "Logout"); ok {
			timeOut = &value
		}
		out = append(out, bubbleAttendanceRecord{ID: id, EmployeeID: employeeID, ModifiedAt: modifiedAt, TimeIn: timeIn, TimeOut: timeOut})
	}
	return out, nil
}

func parseBubbleProductivities(value any) ([]bubbleProductivityRecord, error) {
	objects, err := bubbleObjectList(value)
	if err != nil {
		return nil, err
	}
	out := make([]bubbleProductivityRecord, 0, len(objects))
	for _, item := range objects {
		id := bubbleString(item, "_id")
		employeeID := bubbleString(item, "EmployeeID")
		attendanceID := bubbleString(item, "Login - Logout")
		taskID := bubbleString(item, "Projects - Projects")
		modifiedAt, modifiedOK := bubbleTime(item, "Modified Date")
		startedAt, startedOK := bubbleTime(item, "Actual Task In")
		if id == "" || employeeID == "" || attendanceID == "" || taskID == "" || !modifiedOK || !startedOK {
			continue
		}
		var endedAt *time.Time
		if value, ok := bubbleTime(item, "Actual Task Out"); ok {
			endedAt = &value
		}
		out = append(out, bubbleProductivityRecord{
			ID: id, EmployeeID: employeeID, AttendanceID: attendanceID, TaskID: taskID,
			ModifiedAt: modifiedAt, StartedAt: startedAt, EndedAt: endedAt,
			Duration: int32(bubbleNumber(item, "Total Task Time")),
			Notes:    bubbleString(item, "ActivityDescription"),
			Padagan:  strings.EqualFold(bubbleString(item, "Operate from TopTracker (Padagan)"), "yes"),
		})
	}
	return out, nil
}

func bubbleObjectList(value any) ([]map[string]any, error) {
	if value == nil {
		return []map[string]any{}, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected list, got %T", value)
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected object, got %T", item)
		}
		out = append(out, object)
	}
	return out, nil
}

func bubbleString(item map[string]any, key string) string {
	value, ok := item[key]
	if !ok {
		return ""
	}
	stringValue, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(stringValue)
}

func bubbleNumber(item map[string]any, key string) int64 {
	value, ok := item[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	default:
		return 0
	}
}

func bubbleTime(item map[string]any, key string) (time.Time, bool) {
	milliseconds := bubbleNumber(item, key)
	if milliseconds <= 0 {
		return time.Time{}, false
	}
	return time.UnixMilli(milliseconds), true
}
