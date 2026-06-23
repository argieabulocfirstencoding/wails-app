package db

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const latestSchemaVersion = 3

type schemaMigration struct {
	Version    int       `gorm:"primaryKey"`
	Name       string    `gorm:"not null"`
	AppliedAt  time.Time `gorm:"not null"`
	Successful bool      `gorm:"not null;default:false"`
}

type migrationStep struct {
	version       int
	name          string
	up            func(tx *gorm.DB) error
	down          func(tx *gorm.DB) error
	transactional bool
}

func migrationPlan() []migrationStep {
	return []migrationStep{
		{
			version:       1,
			name:          "repair_users_schema",
			transactional: true,
			up: func(tx *gorm.DB) error {
				return repairUsersSchema(tx)
			},
			down: func(tx *gorm.DB) error {
				return nil
			},
		},
		{
			version:       2,
			name:          "hot_query_indexes",
			transactional: true,
			up:            applyHotQueryIndexes,
			down:          dropHotQueryIndexes,
		},
		{
			version:       3,
			name:          "repair_task_and_screenshot_foreign_keys",
			transactional: false,
			up:            repairTaskAndScreenshotSchemas,
			down: func(tx *gorm.DB) error {
				return nil
			},
		},
	}
}

func migrateSchema(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("nil database")
	}

	if err := database.AutoMigrate(&schemaMigration{}); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	current, err := currentSchemaVersion(database)
	if err != nil {
		return err
	}
	if current > latestSchemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d", current, latestSchemaVersion)
	}

	for _, step := range migrationPlan() {
		if step.version <= current {
			continue
		}
		if err := runMigrationStep(database, step); err != nil {
			return err
		}
	}

	return nil
}

func currentSchemaVersion(database *gorm.DB) (int, error) {
	var row schemaMigration
	err := database.
		Where("successful = ?", true).
		Order("version DESC").
		First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, fmt.Errorf("resolve current schema version: %w", err)
	}
	return row.Version, nil
}

func runMigrationStep(database *gorm.DB, step migrationStep) error {
	if !step.transactional {
		return runNonTransactionalMigrationStep(database, step)
	}

	tx := database.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	appliedAt := time.Now().UTC()
	record := schemaMigration{
		Version:    step.version,
		Name:       step.name,
		AppliedAt:  appliedAt,
		Successful: false,
	}
	if err := tx.Create(&record).Error; err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("create migration record v%d: %w", step.version, err)
	}

	if err := step.up(tx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply migration v%d (%s): %w", step.version, step.name, err)
	}

	if err := tx.Model(&schemaMigration{}).
		Where("version = ?", step.version).
		Update("successful", true).Error; err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("mark migration v%d successful: %w", step.version, err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit migration v%d: %w", step.version, err)
	}

	return nil
}

func runNonTransactionalMigrationStep(database *gorm.DB, step migrationStep) error {
	record := schemaMigration{
		Version:    step.version,
		Name:       step.name,
		AppliedAt:  time.Now().UTC(),
		Successful: false,
	}

	if err := database.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "version"}},
		UpdateAll: true,
	}).Create(&record).Error; err != nil {
		return fmt.Errorf("create migration record v%d: %w", step.version, err)
	}

	if err := step.up(database); err != nil {
		return fmt.Errorf("apply migration v%d (%s): %w", step.version, step.name, err)
	}

	if err := database.Model(&schemaMigration{}).
		Where("version = ?", step.version).
		Update("successful", true).Error; err != nil {
		return fmt.Errorf("mark migration v%d successful: %w", step.version, err)
	}

	return nil
}
