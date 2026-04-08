package service_classes

import (
	"fesapp/backend/db/models"
	"fesapp/backend/types"
)

type AuthService struct {
	SyncService *FesSyncService
	fesClient   *FesClientService
	logger      *LoggerService
	baseUrl     string
}

func NewAuthService() *AuthService {
	return &AuthService{
		SyncService: NewFesSyncService(),
		fesClient:   NewFesClientService(),
		logger:      NewLoggerService(),
		baseUrl:     "https://firstencoding.app/api/1.1/wf/",
	}
}

func (a *AuthService) Login(email, password string) (*models.Attendance, *models.AuthUser, error) {

	attendance, err := a.AttemptLogin(email, password)
	if err != nil {
		return nil, nil, err
	}

	authUser, err := a.getAuthUser(email)
	if err != nil {
		return nil, nil, err
	}

	return attendance, authUser, nil
}

func (a *AuthService) Logout(email string) error {
	return nil
}

//  ======================= PRIVATE METHODS ===========================

func (a *AuthService) AttemptLogin(email, password string) (*models.Attendance, error) {

	resp, err := a.fesClient.Request(types.Request{
		Pathname: "login",
		ReqType:  "POST",
		Body: map[string]any{
			"email":    email,
			"password": password,
		},
	})
	if err != nil {
		return nil, err
	}

	a.logger.Printf("Login Response: %v", resp)

	// TODO add attempt login logic to fes app
	return nil, nil
}

func (a *AuthService) getAuthUser(email string) (*models.AuthUser, error) {
	// TODO get auth user logic from fes app

	resp, err := a.fesClient.Request(types.Request{
		Pathname: "employee_details",
		ReqType:  "POST",
		Body: map[string]any{
			"email": email,
		},
	})
	if err != nil {
		return nil, err
	}

	a.logger.Printf("Auth User Response: %v", resp)

	return nil, nil
}
