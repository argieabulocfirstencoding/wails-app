package services

import (
	"fesapp/backend/db"

	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
)

var validate = validator.New()

func CreateUser(name, email, password string) (*db.User, error) {
	// Create temp user for validation
	user := db.User{Name: name, Email: email, Password: password}

	// Validate struct
	if err := validate.Struct(user); err != nil {
		return nil, err
	}

	// Hash password before saving
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user.Password = string(hashedPassword)

	// Save to DB
	result := db.DB.Create(&user)
	return &user, result.Error
}
