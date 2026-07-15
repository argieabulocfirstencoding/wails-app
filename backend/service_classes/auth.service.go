package service_classes

import (
	"context"
	"fesapp/backend/db/models"
	"fesapp/backend/services"
	"fesapp/backend/types"
	"fesapp/backend/utils"
	"fmt"
	"strings"
)

type AuthService struct {
	SyncService       *SyncService
	UserService       *UserService
	AttendanceService *AttendanceService
	TaskService       *TaskService
	FesClientService  *FesClientService
	Logger            *utils.LoggerService
	ctx               context.Context
}

type authenticatedLogin struct {
	UserID string
	Email  string
}

func NewAuthService(connectivity ...*ConnectivityStateService) *AuthService {
	var connectivityState *ConnectivityStateService
	if len(connectivity) > 0 {
		connectivityState = connectivity[0]
	}
	if connectivityState == nil {
		connectivityState = NewConnectivityStateService()
	}
	SyncService := NewSyncService(connectivityState)

	return &AuthService{
		FesClientService:  NewFesClientService(connectivityState),
		Logger:            utils.NewLoggerService(),
		UserService:       NewUserService(),
		SyncService:       SyncService,
		AttendanceService: NewAttendanceService(SyncService),
		TaskService:       NewTaskService(SyncService, connectivityState),
	}
}

func (a *AuthService) Login(ctx context.Context, email, password string) (*models.Attendance, *models.AuthUser, error) {
	a.ctx = ctx
	a.TaskService.Connect(ctx)

	login, err := a.AttemptLogin(email, password)
	if err != nil {
		return nil, nil, err
	}

	attendance, err := a.AttendanceService.LoginAttendance(login.UserID)
	if err != nil {
		return nil, nil, err
	}
	if attendance == nil {
		return nil, nil, fmt.Errorf("login attendance is nil")
	}

	authUser := &models.AuthUser{
		LocalID: login.UserID,
		Email:   login.Email,
	}

	// NOTE INITIALIZATIONS
	if a.TaskService.Attendance != nil {
		*a.TaskService.Attendance = attendance
	} else {
		a.TaskService.Attendance = &attendance
	}
	a.TaskService.SyncService.SetAuthUser(authUser)

	a.TaskService.SyncService.Attendance = a.TaskService.Attendance
	a.TaskService.SyncService.TaskRecord = &a.TaskService.TaskRecord

	attendance.User.ID = login.UserID
	attendance.User.Email = login.Email

	if err := a.UserService.FetchEmployeeDetails(ctx, &attendance.User); err != nil {
		return nil, nil, err
	}

	// The login identifier may be an FES ID. Use only a validated employee
	// email for the authenticated session and downstream Chrome profile match.
	authUser.Email = resolvedAuthenticatedEmployeeEmail(&attendance.User)

	// go func() {
	// 	a.SyncService.Connect(a.ctx)
	// 	a.SyncService.SyncUser(*authUser)
	// 	// a.SyncService.SyncTasks()

	// }()

	return attendance, authUser, nil
}

func (a *AuthService) Logout() error {
	attendance := a.TaskService.Attendance
	workingTaskID := a.TaskService.WorkingTaskID

	if workingTaskID != nil && *workingTaskID != nil {
		a.TaskService.StopTracking(**workingTaskID)
		a.Logger.Printf("WorkingTaskID: %s", **workingTaskID)
	}

	if attendance != nil && *attendance != nil {
		if err := services.LogoutAttendance(*attendance); err != nil {
			return err
		}

		if err := a.TaskService.SyncService.SyncAttendance(*attendance); err != nil {
			a.Logger.Printf("error syncing attendance on logout: %v", err)
			return fmt.Errorf("logout sync failed: %w", err)
		}
	} else {
		a.Logger.Println("No active AttendanceID")
	}

	return nil
}

//  ======================= PRIVATE METHODS ===========================

func (a *AuthService) AttemptLogin(identifier, password string) (*authenticatedLogin, error) {

	// a.logger.Printlnf("Login Attempt Email: ", email)
	// a.logger.Printlnf("Login Attempt Password: ", a.maskPassword(password))
	// now := time.Now()

	resp, err := a.FesClientService.Request(types.Request{
		Url:    "wf/login",
		Method: "POST",
		Body: map[string]any{
			"email":    identifier,
			"password": password,
			// "timestamp": now.UnixMilli(), // TODO find way to add this
		},
	})

	if err != nil {
		a.Logger.Printlnf("Attempt Login Error	: %v", err)
		return nil, err
	}

	userIDRaw, ok := resp["user_id"]
	if !ok {
		return nil, a.Logger.Error("login response missing user_id")
	}
	userId, ok := userIDRaw.(string)
	if !ok || strings.TrimSpace(userId) == "" {
		return nil, a.Logger.Error("login response user_id has invalid format")
	}

	tokenRaw, ok := resp["token"]
	if !ok {
		return nil, a.Logger.Error("login response missing token")
	}
	token, ok := tokenRaw.(string)
	if !ok || strings.TrimSpace(token) == "" {
		return nil, a.Logger.Error("login response token has invalid format")
	}

	a.TaskService.SyncService.FesClientService.SetApiToken(token)

	authenticatedEmail := authenticatedEmailFromLoginResponse(resp, identifier)
	_, create_user_err := a.UserService.CreateUser(userId, authenticatedEmail)
	if create_user_err != nil {
		a.Logger.Printlnf("Error creating user: %v", create_user_err)
	}

	return &authenticatedLogin{UserID: userId, Email: authenticatedEmail}, nil
}

// authenticatedEmailFromLoginResponse prefers the identity confirmed by the
// FES login endpoint. Falling back to the supplied identifier preserves normal
// email login while preventing an FES ID from being treated as an email.
func authenticatedEmailFromLoginResponse(response map[string]any, identifier string) string {
	if email := extractEmployeeEmail(response, nil); email != "" {
		return email
	}

	return normalizedEmployeeEmail(identifier)
}

func (a *AuthService) maskPassword(p string) string {
	n := len(p)
	if n <= 2 {
		return "****" // Too short to show edges, just mask fully
	}
	// Keeps first char, middle is 'x', keeps last char
	return string(p[0]) + strings.Repeat("x", n-2) + string(p[n-1])
}
