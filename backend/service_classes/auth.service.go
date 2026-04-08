package service_classes

import (
	"fesapp/backend/db/models"
	"fesapp/backend/types"
	"fesapp/backend/utils"
	"strings"
)

type AuthService struct {
	SyncService       *SyncService
	userService       *UserService
	attendanceService *AttendanceService
	fesClient         *FesClientService
	logger            *utils.LoggerService
}

func NewAuthService() *AuthService {
	return &AuthService{
		SyncService:       NewFesSyncService(),
		fesClient:         NewFesClientService(),
		logger:            utils.NewLoggerService(),
		userService:       NewUserService(),
		attendanceService: NewAttendanceService(),
	}
}

func (a *AuthService) Login(email, password string) (*models.Attendance, *models.AuthUser, error) {

	userId, err := a.AttemptLogin(email, password)
	if err != nil {
		return nil, nil, err
	}

	attendance, err := a.attendanceService.LoginAttendance(*userId)
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

func (a *AuthService) AttemptLogin(email, password string) (*string, error) {

	// a.logger.Printlnf("Login Attempt Email: ", email)
	// a.logger.Printlnf("Login Attempt Password: ", a.maskPassword(password))
	// now := time.Now()

	resp, err := a.fesClient.Request(types.Request{
		Pathname: "login",
		ReqType:  "POST",
		Body: map[string]any{
			"email":    email,
			"password": password,
			// "timestamp": now.UnixMilli(), // TODO find way to add this
		},
	})

	if err != nil {
		a.logger.Printlnf("Attempt Login Error: %v", err)
		return nil, err
	}

	a.logger.Printlnf("Login Response: %v", resp)

	result, ok := resp["response"].(map[string]any)
	if !ok {
		a.logger.Println("Error on parsing response")
		return nil, err
	}

	userId := result["user_id"].(string)

	_, create_user_err := a.userService.CreateUser(userId, email)
	if create_user_err != nil {
		a.logger.Printlnf("Error creating user: %v", create_user_err)
	}

	return &userId, nil
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

	_, ok := resp["response"].(map[string]any)
	if !ok {
		a.logger.Println("Error on parsing response")
		return nil, err
	}

	// userId := result["user_id"].(string)

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
