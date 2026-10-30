package service_classes

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/types"
	"gorm.io/gorm"
)

const (
	bubbleResolveAttendanceConflictEndpoint = "wf/resolve_attendance_conflict"
	bubbleResolveTaskConflictEndpoint       = "wf/resolve_task_conflict"
)

func attendanceConflictValues(local *models.Attendance) map[string]any {
	values := map[string]any{"time_in": local.TimeIn.UnixMilli(), "time_out": nil}
	if local.TimeOut != nil {
		values["time_out"] = local.TimeOut.UnixMilli()
	}
	return values
}

func bubbleAttendanceConflictValues(remote bubbleAttendanceRecord) map[string]any {
	values := map[string]any{"time_in": remote.TimeIn.UnixMilli(), "time_out": nil}
	if remote.TimeOut != nil {
		values["time_out"] = remote.TimeOut.UnixMilli()
	}
	return values
}

func taskConflictValues(local *models.TaskRecord) map[string]any {
	values := map[string]any{
		"started_at": local.StartedAt.UnixMilli(), "ended_at": nil, "duration": local.Duration,
		"notes": local.Notes, "padagan": local.Padagan,
	}
	if local.EndedAt != nil {
		values["ended_at"] = local.EndedAt.UnixMilli()
	}
	return values
}

func bubbleTaskConflictValues(remote bubbleProductivityRecord) map[string]any {
	values := map[string]any{
		"started_at": remote.StartedAt.UnixMilli(), "ended_at": nil, "duration": remote.Duration,
		"notes": remote.Notes, "padagan": remote.Padagan,
	}
	if remote.EndedAt != nil {
		values["ended_at"] = remote.EndedAt.UnixMilli()
	}
	return values
}

func differingConflictFields(local, remote map[string]any) []models.SyncConflictField {
	keys := make([]string, 0, len(local))
	for key := range local {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fields := make([]models.SyncConflictField, 0, len(keys))
	for _, key := range keys {
		if !reflect.DeepEqual(local[key], remote[key]) {
			fields = append(fields, models.SyncConflictField{Field: key, LocalValue: local[key], BubbleValue: remote[key]})
		}
	}
	return fields
}

func storeSyncConflict(tx *gorm.DB, userID, recordType, remoteID string, attendanceID, taskRecordID *uint, modifiedAt time.Time, local, remote map[string]any) error {
	fields := differingConflictFields(local, remote)
	if len(fields) == 0 {
		return nil
	}
	localJSON, err := json.Marshal(local)
	if err != nil {
		return err
	}
	remoteJSON, err := json.Marshal(remote)
	if err != nil {
		return err
	}
	fieldsJSON, err := json.Marshal(fields)
	if err != nil {
		return err
	}

	query := tx.Where("user_id = ? AND record_type = ? AND remote_id = ? AND status IN ?", userID, recordType, remoteID, []string{"open", "stale", "identity_conflict"})
	var existing models.SyncConflict
	err = query.First(&existing).Error
	if err == nil {
		return tx.Model(&existing).Updates(map[string]any{
			"attendance_id": attendanceID, "task_record_id": taskRecordID, "expected_remote_modified_at": modifiedAt,
			"local_values": string(localJSON), "bubble_values": string(remoteJSON), "differences": string(fieldsJSON), "status": "open",
		}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Create(&models.SyncConflict{
		UserID: userID, RecordType: recordType, RemoteID: remoteID, AttendanceID: attendanceID, TaskRecordID: taskRecordID,
		ExpectedRemoteModifiedAt: modifiedAt, LocalValues: string(localJSON), BubbleValues: string(remoteJSON), Differences: string(fieldsJSON), Status: "open",
	}).Error
}

func storeIdentityConflict(tx *gorm.DB, userID, recordType, remoteID string, attendanceID, taskRecordID *uint, modifiedAt time.Time, local, remote map[string]any) error {
	localJSON, err := json.Marshal(local)
	if err != nil {
		return err
	}
	remoteJSON, err := json.Marshal(remote)
	if err != nil {
		return err
	}
	reasonJSON, err := json.Marshal([]models.SyncConflictField{{Field: "identity", LocalValue: "desktop", BubbleValue: "bubble"}})
	if err != nil {
		return err
	}
	var existing models.SyncConflict
	err = tx.Where("user_id = ? AND record_type = ? AND remote_id = ? AND status = ?", userID, recordType, remoteID, "identity_conflict").First(&existing).Error
	if err == nil {
		return tx.Model(&existing).Updates(map[string]any{
			"attendance_id": attendanceID, "task_record_id": taskRecordID, "expected_remote_modified_at": modifiedAt,
			"local_values": string(localJSON), "bubble_values": string(remoteJSON), "differences": string(reasonJSON),
		}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Create(&models.SyncConflict{
		UserID: userID, RecordType: recordType, RemoteID: remoteID, AttendanceID: attendanceID, TaskRecordID: taskRecordID,
		ExpectedRemoteModifiedAt: modifiedAt, LocalValues: string(localJSON), BubbleValues: string(remoteJSON), Differences: string(reasonJSON), Status: "identity_conflict",
	}).Error
}

func markAttendanceConflict(tx *gorm.DB, userID string, local *models.Attendance, remote bubbleAttendanceRecord) error {
	if err := storeSyncConflict(tx, userID, "attendance", remote.ID, &local.ID, nil, remote.ModifiedAt, attendanceConflictValues(local), bubbleAttendanceConflictValues(remote)); err != nil {
		return err
	}
	return tx.Model(&models.Attendance{}).Where("id = ?", local.ID).Updates(map[string]any{"sync_status": syncStateConflict, "last_error": "Bubble and desktop values differ; review required"}).Error
}

func markTaskConflict(tx *gorm.DB, userID string, local *models.TaskRecord, remote bubbleProductivityRecord) error {
	if err := storeSyncConflict(tx, userID, "task", remote.ID, nil, &local.ID, remote.ModifiedAt, taskConflictValues(local), bubbleTaskConflictValues(remote)); err != nil {
		return err
	}
	return tx.Model(&models.TaskRecord{}).Where("id = ?", local.ID).Updates(map[string]any{"sync_status": syncStateConflict, "last_error": "Bubble and desktop values differ; review required"}).Error
}

func markAttendanceIdentityConflict(tx *gorm.DB, userID string, local *models.Attendance, remote bubbleAttendanceRecord) error {
	if err := storeIdentityConflict(tx, userID, "attendance", remote.ID, &local.ID, nil, remote.ModifiedAt, attendanceConflictValues(local), bubbleAttendanceConflictValues(remote)); err != nil {
		return err
	}
	return tx.Model(&models.Attendance{}).Where("id = ?", local.ID).Updates(map[string]any{"sync_status": syncStateIdentityConflict, "last_error": "Bubble session has no shared record identity; administrator review required"}).Error
}

func markTaskIdentityConflict(tx *gorm.DB, userID string, local *models.TaskRecord, remote bubbleProductivityRecord) error {
	if err := storeIdentityConflict(tx, userID, "task", remote.ID, nil, &local.ID, remote.ModifiedAt, taskConflictValues(local), bubbleTaskConflictValues(remote)); err != nil {
		return err
	}
	return tx.Model(&models.TaskRecord{}).Where("id = ?", local.ID).Updates(map[string]any{"sync_status": syncStateIdentityConflict, "last_error": "Bubble session has no shared record identity; administrator review required"}).Error
}

func (s *SyncService) GetOpenSyncConflicts(userID string) ([]models.SyncConflict, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("cannot list conflicts without an authenticated user")
	}
	var conflicts []models.SyncConflict
	err := db.DB.Where("user_id = ? AND status IN ?", userID, []string{"open", "stale", "identity_conflict"}).Order("updated_at DESC").Find(&conflicts).Error
	if err != nil {
		return nil, err
	}
	for i := range conflicts {
		if conflicts[i].TaskRecordID == nil {
			continue
		}
		var record models.TaskRecord
		if err := db.DB.Preload("Task.Client").First(&record, *conflicts[i].TaskRecordID).Error; err == nil {
			record.SetTaskLabel()
			conflicts[i].TaskLabel = record.TaskLabel
		}
	}
	return conflicts, nil
}

func jsonConflictValues(raw string) (map[string]any, error) {
	values := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, fmt.Errorf("read conflict snapshot: %w", err)
	}
	return values, nil
}

func chooseConflictValues(local, bubble map[string]any, fields []models.SyncConflictField, selections map[string]string) (map[string]any, error) {
	merged := make(map[string]any, len(local))
	for key, value := range local {
		merged[key] = value
	}
	for _, field := range fields {
		choice := strings.ToLower(strings.TrimSpace(selections[field.Field]))
		switch choice {
		case "local":
			merged[field.Field] = local[field.Field]
		case "bubble":
			merged[field.Field] = bubble[field.Field]
		default:
			return nil, fmt.Errorf("select Bubble or desktop for %s", field.Field)
		}
	}
	return merged, nil
}

func conflictTimestamp(response map[string]any, fallback time.Time) time.Time {
	for _, key := range []string{"modified_at", "modifiedAt", "Modified Date"} {
		if value, ok := response[key]; ok {
			switch typed := value.(type) {
			case float64:
				if typed > 0 {
					return time.UnixMilli(int64(typed))
				}
			case int64:
				if typed > 0 {
					return time.UnixMilli(typed)
				}
			case string:
				if timestamp, err := time.Parse(time.RFC3339Nano, typed); err == nil {
					return timestamp
				}
			}
		}
	}
	return fallback
}

// conflictDateValue turns the persisted JSON timestamp back into a Bubble API
// date. Bubble's native API workflow accepts RFC3339 dates directly, avoiding
// a server-side milliseconds-to-date conversion.
func conflictDateValue(values map[string]any, field string) (any, error) {
	timestamp, err := milliValue(values, field)
	if err != nil || timestamp == nil {
		return nil, err
	}
	return timestamp.UTC().Format(time.RFC3339Nano), nil
}

func conflictResolutionRequest(conflict models.SyncConflict, values map[string]any) (string, map[string]any, error) {
	expectedModifiedAt := conflict.ExpectedRemoteModifiedAt.UTC().Format(time.RFC3339Nano)
	switch conflict.RecordType {
	case "attendance":
		timeIn, err := conflictDateValue(values, "time_in")
		if err != nil {
			return "", nil, fmt.Errorf("invalid time_in: %w", err)
		}
		if timeIn == nil {
			return "", nil, fmt.Errorf("invalid time_in")
		}
		timeOut, err := conflictDateValue(values, "time_out")
		if err != nil {
			return "", nil, fmt.Errorf("invalid time_out: %w", err)
		}
		return bubbleResolveAttendanceConflictEndpoint, map[string]any{
			"record_id": conflict.RemoteID, "expected_modified_at": expectedModifiedAt,
			"time_in": timeIn, "time_out": timeOut,
		}, nil
	case "task":
		startedAt, err := conflictDateValue(values, "started_at")
		if err != nil || startedAt == nil {
			return "", nil, fmt.Errorf("invalid started_at")
		}
		endedAt, err := conflictDateValue(values, "ended_at")
		if err != nil {
			return "", nil, fmt.Errorf("invalid ended_at: %w", err)
		}
		duration, err := numberValue(values, "duration")
		if err != nil {
			return "", nil, fmt.Errorf("invalid duration: %w", err)
		}
		notes, ok := values["notes"].(string)
		if !ok {
			return "", nil, fmt.Errorf("invalid notes")
		}
		padagan, ok := values["padagan"].(bool)
		if !ok {
			return "", nil, fmt.Errorf("invalid padagan")
		}
		return bubbleResolveTaskConflictEndpoint, map[string]any{
			"record_id": conflict.RemoteID, "expected_modified_at": expectedModifiedAt,
			"started_at": startedAt, "ended_at": endedAt, "duration": duration,
			"notes": notes, "padagan": padagan,
		}, nil
	default:
		return "", nil, fmt.Errorf("unknown conflict record type")
	}
}

func milliValue(values map[string]any, field string) (*time.Time, error) {
	value, ok := values[field]
	if !ok || value == nil {
		return nil, nil
	}
	number, ok := value.(float64)
	if !ok {
		return nil, fmt.Errorf("invalid %s", field)
	}
	timestamp := time.UnixMilli(int64(number))
	return &timestamp, nil
}

func numberValue(values map[string]any, field string) (int32, error) {
	value, ok := values[field].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid %s", field)
	}
	return int32(value), nil
}

func (s *SyncService) ResolveSyncConflict(userID string, conflictID uint, selections map[string]string) error {
	if s == nil || s.FesClientService == nil {
		return fmt.Errorf("conflict resolver is unavailable")
	}
	var conflict models.SyncConflict
	if err := db.DB.Where("id = ? AND user_id = ?", conflictID, userID).First(&conflict).Error; err != nil {
		return err
	}
	if conflict.Status == "identity_conflict" {
		return fmt.Errorf("identity conflicts require administrator review")
	}
	if conflict.Status != "open" {
		return fmt.Errorf("conflict is no longer open")
	}
	local, err := jsonConflictValues(conflict.LocalValues)
	if err != nil {
		return err
	}
	bubble, err := jsonConflictValues(conflict.BubbleValues)
	if err != nil {
		return err
	}
	var fields []models.SyncConflictField
	if err := json.Unmarshal([]byte(conflict.Differences), &fields); err != nil {
		return fmt.Errorf("read conflict fields: %w", err)
	}
	merged, err := chooseConflictValues(local, bubble, fields, selections)
	if err != nil {
		return err
	}

	endpoint, body, err := conflictResolutionRequest(conflict, merged)
	if err != nil {
		return err
	}
	response, err := s.FesClientService.Request(types.Request{Url: endpoint, Method: "POST", Body: body})
	if err != nil {
		return err
	}
	if stale, _ := response["stale"].(bool); stale {
		return db.DB.Model(&models.SyncConflict{}).Where("id = ?", conflict.ID).Update("status", "stale").Error
	}
	remoteModifiedAt := conflictTimestamp(response, conflict.ExpectedRemoteModifiedAt)
	now := time.Now()
	resolution, _ := json.Marshal(selections)
	return db.DB.Transaction(func(tx *gorm.DB) error {
		switch conflict.RecordType {
		case "attendance":
			if conflict.AttendanceID == nil {
				return fmt.Errorf("conflict has no attendance")
			}
			var attendance models.Attendance
			if err := tx.Where("id = ? AND user_id = ?", *conflict.AttendanceID, userID).First(&attendance).Error; err != nil {
				return err
			}
			timeIn, err := milliValue(merged, "time_in")
			if err != nil || timeIn == nil {
				return fmt.Errorf("invalid time_in")
			}
			timeOut, err := milliValue(merged, "time_out")
			if err != nil {
				return err
			}
			if err := tx.Model(&attendance).Updates(map[string]any{"time_in": *timeIn, "time_out": timeOut, "remote_modified_at": remoteModifiedAt, "sync_status": syncStateSynced, "synced_at": now, "last_error": ""}).Error; err != nil {
				return err
			}
		case "task":
			if conflict.TaskRecordID == nil {
				return fmt.Errorf("conflict has no task record")
			}
			var record models.TaskRecord
			if err := tx.Where("id = ? AND user_id = ?", *conflict.TaskRecordID, userID).First(&record).Error; err != nil {
				return err
			}
			startedAt, err := milliValue(merged, "started_at")
			if err != nil || startedAt == nil {
				return fmt.Errorf("invalid started_at")
			}
			endedAt, err := milliValue(merged, "ended_at")
			if err != nil {
				return err
			}
			duration, err := numberValue(merged, "duration")
			if err != nil {
				return err
			}
			notes, ok := merged["notes"].(string)
			if !ok {
				return fmt.Errorf("invalid notes")
			}
			padagan, ok := merged["padagan"].(bool)
			if !ok {
				return fmt.Errorf("invalid padagan")
			}
			if err := tx.Model(&record).Updates(map[string]any{"started_at": *startedAt, "ended_at": endedAt, "duration": duration, "notes": notes, "padagan": padagan, "remote_modified_at": remoteModifiedAt, "sync_status": syncStateSynced, "synced_at": now, "last_error": ""}).Error; err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown conflict record type")
		}
		return tx.Model(&models.SyncConflict{}).Where("id = ?", conflict.ID).Updates(map[string]any{"status": "resolved", "resolution": string(resolution), "resolved_at": now}).Error
	})
}
