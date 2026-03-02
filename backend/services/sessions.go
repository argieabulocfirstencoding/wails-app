package services

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fmt"

	"gorm.io/gorm"
)

func AutoCloseStuckSessions(userID string) error {
	// cutoff := time.Now().Add(-30 * time.Minute)

	result := db.DB.Model(&models.TaskRecord{}).
		Where("user_id = ? AND ended_at IS NULL AND synced_at IS NOT NULL", userID).
		Updates(map[string]interface{}{
			"ended_at": gorm.Expr("synced_at"),
			"duration": gorm.Expr(`
            strftime('%s', substr(synced_at, 1, 19)) -
            strftime('%s', substr(started_at, 1, 19))
        `),
		})

	if result.Error != nil {
		fmt.Printf("Error on closing session %v", result.Error)
		return result.Error
	}

	// TODO add gap calculator here as well

	fmt.Printf("Auto-closed %d unfinished sessions for user %s", result.RowsAffected, userID)
	return nil
}
