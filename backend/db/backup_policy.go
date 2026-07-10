package db

import "gorm.io/gorm"

func needsMigrationBackup(database *gorm.DB) (bool, error) {
	if database == nil {
		return false, nil
	}

	plans, err := pendingMigrationPlans(database, migrationCatalog())
	if err != nil {
		return false, err
	}
	for _, plan := range plans {
		if plan.RequiresBackup {
			return true, nil
		}
	}
	return false, nil
}
