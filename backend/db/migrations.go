package db

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

const gormMigrationsTable = "gorm_migrations"

type migrationAction string

const (
	migrationActionNoop    migrationAction = "noop"
	migrationActionCreate  migrationAction = "create"
	migrationActionRebuild migrationAction = "rebuild"
	migrationActionIndex   migrationAction = "index"
)

type schemaPlan struct {
	Action         migrationAction
	RequiresBackup bool
}

type catalogMigration struct {
	ID          string
	Description string
	Plan        func(*gorm.DB) (schemaPlan, error)
	Migrate     func(*gorm.DB) error
	Rollback    gormigrate.RollbackFunc
}

func runMigrations(database *gorm.DB, dbPath string) error {
	if database == nil {
		return fmt.Errorf("nil database")
	}

	catalog := migrationCatalog()
	pendingPlans, err := pendingMigrationPlans(database, catalog)
	if err != nil {
		return fmt.Errorf("resolve pending migrations: %w", err)
	}

	backupPath, err := maybeCreateMigrationBackup(database, dbPath, pendingPlans)
	if err != nil {
		return fmt.Errorf("failed to create pre-migration backup: %w", err)
	}

	migrator := gormigrate.New(database, migrationOptions(), toGormigrateMigrations(catalog))
	if err := migrator.Migrate(); err != nil {
		if backupPath != "" {
			return fmt.Errorf("failed to apply schema migrations: %w (rollback backup: %s)", err, backupPath)
		}
		return fmt.Errorf("failed to apply schema migrations: %w", err)
	}

	return nil
}

func migrationCatalog() []catalogMigration {
	return []catalogMigration{
		{
			ID:          "20260701_001_users_canonical",
			Description: "create or rebuild users into the canonical schema",
			Plan:        planUsersMigration,
			Migrate:     ensureUsersSchema,
		},
		{
			ID:          "20260701_002_clients_canonical",
			Description: "create or rebuild clients into the canonical schema",
			Plan:        planClientsMigration,
			Migrate:     ensureClientsSchema,
		},
		{
			ID:          "20260701_003_tasks_canonical",
			Description: "create or rebuild tasks into the canonical schema",
			Plan:        planTasksMigration,
			Migrate:     ensureTasksSchema,
		},
		{
			ID:          "20260701_004_attendances_canonical",
			Description: "create or rebuild attendances into the canonical schema",
			Plan:        planAttendancesMigration,
			Migrate:     ensureAttendancesSchema,
		},
		{
			ID:          "20260701_005_task_records_canonical",
			Description: "create or rebuild task_records into the canonical schema",
			Plan:        planTaskRecordsMigration,
			Migrate:     ensureTaskRecordsSchema,
		},
		{
			ID:          "20260701_006_screenshots_canonical",
			Description: "create or rebuild screenshots into the canonical schema",
			Plan:        planScreenshotsMigration,
			Migrate:     ensureScreenshotsSchema,
		},
		{
			ID:          "20260701_007_indexes_canonical",
			Description: "ensure hot query indexes exist",
			Plan:        planIndexesMigration,
			Migrate:     ensureSchemaIndexes,
		},
		{
			ID:          "20260715_008_remote_sync_metadata",
			Description: "add Bubble reconciliation metadata",
			Plan:        planRemoteSyncMetadataMigration,
			Migrate:     ensureRemoteSyncMetadata,
		},
		{
			ID:          "20260929_009_task_session_identity",
			Description: "add durable task session identities and task-in dispatch metadata",
			Plan:        planTaskSessionIdentityMigration,
			Migrate:     ensureTaskSessionIdentity,
		},
	}
}

func migrationOptions() *gormigrate.Options {
	return &gormigrate.Options{
		TableName:                 gormMigrationsTable,
		IDColumnName:              "id",
		IDColumnSize:              255,
		UseTransaction:            false,
		ValidateUnknownMigrations: true,
	}
}

func toGormigrateMigrations(catalog []catalogMigration) []*gormigrate.Migration {
	migrations := make([]*gormigrate.Migration, 0, len(catalog))
	for _, item := range catalog {
		migration := item
		migrations = append(migrations, &gormigrate.Migration{
			ID: migration.ID,
			Migrate: func(tx *gorm.DB) error {
				if err := migration.Migrate(tx); err != nil {
					return fmt.Errorf("%s: %w", migration.Description, err)
				}
				return nil
			},
			Rollback: migration.Rollback,
		})
	}
	return migrations
}

func pendingMigrationPlans(database *gorm.DB, catalog []catalogMigration) ([]schemaPlan, error) {
	appliedIDs, err := appliedMigrationIDs(database)
	if err != nil {
		return nil, err
	}

	plans := make([]schemaPlan, 0, len(catalog))
	for _, item := range catalog {
		if _, ok := appliedIDs[item.ID]; ok {
			continue
		}

		plan, err := item.Plan(database)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", item.ID, err)
		}
		plans = append(plans, plan)
	}

	return plans, nil
}

func appliedMigrationIDs(database *gorm.DB) (map[string]struct{}, error) {
	if !database.Migrator().HasTable(gormMigrationsTable) {
		return map[string]struct{}{}, nil
	}

	var ids []string
	if err := database.Table(gormMigrationsTable).Select("id").Scan(&ids).Error; err != nil {
		return nil, err
	}

	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out, nil
}

func maybeCreateMigrationBackup(database *gorm.DB, dbPath string, plans []schemaPlan) (string, error) {
	for _, plan := range plans {
		if plan.RequiresBackup {
			return createMigrationBackup(database, dbPath)
		}
	}
	return "", nil
}
