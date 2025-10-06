package services

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"log"
	"time"

	"gorm.io/gorm"
)

func AutoCloseStuckSessions(userID string) error {
	cutoff := time.Now().Add(-30 * time.Minute)

	result := db.DB.Model(&models.TaskRecord{}).
		Where("user_id = ? AND ended_at IS NULL AND synced_at IS NOT NULL AND synced_at < ?", userID, cutoff).
		Updates(map[string]interface{}{
			"ended_at":   gorm.Expr("synced_at"),
			"updated_at": time.Now(),
		})

	if result.Error != nil {
		return result.Error
	}

	log.Printf("Auto-closed %d unfinished sessions for user %s", result.RowsAffected, userID)
	return nil
}
