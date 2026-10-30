package service_classes

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"github.com/google/uuid"
)

var errTaskStateChanged = errors.New("task changed while synchronizing")

// Legacy rows with previous attempts may already exist remotely. Giving those
// rows a fresh key and posting again would merely give the duplicate a new name.
func prepareTaskSessionIdentity(record *models.TaskRecord) error {
	if record.ClientSessionID != nil && strings.TrimSpace(*record.ClientSessionID) != "" {
		return nil
	}
	if record.AttemptCount > 0 || record.LastAttemptAt != nil || record.TaskInAttemptedAt != nil ||
		record.SyncStatus != syncStatePending {
		return fmt.Errorf("%w: legacy task start has an uncertain remote result; verify the matching Bubble session before retrying", errSyncBlocked)
	}
	id := uuid.NewString()
	result := db.DB.Model(&models.TaskRecord{}).
		Where("id = ? AND (client_session_id IS NULL OR client_session_id = '')", record.ID).
		Update("client_session_id", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errTaskStateChanged
	}
	record.ClientSessionID = &id
	return nil
}

// Commit the dispatch marker BEFORE HTTP. If a process dies or the response is
// lost, only a request sent under the verified idempotent contract can replay.
func recordTaskInDispatch(record *models.TaskRecord) error {
	if record.TaskInAttemptedAt != nil {
		if !record.TaskInIdempotent {
			return fmt.Errorf("%w: task start may already exist in Bubble; verify it before retrying", errSyncBlocked)
		}
		return nil
	}
	now := time.Now()
	result := db.DB.Model(&models.TaskRecord{}).
		Where("id = ? AND task_in_attempted_at IS NULL", record.ID).
		Updates(map[string]any{"task_in_attempted_at": now, "task_in_idempotent": true})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errTaskStateChanged
	}
	record.TaskInAttemptedAt = &now
	record.TaskInIdempotent = true
	return nil
}

// legacyTaskStartNeedsRecovery identifies records created while task-start
// retries were disabled behind the rollout flag.  The original request still
// used ClientSessionID, but the local dispatch marker was deliberately left
// non-idempotent.  Such a record must be reconciled with Bubble before it can
// be retried.
func legacyTaskStartNeedsRecovery(record *models.TaskRecord) bool {
	return record != nil &&
		record.TaskInAttemptedAt != nil &&
		!record.TaskInIdempotent &&
		!taskRecordHasRemoteProductivity(record) &&
		record.ClientSessionID != nil &&
		strings.TrimSpace(*record.ClientSessionID) != ""
}

// authorizeLegacyTaskInReplay records that a complete authenticated Bubble
// reconciliation found no record for this durable session ID.  From then on,
// task_in may be replayed with the same key under the idempotent contract.
func authorizeLegacyTaskInReplay(record *models.TaskRecord) error {
	if !legacyTaskStartNeedsRecovery(record) {
		return nil
	}

	result := db.DB.Model(&models.TaskRecord{}).
		Where(`id = ? AND user_id = ? AND task_in_attempted_at IS NOT NULL
			AND task_in_idempotent = ? AND (productivity IS NULL OR TRIM(productivity) = '')`, record.ID, record.UserID, false).
		Update("task_in_idempotent", true)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errTaskStateChanged
	}
	record.TaskInIdempotent = true
	return nil
}

func persistTaskProductivity(record *models.TaskRecord, productivity string) error {
	productivity = strings.TrimSpace(productivity)
	if productivity == "" {
		return fmt.Errorf("task-in response missing productivity identifier")
	}
	result := db.DB.Model(&models.TaskRecord{}).
		Where("id = ? AND user_id = ? AND (productivity IS NULL OR TRIM(productivity) = '' OR productivity = ?)", record.ID, record.UserID, productivity).
		Update("productivity", productivity)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("%w: task session has a conflicting Bubble identity; review required", errSyncBlocked)
	}
	record.Productivity = &productivity
	return nil
}
