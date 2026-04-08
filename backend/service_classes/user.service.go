package service_classes

import (
	"fesapp/backend/db/models"
	"fesapp/backend/repository"
)

type UserService struct {
	userRepo *repository.UserRepository
}

func NewUserService() *UserService {
	return &UserService{
		userRepo: &repository.UserRepository{},
	}
}

func (u *UserService) GetUserFromId(id string) (*models.User, error) {
	return u.userRepo.GetUserFromId(id)
}

func (u *UserService) CreateUser(id string, email string) (*models.User, error) {
	return u.userRepo.CreateUser(id, email)
}
