package services

import (
	"errors"
	"fesapp/backend/db"

	"golang.org/x/crypto/bcrypt"
)

type AuthUser struct {
	LocalID string `json:"localId"` // uid
	Email   string  `json:"email"`
	// TODO add profile
}

// NOTE for local login
func Login(email, password string) (*db.User, error) {

	// TODO add api token check
	var user db.User

	result := db.DB.Where("email = ?", email).First(&user)
	if result.Error != nil {
		return nil, errors.New("invalid email or password")
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	return &user, nil
}
