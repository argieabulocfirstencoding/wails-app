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

func NewAuthService() *AuthService {
	SyncService := NewSyncService()

	return &AuthService{
		FesClientService:  NewFesClientService(),
		Logger:            utils.NewLoggerService(),
		UserService:       NewUserService(),
		SyncService:       SyncService,
		AttendanceService: NewAttendanceService(SyncService),
		TaskService:       NewTaskService(SyncService),
	}
}

func (a *AuthService) Login(ctx context.Context, email, password string) (*models.Attendance, *models.AuthUser, error) {
	a.ctx = ctx
	a.TaskService.Connect(ctx)

	userId, err := a.AttemptLogin(email, password)
	if err != nil {
		return nil, nil, err
	}

	attendance, err := a.AttendanceService.LoginAttendance(*userId)
	if err != nil {
		return nil, nil, err
	}

	authUser := &models.AuthUser{
		LocalID: *userId,
		Email:   email,
	}

	// NOTE INITIALIZATIONS
	a.TaskService.Attendance = &attendance
	a.TaskService.SyncService.AuthUser = authUser

	a.TaskService.SyncService.Attendance = a.TaskService.Attendance
	a.TaskService.SyncService.TaskRecord = &a.TaskService.TaskRecord

	if err := a.TaskService.SyncService.SyncAttendance(attendance); err != nil {
		a.Logger.Printf("error syncing attendance on login")
	}

	attendance.User.ID = *userId
	attendance.User.Email = email

	if err := a.UserService.FetchEmployeeDetails(ctx, &attendance.User); err != nil {
		return nil, nil, err
	}

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

	if attendance != nil {
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

func (a *AuthService) AttemptLogin(email, password string) (*string, error) {

	// a.logger.Printlnf("Login Attempt Email: ", email)
	// a.logger.Printlnf("Login Attempt Password: ", a.maskPassword(password))
	// now := time.Now()

	resp, err := a.FesClientService.Request(types.Request{
		Url:    "wf/login",
		Method: "POST",
		Body: map[string]any{
			"email":    email,
			"password": password,
			// "timestamp": now.UnixMilli(), // TODO find way to add this
		},
	})

	if err != nil {
		a.Logger.Printlnf("Attempt Login Error	: %v", err)
		return nil, err
	}

	userId := resp["user_id"].(string)
	token := resp["token"].(string)

	a.TaskService.SyncService.FesClientService.SetApiToken(token)

	_, create_user_err := a.UserService.CreateUser(userId, email)
	if create_user_err != nil {
		a.Logger.Printlnf("Error creating user: %v", create_user_err)
	}

	return &userId, nil
}

func (a *AuthService) maskPassword(p string) string {
	n := len(p)
	if n <= 2 {
		return "****" // Too short to show edges, just mask fully
	}
	// Keeps first char, middle is 'x', keeps last char
	return string(p[0]) + strings.Repeat("x", n-2) + string(p[n-1])
}
