package service_classes

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TaskSyncRecoveryResult tells the desktop whether a single Sync action
// completed or requires a narrow, safe follow-up confirmation/review.
type TaskSyncRecoveryResult struct {
	Action     string                        `json:"action"`
	Message    string                        `json:"message"`
	Candidates []LegacyTaskRecoveryCandidate `json:"candidates,omitempty"`
}

func taskNeedsConditionalRecovery(record *models.TaskRecord) bool {
	return legacyTaskRecoveryEligible(record) || taskRecordNeedsRemoteRepair(record)
}

func (s *SyncService) taskRecoveryContext(recordID uint) (*models.TaskRecord, *models.Attendance, error) {
	record, err := s.TaskRepository.GetTaskRecord(recordID)
	if err != nil {
		return nil, nil, err
	}
	if err := s.ensureTaskRecordOwnership(record); err != nil {
		return nil, nil, err
	}
	if !taskNeedsConditionalRecovery(record) {
		return nil, nil, fmt.Errorf("task record does not require conditional recovery")
	}
	attendance, err := s.AuthRepository.GetAttendanceFromID(record.AttendanceID)
	if err != nil {
		return nil, nil, err
	}
	if attendance.LoginLogoutID == nil || strings.TrimSpace(*attendance.LoginLogoutID) == "" {
		return nil, nil, fmt.Errorf("task recovery requires a synced Bubble login session")
	}
	return record, attendance, nil
}

func taskTimesOverlap(local *models.TaskRecord, remote bubbleProductivityRecord) bool {
	if local == nil || local.StartedAt.IsZero() || remote.StartedAt.IsZero() {
		return false
	}
	now := time.Now()
	localEnd := now
	if local.EndedAt != nil {
		localEnd = *local.EndedAt
	}
	remoteEnd := now
	if remote.EndedAt != nil {
		remoteEnd = *remote.EndedAt
	}
	return local.StartedAt.Before(remoteEnd) && remote.StartedAt.Before(localEnd)
}

func recoveryCandidates(records []bubbleProductivityRecord) []LegacyTaskRecoveryCandidate {
	out := make([]LegacyTaskRecoveryCandidate, 0, len(records))
	for _, record := range records {
		out = append(out, recoveryCandidate(record))
	}
	return out
}

func (s *SyncService) recordTaskRecoveryAudit(userID string, recordID uint, action, remoteID string, details map[string]any) {
	raw, err := json.Marshal(details)
	if err != nil {
		return
	}
	if err := db.DB.Create(&models.TaskRecoveryAudit{UserID: userID, TaskRecordID: recordID, Action: action, RemoteID: remoteID, Details: string(raw)}).Error; err != nil {
		s.Logger.Warnf("op=task_sync_recovery_audit_failed record_id=%d action=%s err=%v", recordID, action, err)
	}
}

func (s *SyncService) linkConditionalTaskRecovery(record *models.TaskRecord, selected bubbleProductivityRecord, correlationID string) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		var current models.TaskRecord
		if err := tx.Where("id = ? AND user_id = ?", record.ID, record.UserID).First(&current).Error; err != nil {
			return err
		}
		if !taskNeedsConditionalRecovery(&current) {
			return fmt.Errorf("task record changed and no longer needs recovery")
		}
		var duplicateCount int64
		if err := tx.Model(&models.TaskRecord{}).Where("user_id = ? AND productivity = ? AND id <> ?", current.UserID, selected.ID, current.ID).Count(&duplicateCount).Error; err != nil {
			return err
		}
		if duplicateCount > 0 {
			return fmt.Errorf("matching Bubble task is already linked to another local record")
		}
		if strings.TrimSpace(selected.ClientSessionID) != "" {
			if err := tx.Model(&models.TaskRecord{}).Where("user_id = ? AND client_session_id = ? AND id <> ?", current.UserID, selected.ClientSessionID, current.ID).Count(&duplicateCount).Error; err != nil {
				return err
			}
			if duplicateCount > 0 {
				return fmt.Errorf("matching Bubble task identity is already linked to another local record")
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
		if len(differingConflictFields(taskConflictValues(&current), bubbleTaskConflictValues(selected))) > 0 {
			if err := markTaskConflict(tx, current.UserID, &current, selected); err != nil {
				return err
			}
		} else if err := tx.Model(&models.TaskRecord{}).Where("id = ?", current.ID).Updates(map[string]any{"sync_status": syncStateSynced, "synced_at": time.Now(), "last_error": ""}).Error; err != nil {
			return err
		}
		details, _ := json.Marshal(map[string]any{"correlation_id": correlationID, "source": "automatic_exact_start_match"})
		return tx.Create(&models.TaskRecoveryAudit{UserID: current.UserID, TaskRecordID: current.ID, Action: "linked", RemoteID: selected.ID, Details: string(details)}).Error
	})
}

func (s *SyncService) createConditionalTaskRecovery(record *models.TaskRecord, correlationID string) error {
	sessionID := uuid.NewString()
	now := time.Now()
	return db.DB.Transaction(func(tx *gorm.DB) error {
		var current models.TaskRecord
		if err := tx.Where("id = ? AND user_id = ?", record.ID, record.UserID).First(&current).Error; err != nil {
			return err
		}
		if !taskNeedsConditionalRecovery(&current) {
			return fmt.Errorf("task record changed and no longer needs recovery")
		}
		if err := tx.Model(&models.TaskRecord{}).Where("id = ?", current.ID).Updates(map[string]any{"productivity": nil, "client_session_id": sessionID, "task_in_attempted_at": now, "task_in_idempotent": true, "sync_status": syncStatePending, "last_error": ""}).Error; err != nil {
			return err
		}
		details, _ := json.Marshal(map[string]any{"correlation_id": correlationID, "source": "user_confirmed_verified_absence", "client_session_id": sessionID})
		return tx.Create(&models.TaskRecoveryAudit{UserID: current.UserID, TaskRecordID: current.ID, Action: "create_confirmed", Details: string(details)}).Error
	})
}

// SyncTaskRecordWithRecovery is the single UI-facing task sync path. It
// delegates ordinary records to the existing sync pipeline, while records
// without a trustworthy Bubble identity are verified before any replacement is
// created.
func (s *SyncService) SyncTaskRecordWithRecovery(recordID uint, confirmCreate bool) (*TaskSyncRecoveryResult, error) {
	correlationID := utils.NewCorrelationID("task_sync_recovery")
	record, err := s.TaskRepository.GetTaskRecord(recordID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureTaskRecordOwnership(record); err != nil {
		return nil, err
	}
	if !taskNeedsConditionalRecovery(record) {
		if err := s.ResyncTaskRecord(recordID); err != nil {
			return nil, err
		}
		return &TaskSyncRecoveryResult{Action: "synced", Message: "Task synced."}, nil
	}

	record, attendance, err := s.taskRecoveryContext(recordID)
	if err != nil {
		return nil, err
	}
	candidates, err := s.legacyTaskRecoveryCandidates(record, attendance)
	if err != nil {
		return nil, err
	}
	exact := make([]bubbleProductivityRecord, 0, 1)
	overlaps := make([]bubbleProductivityRecord, 0)
	for _, candidate := range candidates {
		if candidate.StartedAt.UnixMilli() == record.StartedAt.UnixMilli() {
			exact = append(exact, candidate)
			continue
		}
		if taskTimesOverlap(record, candidate) {
			overlaps = append(overlaps, candidate)
		}
	}
	if len(exact) == 1 {
		if err := s.linkConditionalTaskRecovery(record, exact[0], correlationID); err != nil {
			return nil, err
		}
		return &TaskSyncRecoveryResult{Action: "linked", Message: "Existing Bubble task linked."}, nil
	}
	if len(exact) > 1 || len(overlaps) > 0 {
		all := append(exact, overlaps...)
		s.recordTaskRecoveryAudit(record.UserID, record.ID, "review_required", "", map[string]any{"correlation_id": correlationID, "exact_matches": len(exact), "overlaps": len(overlaps)})
		return &TaskSyncRecoveryResult{Action: "needs_review", Message: "Possible duplicate time found in Bubble. Review is required before creating another task.", Candidates: recoveryCandidates(all)}, nil
	}
	if !confirmCreate {
		return &TaskSyncRecoveryResult{Action: "needs_create_confirmation", Message: "No matching or overlapping Bubble task was found. Confirm to create one from this local record."}, nil
	}
	if err := s.createConditionalTaskRecovery(record, correlationID); err != nil {
		return nil, err
	}
	if err := s.syncTaskRecord(recordID, true); err != nil {
		return nil, err
	}
	return &TaskSyncRecoveryResult{Action: "created", Message: "Bubble task created and synced."}, nil
}
