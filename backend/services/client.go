package services

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"

	"gorm.io/gorm/clause"
)

func GetClientFromId(id string) (*models.Client, error) {
	// TODO get client from id

	return nil, nil
}

func BulkCreateClients(clients []models.Client) error {
	if len(clients) == 0 {
		return nil
	}

	return db.DB.Clauses(clause.OnConflict{
		UpdateAll: true, // updates all fields except primary keys
	}).Create(&clients).Error
}
