package db

import "gorm.io/gorm"

func needsMigrationBackup(database *gorm.DB) (bool, error) {
	if database == nil {
		return false, nil
	}

	if usersSchemaNeedsRepair(database) {
		return true, nil
	}

	taskRecordsNeedsRepair, err := taskRecordsSchemaNeedsRepair(database)
	if err != nil {
		return false, err
	}
	if taskRecordsNeedsRepair {
		return true, nil
	}

	taskTablesNeedRepair, err := taskAndScreenshotSchemasNeedRepair(database)
	if err != nil {
		return false, err
	}
	if taskTablesNeedRepair {
		return true, nil
	}

	if !database.Migrator().HasTable("schema_migrations") {
		return true, nil
	}

	current, err := currentSchemaVersion(database)
	if err != nil {
		return false, err
	}

	return current < latestSchemaVersion, nil
}
