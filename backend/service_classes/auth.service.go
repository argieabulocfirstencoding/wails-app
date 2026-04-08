package service_classes

import (
	"fesapp/backend/db/models"
	"fesapp/backend/types"
	"strings"
)

type AuthService struct {
	SyncService *SyncService
	fesClient   *FesClientService
	logger      *LoggerService
}

func NewAuthService() *AuthService {
	return &AuthService{
		SyncService: NewFesSyncService(),
		fesClient:   NewFesClientService(),
		logger:      NewLoggerService(),
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

	a.logger.Printlnf("Login Attempt Email: ", email)
	a.logger.Printlnf("Login Attempt Password: ", a.maskPassword(password))

	resp, err := a.fesClient.Request(types.Request{
		Pathname: "login",
		ReqType:  "POST",
		Body: map[string]any{
			"email":    email,
			"password": password,
		},
	})

	if err != nil {
		a.logger.Printlnf("Attempt Login Error: %v", err)
		return nil, err
	}

	a.logger.Printlnf("Login Response: %v", resp)

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
		a.logger.Printlnf("Auth User Error: %v", err)
		return nil, err
	}

	a.logger.Printlnf("Auth User Response: %v", resp)

	return nil, nil
}

func (a *AuthService) maskPassword(p string) string {
	n := len(p)
	if n <= 2 {
		return "****" // Too short to show edges, just mask fully
	}
	// Keeps first char, middle is 'x', keeps last char
	return string(p[0]) + strings.Repeat("x", n-2) + string(p[n-1])
}
