package db

import (
	"fmt"

	"fesapp/backend/db/models"
	"gorm.io/gorm"
)

func planTaskRecoveryAuditMigration(database *gorm.DB) (schemaPlan, error) {
	if database == nil {
		return schemaPlan{}, fmt.Errorf("nil database")
	}
	if database.Migrator().HasTable(&models.TaskRecoveryAudit{}) {
		return schemaPlan{Action: migrationActionNoop}, nil
	}
	return schemaPlan{Action: migrationActionCreate}, nil
}

func ensureTaskRecoveryAuditSchema(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("nil database")
	}
	return database.AutoMigrate(&models.TaskRecoveryAudit{})
}
