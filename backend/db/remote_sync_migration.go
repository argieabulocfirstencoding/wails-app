package db

import (
	"fmt"

	"fesapp/backend/db/models"
	"gorm.io/gorm"
)

func planRemoteSyncMetadataMigration(database *gorm.DB) (schemaPlan, error) {
	if database == nil {
		return schemaPlan{}, fmt.Errorf("nil database")
	}

	if !database.Migrator().HasTable("attendances") || !database.Migrator().HasTable("task_records") {
		return schemaPlan{Action: migrationActionNoop}, nil
	}

	if !database.Migrator().HasColumn(&models.Attendance{}, "RemoteModifiedAt") ||
		!database.Migrator().HasColumn(&models.TaskRecord{}, "RemoteModifiedAt") {
		return schemaPlan{Action: migrationActionIndex}, nil
	}

	return schemaPlan{Action: migrationActionNoop}, nil
}

func ensureRemoteSyncMetadata(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("nil database")
	}

	if database.Migrator().HasTable("attendances") && !database.Migrator().HasColumn(&models.Attendance{}, "RemoteModifiedAt") {
		if err := database.Migrator().AddColumn(&models.Attendance{}, "RemoteModifiedAt"); err != nil {
			return fmt.Errorf("add attendances.remote_modified_at: %w", err)
		}
	}
	if database.Migrator().HasTable("task_records") && !database.Migrator().HasColumn(&models.TaskRecord{}, "RemoteModifiedAt") {
		if err := database.Migrator().AddColumn(&models.TaskRecord{}, "RemoteModifiedAt"); err != nil {
			return fmt.Errorf("add task_records.remote_modified_at: %w", err)
		}
	}

	for _, statement := range []string{
		"CREATE INDEX IF NOT EXISTS idx_attendances_user_login_logout_id ON attendances(user_id, login_logout_id)",
		"CREATE INDEX IF NOT EXISTS idx_task_records_user_productivity ON task_records(user_id, productivity)",
	} {
		if err := database.Exec(statement).Error; err != nil {
			return fmt.Errorf("create remote sync index: %w", err)
		}
	}

	return nil
}
