package services

import (
	"fesapp/backend/db"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// TODO remove this, firebase handles the creation or change to log user authenticated with firebase
func CreateUser(id string, email string) (*db.User, error) {
	// Create temp user for validation
	user := db.User{ID: id, Email: email}

	// Validate struct
	if err := validate.Struct(user); err != nil {
		return nil, err
	}
	
	// TODO add update as well the time when the user logs in

	// Save to DB
	result := db.DB.Where("id = ?", id).FirstOrCreate(&user)
	return &user, result.Error
}
