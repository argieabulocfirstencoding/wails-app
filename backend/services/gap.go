package services

import (
	"errors"
	"fesapp/backend/db/models"
	"fmt"
	"time"

	"gorm.io/gorm"
)

func CalculateGap(tr *models.TaskRecord, now time.Time) error {
	var gap time.Duration

	taskRecord, err := GetLastTaskRecordFromAttendance(tr.UserID, tr.AttendanceID)
	fmt.Printf("taskrecord ended_at: %s", taskRecord.EndedAt)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fmt.Print("not found so 0")
			return nil
		} else {
			return err
		}
	}

	gap = now.Sub(*taskRecord.EndedAt)
	fmt.Printf("gap: %d", gap)
	
	tr.Gap = int32(gap / time.Second)
	return nil
}
