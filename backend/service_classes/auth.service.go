package service_classes

import (
	"context"
	"fesapp/backend/db/models"
	"fesapp/backend/services"
	"fesapp/backend/types"
	"fesapp/backend/utils"
	"fmt"
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

	userId, err := a.AttemptLogin(email, password)
	if err != nil {
		return nil, nil, err
	}

	attendance, err := a.AttendanceService.LoginAttendance(*userId)
	if err != nil {
		return nil, nil, err
	}
	if attendance == nil {
		return nil, nil, fmt.Errorf("login attendance is nil")
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
	userID := ""
	attendanceID := uint(0)
	if attendance != nil && *attendance != nil {
		userID = (*attendance).UserID
		attendanceID = (*attendance).ID
	}
	a.Logger.Infof("op=logout_local_start user_id=%s attendance_id=%d has_working_task=%t", userID, attendanceID, workingTaskID != nil && *workingTaskID != nil)

	if workingTaskID != nil && *workingTaskID != nil {
		taskID := **workingTaskID
		if err := a.TaskService.StopTracking(taskID); err != nil {
			a.Logger.Warnf("op=logout_local_task_stop_failed user_id=%s attendance_id=%d task_id=%s err=%v", userID, attendanceID, taskID, err)
			return err
		}
		a.Logger.Printf("WorkingTaskID: %s", taskID)
	}

	if attendance != nil && *attendance != nil {
		if err := services.LogoutAttendance(*attendance); err != nil {
			a.Logger.Warnf("op=logout_local_failed user_id=%s attendance_id=%d err=%v", userID, attendanceID, err)
			return err
		}
		timeOut := ""
		if (*attendance).TimeOut != nil {
			timeOut = (*attendance).TimeOut.Format(time.RFC3339)
		}
		a.Logger.Infof("op=logout_local_saved user_id=%s attendance_id=%d time_out=%s sync_status=%s", userID, attendanceID, timeOut, (*attendance).SyncStatus)

		if err := a.TaskService.SyncService.SyncAttendance(*attendance); err != nil {
			a.Logger.Printf("error syncing attendance on logout: %v", err)
			a.Logger.Warnf("op=logout_sync_failed_after_local_save user_id=%s attendance_id=%d err=%v", userID, attendanceID, err)
			return nil
		}
		a.Logger.Infof("op=logout_sync_success user_id=%s attendance_id=%d", userID, attendanceID)
	} else {
		a.Logger.Println("No active AttendanceID")
		a.Logger.Warn("op=logout_local_skipped reason=no_active_attendance")
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
