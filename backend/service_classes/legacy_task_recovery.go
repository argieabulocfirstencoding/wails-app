package service_classes

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/types"
	"fesapp/backend/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LegacyTaskRecoveryCandidate is a Bubble productivity row the user may
// explicitly link to one legacy desktop task record.
type LegacyTaskRecoveryCandidate struct {
	ProductivityID  string     `json:"productivityId"`
	ClientSessionID string     `json:"clientSessionId"`
	StartedAt       time.Time  `json:"startedAt"`
	EndedAt         *time.Time `json:"endedAt,omitempty"`
	Duration        int32      `json:"duration"`
	Notes           string     `json:"notes"`
	Padagan         bool       `json:"padagan"`
}

func legacyTaskRecoveryEligible(record *models.TaskRecord) bool {
	return record != nil &&
		!taskRecordHasRemoteProductivity(record) &&
		(record.ClientSessionID == nil || strings.TrimSpace(*record.ClientSessionID) == "") &&
		(record.AttemptCount > 0 || record.LastAttemptAt != nil || record.TaskInAttemptedAt != nil || record.SyncStatus != syncStatePending)
}

func (s *SyncService) legacyTaskRecoveryRecord(recordID uint) (*models.TaskRecord, *models.Attendance, error) {
	record, err := s.TaskRepository.GetTaskRecord(recordID)
	if err != nil {
		return nil, nil, err
	}
	if err := s.ensureTaskRecordOwnership(record); err != nil {
		return nil, nil, err
	}
	if !legacyTaskRecoveryEligible(record) {
		return nil, nil, fmt.Errorf("task record is not eligible for legacy recovery")
	}
	attendance, err := s.AuthRepository.GetAttendanceFromID(record.AttendanceID)
	if err != nil {
		return nil, nil, err
	}
	if attendance.LoginLogoutID == nil || strings.TrimSpace(*attendance.LoginLogoutID) == "" {
		return nil, nil, fmt.Errorf("legacy task recovery requires a synced Bubble login session")
	}
	return record, attendance, nil
}

func (s *SyncService) legacyTaskRecoveryCandidates(record *models.TaskRecord, attendance *models.Attendance) ([]bubbleProductivityRecord, error) {
	if !s.Connectivity.IsOnline() {
		return nil, fmt.Errorf("cannot recover a legacy task record while offline")
	}
	user, err := s.UserRepository.GetUserFromId(record.UserID)
	if err != nil {
		return nil, err
	}
	employeeID, err := s.resolveEmployeeID(user)
	if err != nil {
		return nil, err
	}
	response, err := s.FesClientService.Request(types.Request{Url: bubbleAttendanceProductivitySyncEndpoint, Method: "GET", Body: map[string]any{}})
	if err != nil {
		return nil, fmt.Errorf("fetch Bubble task sessions: %w", err)
	}
	parsed, err := parseBubbleProductivities(response["employee_prods"])
	if err != nil {
		return nil, fmt.Errorf("parse Bubble task sessions: %w", err)
	}
	attendanceID := strings.TrimSpace(*attendance.LoginLogoutID)
	candidates := make([]bubbleProductivityRecord, 0)
	for _, remote := range parsed.Records {
		if remote.EmployeeID == employeeID && remote.AttendanceID == attendanceID && remote.TaskID == record.TaskID {
			candidates = append(candidates, remote)
		}
	}
	return candidates, nil
}

func recoveryCandidate(remote bubbleProductivityRecord) LegacyTaskRecoveryCandidate {
	return LegacyTaskRecoveryCandidate{ProductivityID: remote.ID, ClientSessionID: remote.ClientSessionID, StartedAt: remote.StartedAt, EndedAt: remote.EndedAt, Duration: remote.Duration, Notes: remote.Notes, Padagan: remote.Padagan}
}

func (s *SyncService) GetLegacyTaskRecoveryCandidates(recordID uint) ([]LegacyTaskRecoveryCandidate, error) {
	correlationID := utils.NewCorrelationID("legacy_task_recovery_candidates")
	record, attendance, err := s.legacyTaskRecoveryRecord(recordID)
	if err != nil {
		return nil, err
	}
	candidates, err := s.legacyTaskRecoveryCandidates(record, attendance)
	if err != nil {
		return nil, err
	}
	out := make([]LegacyTaskRecoveryCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, recoveryCandidate(candidate))
	}
	s.Logger.Infof("op=legacy_task_recovery_candidates correlation_id=%s record_id=%d candidates=%d", correlationID, recordID, len(out))
	return out, nil
}

func (s *SyncService) LinkLegacyTaskRecovery(recordID uint, productivityID string) error {
	correlationID := utils.NewCorrelationID("legacy_task_recovery_link")
	productivityID = strings.TrimSpace(productivityID)
	if productivityID == "" {
		return fmt.Errorf("select a Bubble task session to link")
	}
	record, attendance, err := s.legacyTaskRecoveryRecord(recordID)
	if err != nil {
		return err
	}
	candidates, err := s.legacyTaskRecoveryCandidates(record, attendance)
	if err != nil {
		return err
	}
	var selected *bubbleProductivityRecord
	for i := range candidates {
		if candidates[i].ID == productivityID {
			selected = &candidates[i]
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("selected Bubble task session is no longer a valid recovery candidate")
	}

	now := time.Now()
	return db.DB.Transaction(func(tx *gorm.DB) error {
		var current models.TaskRecord
		if err := tx.Where("id = ? AND user_id = ?", record.ID, record.UserID).First(&current).Error; err != nil {
			return err
		}
		if !legacyTaskRecoveryEligible(&current) {
			return fmt.Errorf("task record changed and can no longer be recovered this way")
		}
		var duplicateCount int64
		if err := tx.Model(&models.TaskRecord{}).Where("user_id = ? AND productivity = ? AND id <> ?", current.UserID, selected.ID, current.ID).Count(&duplicateCount).Error; err != nil {
			return err
		}
		if duplicateCount > 0 {
			return fmt.Errorf("selected Bubble task session is already linked to another local record")
		}
		if strings.TrimSpace(selected.ClientSessionID) != "" {
			if err := tx.Model(&models.TaskRecord{}).Where("user_id = ? AND client_session_id = ? AND id <> ?", current.UserID, selected.ClientSessionID, current.ID).Count(&duplicateCount).Error; err != nil {
				return err
			}
			if duplicateCount > 0 {
				return fmt.Errorf("selected Bubble task session identity is already linked to another local record")
			}
		}
		updates := map[string]any{"productivity": selected.ID, "remote_modified_at": selected.ModifiedAt, "task_in_idempotent": true, "last_error": ""}
		if strings.TrimSpace(selected.ClientSessionID) != "" {
			updates["client_session_id"] = selected.ClientSessionID
		}
		if err := tx.Model(&models.TaskRecord{}).Where("id = ?", current.ID).Updates(updates).Error; err != nil {
			return err
		}
		current.Productivity = &selected.ID
		if strings.TrimSpace(selected.ClientSessionID) != "" {
			current.ClientSessionID = &selected.ClientSessionID
		}
		if len(differingConflictFields(taskConflictValues(&current), bubbleTaskConflictValues(*selected))) > 0 {
			if err := markTaskConflict(tx, current.UserID, &current, *selected); err != nil {
				return err
			}
		} else if err := tx.Model(&models.TaskRecord{}).Where("id = ?", current.ID).Updates(map[string]any{"sync_status": syncStateSynced, "synced_at": now, "last_error": ""}).Error; err != nil {
			return err
		}
		details, _ := json.Marshal(map[string]any{"correlation_id": correlationID, "source": "user_selected_bubble_session"})
		return tx.Create(&models.TaskRecoveryAudit{UserID: current.UserID, TaskRecordID: current.ID, Action: "linked", RemoteID: selected.ID, Details: string(details)}).Error
	})
}

func (s *SyncService) CreateLegacyTaskRecovery(recordID uint) error {
	correlationID := utils.NewCorrelationID("legacy_task_recovery_create")
	record, _, err := s.legacyTaskRecoveryRecord(recordID)
	if err != nil {
		return err
	}
	if !s.Connectivity.IsOnline() {
		return fmt.Errorf("cannot create a recovered Bubble task while offline")
	}
	sessionID := uuid.NewString()
	now := time.Now()
	if err := db.DB.Transaction(func(tx *gorm.DB) error {
		var current models.TaskRecord
		if err := tx.Where("id = ? AND user_id = ?", record.ID, record.UserID).First(&current).Error; err != nil {
			return err
		}
		if !legacyTaskRecoveryEligible(&current) {
			return fmt.Errorf("task record changed and can no longer be recovered this way")
		}
		if err := tx.Model(&models.TaskRecord{}).Where("id = ?", current.ID).Updates(map[string]any{"client_session_id": sessionID, "task_in_attempted_at": now, "task_in_idempotent": true, "sync_status": syncStatePending, "last_error": ""}).Error; err != nil {
			return err
		}
		details, _ := json.Marshal(map[string]any{"correlation_id": correlationID, "source": "user_confirmed_no_matching_bubble_session", "client_session_id": sessionID})
		return tx.Create(&models.TaskRecoveryAudit{UserID: current.UserID, TaskRecordID: current.ID, Action: "create_confirmed", Details: string(details)}).Error
	}); err != nil {
		return err
	}
	if err := s.syncTaskRecord(recordID, true); err != nil {
		return err
	}
	s.Logger.Infof("op=legacy_task_recovery_create_success correlation_id=%s record_id=%d", correlationID, recordID)
	return nil
}
