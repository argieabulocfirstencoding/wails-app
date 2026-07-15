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
const bubbleEmployeeDetailsEndpoint = "wf/employee_details"

type BrowserSyncResult struct {
	AttendancesCreated       int    `json:"attendancesCreated"`
	AttendancesUpdated       int    `json:"attendancesUpdated"`
	RecordsCreated           int    `json:"recordsCreated"`
	RecordsUpdated           int    `json:"recordsUpdated"`
	FallbackTasksCreated     int    `json:"fallbackTasksCreated"`
	RecoveredAttendanceLinks int    `json:"recoveredAttendanceLinks"`
	Conflicts                int    `json:"conflicts"`
	Skipped                  int    `json:"skipped"`
	SkippedOtherEmployee     int    `json:"skippedOtherEmployee"`
	SkippedMissingAttendance int    `json:"skippedMissingAttendance"`
	SkippedMalformed         int    `json:"skippedMalformed"`
	RecordsVerified          int    `json:"recordsVerified"`
	RecordsMissingInBubble   int    `json:"recordsMissingInBubble"`
	RecordsMissingID         int    `json:"recordsMissingId"`
	VerificationSkipped      bool   `json:"verificationSkipped"`
	VerificationWarning      string `json:"verificationWarning"`
	CatalogRefreshWarning    string `json:"catalogRefreshWarning"`
}

type bubbleAttendanceRecord struct {
	ID         string
	EmployeeID string
	ModifiedAt time.Time
	TimeIn     time.Time
	TimeOut    *time.Time
}

type bubbleProductivityRecord struct {
	ID             string
	EmployeeID     string
	ModifiedAt     time.Time
	AttendanceID   string
	AttendanceTime time.Time
	TaskID         string
	StartedAt      time.Time
	EndedAt        *time.Time
	Duration       int32
	Notes          string
	Padagan        bool
	TaskTitle      string
	ProjectName    string
}

type bubbleProductivityParseResult struct {
	Records          []bubbleProductivityRecord
	SkippedMalformed int
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
	employeeID, err := s.resolveEmployeeID(user)
	if err != nil {
		return nil, err
	}

	result := &BrowserSyncResult{}
	if err := s.FesClientService.SyncClientsAndProjects(); err != nil {
		result.CatalogRefreshWarning = err.Error()
	}

	response, err := s.FesClientService.Request(types.Request{
		Url:    bubbleAttendanceProductivitySyncEndpoint,
		Method: "GET",
		Body:   map[string]any{},
	})
	if err != nil {
		return nil, fmt.Errorf("fetch browser updates: %w", err)
	}

	attendances, err := parseBubbleAttendances(response["employee_login_logout"])
	if err != nil {
		return nil, fmt.Errorf("parse browser attendance updates: %w", err)
	}
	parsedRecords, err := parseBubbleProductivities(response["employee_prods"])
	if err != nil {
		return nil, fmt.Errorf("parse browser productivity updates: %w", err)
	}
	result.SkippedMalformed = parsedRecords.SkippedMalformed
	result.Skipped += parsedRecords.SkippedMalformed
	if parsedRecords.SkippedMalformed > 0 {
		result.VerificationSkipped = true
		result.VerificationWarning = "Bubble returned incomplete productivity rows; local sync statuses were not verified"
	}

	err = db.DB.Transaction(func(tx *gorm.DB) error {
		attendanceByRemoteID := make(map[string]*models.Attendance)
		for _, remote := range attendances {
			if remote.EmployeeID != employeeID {
				result.Skipped++
				result.SkippedOtherEmployee++
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

		for _, remote := range parsedRecords.Records {
			if remote.EmployeeID != employeeID {
				result.Skipped++
				result.SkippedOtherEmployee++
				continue
			}

			attendance := attendanceByRemoteID[remote.AttendanceID]
			attendanceLinkRecovered := false
			if attendance == nil && remote.AttendanceID == "" && !remote.AttendanceTime.IsZero() {
				attendance = attendanceByTime(attendanceByRemoteID, remote.AttendanceTime)
				if attendance == nil {
					localAttendance, err := findAttendanceByTime(tx, s.AuthUser.LocalID, remote.AttendanceTime)
					if err == nil {
						attendance = localAttendance
					} else if !errors.Is(err, gorm.ErrRecordNotFound) {
						return err
					}
				}
				attendanceLinkRecovered = attendance != nil
			}
			if attendance == nil {
				if remote.AttendanceID == "" {
					result.Skipped++
					result.SkippedMissingAttendance++
					continue
				}
				var err error
				attendance, err = findAttendanceByRemoteID(tx, s.AuthUser.LocalID, remote.AttendanceID)
				if err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						result.Skipped++
						result.SkippedMissingAttendance++
						continue
					}
					return err
				}
			}
			if attendanceLinkRecovered {
				result.RecoveredAttendanceLinks++
			}

			fallbackTaskCreated, err := ensureBrowserTask(tx, remote)
			if err != nil {
				return err
			}
			if fallbackTaskCreated {
				result.FallbackTasksCreated++
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

		if !result.VerificationSkipped {
			remoteProductivityIDs := make(map[string]struct{}, len(parsedRecords.Records))
			for _, remote := range parsedRecords.Records {
				if remote.EmployeeID == employeeID {
					remoteProductivityIDs[remote.ID] = struct{}{}
				}
			}

			verified, missingInBubble, missingID, err := reconcileLocalProductivityPresence(tx, s.AuthUser.LocalID, remoteProductivityIDs)
			if err != nil {
				return err
			}
			result.RecordsVerified = verified
			result.RecordsMissingInBubble = missingInBubble
			result.RecordsMissingID = missingID
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reconcile browser updates: %w", err)
	}

	s.emit("browser-records-synced", result)
	return result, nil
}

func (s *SyncService) resolveEmployeeID(user *models.User) (string, error) {
	if user == nil {
		return "", fmt.Errorf("cannot check browser updates without a local user profile")
	}
	if employeeID := strings.TrimSpace(user.EmployeeId); employeeID != "" {
		return employeeID, nil
	}

	email := strings.TrimSpace(user.Email)
	if email == "" && s.AuthUser != nil {
		email = strings.TrimSpace(s.AuthUser.Email)
	}
	if email == "" {
		return "", fmt.Errorf("cannot refresh employee profile without an email address")
	}

	response, err := s.FesClientService.Request(types.Request{
		Url:    bubbleEmployeeDetailsEndpoint,
		Method: "POST",
		Body:   map[string]any{"email": email},
	})
	if err != nil {
		return "", fmt.Errorf("refresh employee profile: %w", err)
	}

	employeeID := bubbleString(response, "employee_unique_id")
	if employeeID == "" {
		return "", fmt.Errorf("employee profile did not include employee_unique_id")
	}

	user.EmployeeId = employeeID
	if err := db.DB.Save(user).Error; err != nil {
		return "", fmt.Errorf("save refreshed employee profile: %w", err)
	}
	return employeeID, nil
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

// reconcileLocalProductivityPresence verifies local records against the complete
// productivity collection returned by Bubble for the authenticated employee.
func reconcileLocalProductivityPresence(tx *gorm.DB, userID string, remoteIDs map[string]struct{}) (int, int, int, error) {
	var localRecords []models.TaskRecord
	if err := tx.
		Where("user_id = ? AND sync_status IN ?", userID, []string{syncStateSynced, syncStateRemoteMissing}).
		Find(&localRecords).Error; err != nil {
		return 0, 0, 0, err
	}

	verified := 0
	missingInBubble := 0
	missingID := 0
	now := time.Now()
	for i := range localRecords {
		local := &localRecords[i]
		productivityID := ""
		if local.Productivity != nil {
			productivityID = strings.TrimSpace(*local.Productivity)
		}

		if productivityID == "" {
			missingID++
			if err := markTaskRecordRemoteMissing(tx, local, "local synced record has no productivity identifier"); err != nil {
				return 0, 0, 0, err
			}
			continue
		}

		if _, found := remoteIDs[productivityID]; !found {
			missingInBubble++
			message := fmt.Sprintf("productivity %q was not found in Bubble during verification", productivityID)
			if err := markTaskRecordRemoteMissing(tx, local, message); err != nil {
				return 0, 0, 0, err
			}
			continue
		}

		verified++
		if strings.EqualFold(local.SyncStatus, syncStateRemoteMissing) {
			local.SyncStatus = syncStateSynced
			local.SyncedAt = &now
			local.LastError = ""
			if err := tx.Save(local).Error; err != nil {
				return 0, 0, 0, err
			}
		}
	}

	return verified, missingInBubble, missingID, nil
}

func markTaskRecordRemoteMissing(tx *gorm.DB, taskRecord *models.TaskRecord, message string) error {
	if taskRecord == nil {
		return fmt.Errorf("cannot mark nil task record as missing in Bubble")
	}
	if taskRecord.SyncStatus == syncStateRemoteMissing && taskRecord.LastError == message {
		return nil
	}
	taskRecord.SyncStatus = syncStateRemoteMissing
	taskRecord.LastError = message
	return tx.Save(taskRecord).Error
}

func findAttendanceByRemoteID(tx *gorm.DB, userID, remoteID string) (*models.Attendance, error) {
	var attendance models.Attendance
	err := tx.Where("user_id = ? AND login_logout_id = ?", userID, remoteID).First(&attendance).Error
	if err != nil {
		return nil, err
	}
	return &attendance, nil
}

func findAttendanceByTime(tx *gorm.DB, userID string, timeIn time.Time) (*models.Attendance, error) {
	var attendance models.Attendance
	err := tx.Where("user_id = ? AND time_in = ?", userID, timeIn).First(&attendance).Error
	if err != nil {
		return nil, err
	}
	return &attendance, nil
}

func attendanceByTime(attendances map[string]*models.Attendance, timeIn time.Time) *models.Attendance {
	for _, attendance := range attendances {
		if attendance != nil && attendance.TimeIn.Equal(timeIn) {
			return attendance
		}
	}
	return nil
}

func ensureBrowserTask(tx *gorm.DB, remote bubbleProductivityRecord) (bool, error) {
	var task models.Task
	err := tx.Where("task_id = ?", remote.TaskID).First(&task).Error
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}

	title := strings.TrimSpace(remote.TaskTitle)
	if title == "" {
		title = "Browser task"
	}
	projectName := strings.TrimSpace(remote.ProjectName)
	if projectName == "" {
		projectName = "Imported Bubble project"
	}
	if err := tx.Create(&models.Task{TaskID: remote.TaskID, Title: title, ProjectName: projectName}).Error; err != nil {
		return false, err
	}
	return true, nil
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

func parseBubbleProductivities(value any) (bubbleProductivityParseResult, error) {
	objects, err := bubbleObjectList(value)
	if err != nil {
		return bubbleProductivityParseResult{}, err
	}
	out := bubbleProductivityParseResult{Records: make([]bubbleProductivityRecord, 0, len(objects))}
	for _, item := range objects {
		id := bubbleString(item, "_id")
		employeeID := bubbleString(item, "EmployeeID")
		attendanceID := bubbleReference(item, "Login - Logout")
		taskID := bubbleReference(item, "Projects - Projects")
		modifiedAt, modifiedOK := bubbleTime(item, "Modified Date")
		startedAt, startedOK := bubbleTime(item, "Actual Task In")
		if id == "" || employeeID == "" || taskID == "" || !modifiedOK || !startedOK {
			out.SkippedMalformed++
			continue
		}
		var endedAt *time.Time
		if value, ok := bubbleTime(item, "Actual Task Out"); ok {
			endedAt = &value
		}
		attendanceTime, _ := bubbleTime(item, "Date")
		out.Records = append(out.Records, bubbleProductivityRecord{
			ID: id, EmployeeID: employeeID, AttendanceID: attendanceID, AttendanceTime: attendanceTime, TaskID: taskID,
			ModifiedAt: modifiedAt, StartedAt: startedAt, EndedAt: endedAt,
			Duration:    int32(bubbleNumber(item, "Total Task Time")),
			Notes:       bubbleString(item, "ActivityDescription"),
			Padagan:     strings.EqualFold(bubbleString(item, "Operate from TopTracker (Padagan)"), "yes"),
			TaskTitle:   firstBubbleString(item, "Account", "Trackers Used"),
			ProjectName: firstBubbleString(item, "Project Name", "Project Name - Projects", "Projects - Projects Display"),
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

func bubbleReference(item map[string]any, key string) string {
	if value := bubbleString(item, key); value != "" {
		return value
	}
	values, ok := item[key].([]any)
	if !ok || len(values) != 1 {
		return ""
	}
	value, ok := values[0].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func firstBubbleString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := bubbleString(item, key); value != "" {
			return value
		}
	}
	return ""
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
