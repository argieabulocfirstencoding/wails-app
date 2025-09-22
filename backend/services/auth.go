package services

import (
	"errors"
	"fesapp/backend/db"

	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/bcrypt"
)

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
	keyring.Set("fesapp", user.Name, user.Password)

	return &user, nil
}
