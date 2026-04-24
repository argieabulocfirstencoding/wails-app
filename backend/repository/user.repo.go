package repository

import (
	"fesapp/backend/db"
	"fesapp/backend/db/models"

	"github.com/go-playground/validator/v10"
)

type UserRepository struct {
	validate *validator.Validate
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		validate: validator.New(),
	}
}

func (u *UserRepository) CreateUser(id string, email string) (*models.User, error) {
	// Create temp user for validation
	user := db.User{ID: id, Email: email}

	// Validate struct
	if err := u.validate.Struct(user); err != nil {
		return nil, err
	}

	// Save to DB
	result := db.DB.Where("id = ?", id).FirstOrCreate(&user)
	return &user, result.Error
}

func (u *UserRepository) GetUserFromId(id string) (*models.User, error) {
	var user db.User

	if err := db.DB.Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}
