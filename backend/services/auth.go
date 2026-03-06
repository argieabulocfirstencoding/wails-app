package services

import (
	"errors"
	"fesapp/backend/db"
)

// NOTE for local login
var OfflineLogin = func(email string) (*db.User, error) {
	var user db.User

	result := db.DB.Where("email = ?", email).First(&user)
	if result.Error != nil {
		return nil, errors.New("no account with that email found")
	}

	return &user, nil
}
