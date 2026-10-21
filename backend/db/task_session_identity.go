package db

import (
	"fesapp/backend/db/models"
	"gorm.io/gorm"
)

const taskSessionIdentityIndex = "idx_task_records_user_client_session"

func planTaskSessionIdentityMigration(database *gorm.DB) (schemaPlan, error) {
	for _, column := range []string{"ClientSessionID", "TaskInAttemptedAt", "TaskInIdempotent"} {
		if !database.Migrator().HasColumn(&models.TaskRecord{}, column) {
			return schemaPlan{Action: migrationActionIndex}, nil
		}
	}
	if !database.Migrator().HasIndex("task_records", taskSessionIdentityIndex) {
		return schemaPlan{Action: migrationActionIndex}, nil
	}
	return schemaPlan{Action: migrationActionNoop}, nil
}

func ensureTaskSessionIdentity(database *gorm.DB) error {
	for _, column := range []string{"ClientSessionID", "TaskInAttemptedAt", "TaskInIdempotent"} {
		if !database.Migrator().HasColumn(&models.TaskRecord{}, column) {
			if err := database.Migrator().AddColumn(&models.TaskRecord{}, column); err != nil {
				return err
			}
		}
	}
	// Legacy NULL identities remain untouched, including duplicate history.
	return database.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_task_records_user_client_session
		ON task_records(user_id, client_session_id)
		WHERE client_session_id IS NOT NULL AND client_session_id <> ''`).Error
}
