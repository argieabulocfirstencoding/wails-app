package db

import "gorm.io/gorm"

var schemaIndexDefinitions = map[string][]string{
	"attendances": {
		"idx_attendances_deleted_at",
		"idx_attendances_user_time_in",
		"idx_attendances_sync_status",
	},
	"task_records": {
		"idx_task_records_user_attendance_ended",
		"idx_task_records_attendance_sync_started",
		"idx_task_records_task_started",
	},
	"screenshots": {
		"idx_screenshots_task_record_taken",
		"idx_screenshots_task_taken",
	},
}

func planIndexesMigration(database *gorm.DB) (schemaPlan, error) {
	for table, indexes := range schemaIndexDefinitions {
		for _, index := range indexes {
			if !database.Migrator().HasIndex(table, index) {
				return schemaPlan{Action: migrationActionIndex}, nil
			}
		}
	}

	return schemaPlan{Action: migrationActionNoop}, nil
}

func ensureSchemaIndexes(tx *gorm.DB) error {
	statements := []string{
		"CREATE INDEX IF NOT EXISTS idx_attendances_deleted_at ON attendances(deleted_at)",
		"CREATE INDEX IF NOT EXISTS idx_attendances_user_time_in ON attendances(user_id, time_in)",
		"CREATE INDEX IF NOT EXISTS idx_attendances_sync_status ON attendances(sync_status)",
		"CREATE INDEX IF NOT EXISTS idx_task_records_user_attendance_ended ON task_records(user_id, attendance_id, ended_at)",
		"CREATE INDEX IF NOT EXISTS idx_task_records_attendance_sync_started ON task_records(attendance_id, sync_status, started_at)",
		"CREATE INDEX IF NOT EXISTS idx_task_records_task_started ON task_records(task_id, started_at)",
		"CREATE INDEX IF NOT EXISTS idx_screenshots_task_record_taken ON screenshots(task_record_id, taken_at)",
		"CREATE INDEX IF NOT EXISTS idx_screenshots_task_taken ON screenshots(task_id, taken_at)",
	}

	for _, stmt := range statements {
		if err := tx.Exec(stmt).Error; err != nil {
			return err
		}
	}

	return nil
}
