package service_classes

import (
	"context"
	"fesapp/backend/db/models"
	"fesapp/backend/types"
	"fesapp/backend/utils"
	"fmt"
	"strconv"
	"strings"
	"time"
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
	UserID     string
	Email      string
	EmployeeID string
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
	if err := a.UserService.SaveLoginEmployeeID(login.UserID, login.EmployeeID); err != nil {
		return nil, nil, fmt.Errorf("save login employee ID: %w", err)
	}
	attendance.User.EmployeeId = login.EmployeeID

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
	return a.LogoutAt(time.Now())
}

// LogoutAt makes local task and attendance closure durable before attempting
// remote sync. A Bubble outage must not prevent an employee from ending a
// shift or retaining its retryable local records.
func (a *AuthService) LogoutAt(endedAt time.Time) error {
	attendance := a.TaskService.Attendance

	if attendance != nil && *attendance != nil {
		record, err := a.TaskService.TaskRepository.GetOpenTaskRecordForAttendance((*attendance).ID)
		if err != nil {
			return fmt.Errorf("find active task during logout: %w", err)
		}
		if record != nil {
			if _, err := a.TaskService.StopTrackingRecordAt(record.ID, endedAt); err != nil {
				return err
			}
			a.Logger.Printf("Stopping task record: %d", record.ID)
		}
	}

	if attendance != nil && *attendance != nil {
		if err := a.AttendanceService.LogoutAttendanceAt(*attendance, endedAt); err != nil {
			return err
		}

		if err := a.TaskService.SyncService.SyncAttendance(*attendance); err != nil {
			a.Logger.Printf("error syncing attendance on logout: %v", err)
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

	expiresIn, err := bubbleSessionDuration(resp["expires"])
	if err != nil {
		return nil, err
	}
	if err := a.TaskService.SyncService.FesClientService.SetBubbleSession(userId, token, expiresIn); err != nil {
		return nil, err
	}
	a.UserService.SetActiveBubbleSessionUser(userId)

	authenticatedEmail := authenticatedEmailFromLoginResponse(resp, identifier)
	_, create_user_err := a.UserService.CreateUser(userId, authenticatedEmail)
	if create_user_err != nil {
		a.Logger.Printlnf("Error creating user: %v", create_user_err)
	}

	return &authenticatedLogin{
		UserID:     userId,
		Email:      authenticatedEmail,
		EmployeeID: loginEmployeeIDFromResponse(resp),
	}, nil
}

func bubbleSessionDuration(value any) (time.Duration, error) {
	var seconds int64
	switch v := value.(type) {
	case float64:
		seconds = int64(v)
		if v != float64(seconds) {
			return 0, fmt.Errorf("login response expires has invalid format")
		}
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("login response expires has invalid format")
		}
		seconds = parsed
	default:
		return 0, fmt.Errorf("login response missing expires")
	}

	if seconds <= 0 || seconds > int64((366*24*time.Hour)/time.Second) {
		return 0, fmt.Errorf("login response expires is outside the allowed range")
	}
	return time.Duration(seconds) * time.Second, nil
}

// loginEmployeeIDFromResponse extracts the Employee thing ID returned by the
// authenticated login workflow. Unlike the profile lookup, this value is
// available even when wf/employee_details returns an empty employee object.
func loginEmployeeIDFromResponse(response map[string]any) string {
	if response == nil {
		return ""
	}

	employeeID, ok := response["employee_id"].(string)
	if !ok {
		return ""
	}

	return strings.TrimSpace(employeeID)
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
