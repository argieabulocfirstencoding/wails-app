package services

import "fesapp/backend/db"

func AddMemo(task_record_id uint, memo, notes string) error {
	task_record, err := GetTaskRecordFromID(task_record_id)
	if err != nil {
		return err
	}
	task_record.Memo = memo
	task_record.Notes = notes
	db.DB.Save(&task_record)

	return nil
}
