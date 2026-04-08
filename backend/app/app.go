package app

import (
	"context"
	_ "embed"
	"errors"
	config "fesapp/backend"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/service_classes"
	"fesapp/backend/services"
	"fesapp/backend/types"
	"fesapp/backend/utils"
	"fmt"
	"log"
	"time"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zalando/go-keyring"
	"gorm.io/gorm"
)

// App struct
type App struct {
	ctx               context.Context
	ScreenshotService *services.ScreenshotService
	SyncService       *services.FirebaseSyncService
	AuthService       *service_classes.AuthService
	windowOpen        bool
	AuthUser          *models.AuthUser
	WorkingTaskID     *string
	AttendanceID      *uint
}

// NewApp creates a new App application struct
func NewApp() *App {
	syncService := services.NewFirebaseSyncService()
	screenshotService := services.NewScreenshotService(syncService)
	authService := service_classes.NewAuthService()

	return &App{
		ScreenshotService: screenshotService,
		SyncService:       syncService,
		AuthService:       authService,
	}
}

// Embed the tray icon
//
//go:embed assets/icon.ico
var trayIcon []byte

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	db.Init(ctx)
	db.DB.Exec("PRAGMA journal_mode = WAL;")
	db.DB.Exec("PRAGMA busy_timeout = 5000;")

	runtime.LogPrint(ctx, "App is starting")
	a.windowOpen = true

	go func() {
		defer func() {
			if r := recover(); r != nil {
				utils.Logger.Printf("Recovered from panic in systray: %v", r)
			}
		}()

		systray.Run(func() {
			// icon is now guaranteed to be valid
			systray.SetIcon(trayIcon)

			env := runtime.Environment(ctx)
			if env.BuildType == "dev" {
				systray.SetTitle("FESApp Dev")
				systray.SetTooltip("FESApp Dev")
			} else {
				systray.SetTitle("FESApp")
				systray.SetTooltip("FESApp")
			}

			mOpen := systray.AddMenuItem("Open", "Show the app window")
			mQuit := systray.AddMenuItem("Quit", "Exit app")

			go func() {
				for {
					select {
					case <-mOpen.ClickedCh:
						runtime.WindowShow(ctx)
						runtime.WindowUnminimise(ctx)
					case <-mQuit.ClickedCh:
						runtime.Quit(ctx)
						return
					}
				}
			}()
		}, func() {})
	}()

	// a.SyncService.Backblaze.Connect(a.ctx)
}

func (a *App) Shutdown(ctx context.Context) {
	a.ctx = ctx

	// cleanup logic here
	runtime.EventsEmit(ctx, "app_shutdown")
	if a.WorkingTaskID != nil {
		a.StopTracking(*a.WorkingTaskID)
		runtime.LogPrintf(ctx, "Closing task: %s", *a.WorkingTaskID)
	}

	runtime.LogPrint(ctx, "App is shutdown")
}

// DEPRECATED this was only used for testing
// func (a *App) TakeScreenshot() []string {
// 	screenshots, err := a.ScreenshotService.Screenshot()
// 	if err != nil {
// 		panic(err)
// 	}

// 	return screenshots
// }

func (a *App) GetMonitorCount() int {
	return services.GetMonitorCount()
}

func (a *App) StartTracking(taskId, localStorageValue string, padagan bool) {
	// NOTE this needs to be refactor, no logic should be applied here
	// FIX https://chatgpt.com/c/69b2df18-4fd0-8321-a821-12d5b92e3b2e

	if a.AttendanceID == nil {
		return
	}

	if a.ScreenshotService.Running {
		a.ScreenshotService.Stop()
	}

	if err := services.StopAllActiveRecords(a.AuthUser.LocalID); err != nil {
		fmt.Printf("Error stoping active task record: %s", err)
	}

	if a.ScreenshotService.TaskRecordID != nil {
		record, err := services.GetTaskRecordFromID(*a.ScreenshotService.TaskRecordID)
		if err == nil {
			a.ScreenshotService.SyncRecord(record)
		}
		if err != nil {
			fmt.Printf("Error on syncing record: %s", err)
		}

	}

	a.ScreenshotService.FolderName = localStorageValue
	a.WorkingTaskID = &taskId
	record, err := services.StartTaskRecord(a.AuthUser.LocalID, taskId, *a.AttendanceID, padagan)

	if err != nil {
		utils.Logger.Printf("error starting task record: %v", err)
	}
	a.ScreenshotService.TaskID = a.WorkingTaskID
	a.ScreenshotService.TaskRecordID = &record.ID

	task, err := services.GetTaskFromID(taskId)
	if err != nil {
		return
	}

	if task.ProjectName == "Break" {
		a.ScreenshotService.SyncRecord(record)
		return
	}

	a.ScreenshotService.Start(a.ctx)
}

func (a *App) StopTracking(taskId string) {
	// NOTE this needs to be refactor, no logic should be applied here
	if a.AuthUser == nil {
		utils.Logger.Println("StopTracking error: AuthUser is nil")
		return
	}

	a.ScreenshotService.Stop()
	err := services.StopTaskRecord(
		a.AuthUser.LocalID,
		taskId,
	)
	if err != nil {
		utils.Logger.Println("StopTaskRecord error:", err)
		return
	}

	// Check before dereferencing
	if a.ScreenshotService.TaskRecordID == nil {
		log.Println("No TaskRecordID available to sync, skipping Firebase sync.")
		return
	}

	// Syncing TaskRecord to Firebase
	if config.OnlineMode {
		// Retrieving TaskRecord from TaskRecordID
		log.Print("Attempting to sync record")
		taskRecord, err := services.GetTaskRecordFromID(*a.ScreenshotService.TaskRecordID)
		if err != nil {
			log.Printf("error retrieving task record: %v\n", err)
		}
		log.Printf("Syncing record: %v\n", taskRecord)

		attendance, _ := services.GetUserAttendance(a.AuthUser.LocalID)
		db.DB.Model(&taskRecord).Update("sync_status", "pending")
		db.DB.Model(&attendance).Update("sync_status", "pending")

		if err := a.SyncService.SyncTaskRecord(taskRecord); err != nil {
			log.Printf("error syncing task_record: %v", err)
		}
	}

	a.ScreenshotService.TaskRecordID = nil
}

func (a *App) BeforeClose(ctx context.Context) (prevent bool) {
	// NOTE this needs to be refactor, no logic should be applied here
	dialog, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:    runtime.QuestionDialog,
		Title:   "Quit?",
		Message: "Are you sure you want to quit? \n\nNote:Leaving will stop working task.",
	})

	if err != nil {
		return false
	}
	return dialog != "Yes"
}

// Deprecated testing purpose only
// func (a *App) CheckAuth(name *string) bool {
// 	service := "fesapp"
// 	if name == nil || *name == "" {
// 		return false
// 	}

// 	_, err := keyring.Get(service, *name)
// 	return err == nil
// }

// FIX this is for testing purpose only, production backend server handles user registration
func (a *App) RegisterUser(email string) (*db.User, error) {
	user, err := services.CreateUser("123", email)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// NOTE this is offline login
func (a *App) LoginUser(email string) (*models.OfflineLoginProps, error) {
	offlineData, attendanceID, authUser, err := services.LoginUser(email)
	if err != nil {
		return nil, err
	}

	a.AttendanceID = attendanceID
	a.AuthUser = authUser

	return offlineData, nil
}

func (a *App) LoginUserOnline(email string, password string) (*models.Attendance, error) {
	attendance, _, err := a.AuthService.Login(a.ctx, email, password)
	if err != nil {
		return nil, err
	}

	// a.AttendanceID = &attendance.ID
	// a.AuthUser = authUser

	return attendance, nil
}

// TODO recheck if this will sustain online login method
func (a *App) BackendFirebaseSync(user_id, email, accessToken, refreshToken, expiresIn string) (*models.Attendance, error) {
	// NOTE this needs to be refactor, no logic should be applied here
	user := &models.FirebaseLoginResponse{
		LocalID:      user_id,
		Email:        email,
		IDToken:      accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}

	// Creates a local representation of your account
	model_user, err := services.CreateUser(user.LocalID, user.Email)
	if err != nil {
		fmt.Printf("error creating account %s\n", err)
		return nil, err
	}

	authUser := &models.AuthUser{
		LocalID: user.LocalID,
		Email:   user.Email,
	}

	attendance, err := services.LoginAttendance(authUser.LocalID)
	if attendance == nil {
		return nil, err
	}

	attendanceID := &attendance.ID

	if model_user != nil && attendanceID != nil {
		go func() {
			services.FetchEmployeeDetails(a.ctx, model_user)
			attendance, err := services.GetAttendanceFromID(*attendanceID)
			if err != nil {
				return
			}
			a.SyncService.SyncAttendance(attendance)
		}()
	}

	keyring.Set("refresh", email, refreshToken)
	now := time.Now()

	a.AttendanceID = attendanceID
	a.AuthUser = authUser
	// a.SyncService.Backblaze.AuthUser = user
	a.SyncService.AuthUser = user
	a.SyncService.IdToken = &user.IDToken
	a.SyncService.IssuedAt = &now

	// TODO handle checking connection and only one connection attempt at a time
	go func() {
		a.SyncService.Connect(a.ctx)
		a.SyncService.SyncUser(*authUser)
		a.SyncService.SyncCredentials()
		// a.SyncService.SyncTasks()

	}()

	// Syncing attendance
	// attendance, err := services.GetAttendanceFromID(*attendanceID)
	// if err != nil {
	// 	fmt.Printf("error on sync connection %s", err)
	// }

	// services.NewSyncService().SyncAttendance(attendance)

	config.OnlineMode = true
	if err := services.AutoCloseStuckSessions(authUser.LocalID); err != nil {
		return nil, err
	}
	return attendance, nil
}

func (a *App) FetchUserDetails() (*models.AuthUser, error) {
	user, err := services.GetUserFromId(a.AuthUser.LocalID)
	if err != nil {
		return nil, err
	}

	authUser := &models.AuthUser{
		LocalID:   user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Position:  user.Position,
		Birthdate: user.Birthdate,
		Phone:     user.Phone,
		Address:   user.Address,
		Photo:     user.Photo,
		FESID:     user.FESID,
	}

	return authUser, nil
}

func (a *App) LogoutUser() error {
	//TODO logout logic involves saving and syncing data to firebase if available as well as store logout time

	if a.WorkingTaskID != nil {
		a.StopTracking(*a.WorkingTaskID)
		log.Printf("WorkingTaskID: %s", *a.WorkingTaskID)
	}

	if a.AttendanceID != nil {
		attendance, err := services.GetAttendanceFromID(*a.AttendanceID)
		if err != nil {
			return err
		}

		log.Printf("Attendance ID: %d", *a.AttendanceID)
		if err := services.LogoutAttendance(attendance); err != nil {
			return err
		}

		if err := a.SyncService.SyncAttendance(attendance); err != nil {
			utils.Logger.Printf("error syncing attendance on logout")
		}
	} else {
		log.Println("No active AttendanceID")
	}

	return nil
}

func (a *App) GetActiveTaskRecord() *string {
	// NOTE this needs to be refactor, no logic should be applied here
	taskRecord, err := services.GetActiveTaskRecord(a.AuthUser.LocalID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Logger.Print("No active record")
		} else {
			utils.Logger.Printf("Error getting active record %v", err)
		}

		return nil
	}

	if taskRecord != nil {
		a.WorkingTaskID = &taskRecord.TaskID
		a.ScreenshotService.TaskID = a.WorkingTaskID
		a.ScreenshotService.TaskRecordID = &taskRecord.ID
		a.ScreenshotService.FolderName = fmt.Sprintf("%d", time.Now().UnixMilli())
		a.ScreenshotService.Start(a.ctx)
	}

	return a.WorkingTaskID
}

func (a *App) GetLastTaskRecord(taskId string) (*models.TaskRecordWithScreenshots, error) {
	record, _ := a.ScreenshotService.GetLatestRecordsWithScreenshots(a.AuthUser.LocalID, taskId)
	// Removed erro handling for record beause most times when screenshots are deleted they throws an error here messing up "break".
	// So move forward disregarding if break is found or not.
	if record != nil {
		a.WorkingTaskID = &record.Record.TaskID
	}

	return record, nil
}

func (a *App) CreateTask(title, client string) (*models.ExpTask, error) {

	task, err := services.CreateTask(a.AuthUser.LocalID, title, client, nil)
	if err != nil {
		return nil, err
	}

	expTask := task.ToExpTask()
	return &expTask, nil
}

func (a *App) GetTasks(isOnline bool) ([]models.ExpTask, error) {
	// NOTE this needs to be refactor, no logic should be applied here
	if config.OnlineMode && isOnline {
		if err := services.SyncClientsAndProjects(); err != nil {
			utils.Logger.Printf("error fetching tasks from fes: %v", err)
		}
	}

	if a.AuthUser == nil {
		return nil, fmt.Errorf("GetTasks error: AuthUser is nil")
	}

	tasks, err := services.GetTasks()
	if err != nil {
		return nil, err
	}

	expTasks := models.ToExpTasks(tasks)
	return expTasks, nil
}

func (a *App) GetActivityLogRecord(dateStr string) ([]models.ActivityLogRecord, error) {
	return services.GetAttendanceAndRecords(dateStr)
}

func (a *App) OpenUpwork(taskID string) error {
	services.FetchUpworkCreds(a.ctx, taskID)

	return nil
}

func (a *App) GetAccounts() ([]models.User, error) {
	// NOTE this needs to be refactor, no logic should be applied here
	var users []models.User

	if err := db.DB.Model(&models.User{}).Find(&users).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("no users found: %v", err)
		}
		return nil, err
	}

	return users, nil
}

// Not used yet
func (a *App) SyncAttendance() error {
	if err := a.SyncService.SyncAllAttendance(); err != nil {
		return err
	}

	return nil
}

// Not used yet
func (a *App) SyncUnsyncedRecords() error {
	if err := a.SyncService.SyncUnsyncedRecords(); err != nil {
		return err
	}

	return nil
}

func (a *App) SyncUnsyncedTaskRecord(taskrecordId uint) error {
	taskRecord, err := services.GetTaskRecordFromID(taskrecordId)
	if err != nil {
		return err
	}

	// FIX change to attempt sync record
	if err := a.SyncService.SyncTaskRecord(taskRecord); err != nil {
		return err
	}

	return nil
}

func (a *App) SyncUnsyncedAttendance() error {
	// For now syncs current attendance
	attendance, err := services.GetAttendanceFromID(*a.AttendanceID)
	if err != nil {
		return err
	}

	// FIX change to attempt sync attendance
	if err := a.SyncService.SyncAttendance(attendance); err != nil {
		return err
	}

	return nil
}

func (a *App) GetCurrentAttendance() (*models.ExpAttendance, error) {

	attendance, err := services.GetAttendanceFromID(*a.AttendanceID)
	if err != nil {
		return nil, err
	}

	expAttendance := attendance.ToExp()
	return &expAttendance, nil
}

func (a *App) GetTotalClientRecords(dateStr string) ([]types.TaskSummary, error) {
	return services.GetTodaySummary(dateStr)
}

// This is used to get the tasks that have recently used so that it can be quickly seen.
// Tasks Records displayed will be ordered by how recent they are
func (a *App) GetLastTaskRecords() ([]models.ExpTaskRecord, error) {
	// NOTE this needs to be refactor, no logic should be applied here
	task_records, err := services.GetAllPreviousTasksRecords(a.AuthUser.LocalID)
	if err != nil {
		return nil, err
	}

	var exp_task_records []models.ExpTaskRecord

	for _, record := range task_records {
		exp_record := record.ToExp()
		exp_task_records = append(exp_task_records, exp_record)
	}

	return exp_task_records, nil
}

func (a *App) AddMemo(memo, notes string) {
	a.ScreenshotService.AddMemo(memo, notes)
}

func (a *App) GetFolderSize() (int64, error) {
	return services.GetScreenshotStorageSize(a.ctx)
}

func (a *App) ClearScreenshots() error {
	return services.ClearScreenshots(a.ctx)
}

// ----------------------------------------------------
// SINGLE INSTANCE LOGIC
// ----------------------------------------------------

// onSecondInstanceLaunch is the callback that runs in the FIRST instance
// when a user tries to launch a second instance of the application.
func (a *App) OnSecondInstanceLaunch(data options.SecondInstanceData) {
	// 4. Implement the logic to bring the main window to the front

	// Log the event and any arguments passed
	log.Printf("Second instance launch attempt detected. Arguments: %v", data.Args)

	// Bring the first instance's window to the foreground

	// 1. Unminimise the window if it was minimized
	runtime.WindowUnminimise(a.ctx)

	// 2. Bring the window to the front and give it focus
	runtime.WindowShow(a.ctx)

	// You could optionally send the arguments to the frontend
	// to open a specific file/view, e.g.:
	// runtime.EventsEmit(a.ctx, "secondInstanceArgs", data.Args)
}
