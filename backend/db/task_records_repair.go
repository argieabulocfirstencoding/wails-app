package db

import (
	"fmt"

	"gorm.io/gorm"
)

func repairTaskRecordsSchema(database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("nil database")
	}

	if err := database.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
		return err
	}
	defer database.Exec("PRAGMA foreign_keys = ON")

	return database.Transaction(func(tx *gorm.DB) error {
		if tx.Migrator().HasTable("task_records") {
			if err := tx.Exec("DROP TABLE IF EXISTS task_records_legacy").Error; err != nil {
				return err
			}
			if err := tx.Exec("ALTER TABLE task_records RENAME TO task_records_legacy").Error; err != nil {
				return err
			}
		}

		createSQL := `
CREATE TABLE task_records (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL,
	attendance_id INTEGER NOT NULL,
	task_id TEXT NOT NULL,
	started_at DATETIME NOT NULL,
	padagan INTEGER NOT NULL DEFAULT 0,
	notes TEXT NOT NULL DEFAULT '',
	ended_at DATETIME,
	duration INTEGER NOT NULL DEFAULT 0,
	synced_at DATETIME,
	productivity TEXT,
	sync_status TEXT NOT NULL DEFAULT 'pending',
	attempt_count INTEGER NOT NULL DEFAULT 0,
	last_error TEXT NOT NULL DEFAULT '',
	last_attempt_at DATETIME,
	gap INTEGER NOT NULL DEFAULT 0,
	FOREIGN KEY(user_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
	FOREIGN KEY(attendance_id) REFERENCES attendances(id) ON UPDATE CASCADE ON DELETE CASCADE,
	FOREIGN KEY(task_id) REFERENCES tasks(task_id) ON UPDATE CASCADE ON DELETE CASCADE
);`

		if err := tx.Exec(createSQL).Error; err != nil {
			return err
		}

		if !tx.Migrator().HasTable("task_records_legacy") {
			return nil
		}

		copySQL := `
INSERT INTO task_records (
	id,
	user_id,
	attendance_id,
	task_id,
	started_at,
	padagan,
	notes,
	ended_at,
	duration,
	synced_at,
	productivity,
	sync_status,
	attempt_count,
	last_error,
	last_attempt_at,
	gap
)
SELECT
	tr.id,
	COALESCE(NULLIF(TRIM(tr.user_id), ''), a.user_id) AS user_id,
	tr.attendance_id,
	tr.task_id,
	COALESCE(tr.started_at, CURRENT_TIMESTAMP),
	COALESCE(tr.padagan, 0),
	COALESCE(tr.notes, ''),
	tr.ended_at,
	COALESCE(tr.duration, 0),
	tr.synced_at,
	tr.productivity,
	COALESCE(NULLIF(TRIM(tr.sync_status), ''), 'pending'),
	COALESCE(tr.attempt_count, 0),
	COALESCE(tr.last_error, ''),
	tr.last_attempt_at,
	COALESCE(tr.gap, 0)
FROM task_records_legacy AS tr
LEFT JOIN attendances AS a ON a.id = tr.attendance_id
INNER JOIN tasks AS t ON t.task_id = tr.task_id
WHERE COALESCE(NULLIF(TRIM(tr.user_id), ''), a.user_id) IS NOT NULL;`

		if err := tx.Exec(copySQL).Error; err != nil {
			return err
		}

		if err := tx.Exec("DROP TABLE task_records_legacy").Error; err != nil {
			return err
		}

		return nil
	})
}
