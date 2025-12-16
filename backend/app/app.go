package app

import (
	"context"
	_ "embed"
	"errors"
	config "fesapp/backend"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/services"
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
	SyncService       *services.SyncService
	windowOpen        bool
	AuthUser          *models.AuthUser
	WorkingTaskID     *string
	AttendanceID      *uint
}

// NewApp creates a new App application struct
func NewApp() *App {
	syncService := services.NewSyncService()
	screenshotService := services.NewScreenshotService(syncService)

	return &App{
		ScreenshotService: screenshotService,
		SyncService:       syncService,
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

func (a *App) TakeScreenshot() []string {
	screenshots, err := a.ScreenshotService.Screenshot()
	if err != nil {
		panic(err)
	}

	return screenshots
}

func (a *App) GetMonitorCount() int {
	return services.GetMonitorCount()
}

func (a *App) StartTracking(taskId, localStorageValue string, padagan bool) {
	if a.ScreenshotService.Running {
		a.ScreenshotService.Stop()
	}
	a.ScreenshotService.FolderName = localStorageValue
	a.WorkingTaskID = &taskId
	record, err := services.StartTaskRecord(a.AuthUser.LocalID, taskId, *a.AttendanceID, padagan)
	if err != nil {
		utils.Logger.Printf("error starting task record: %v", err)
	}
	a.ScreenshotService.TaskID = a.WorkingTaskID
	a.ScreenshotService.TaskRecordID = &record.ID
	a.ScreenshotService.Start(a.ctx)
}

func (a *App) StopTracking(taskId string) {
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
func (a *App) CheckAuth(name *string) bool {
	service := "fesapp"
	if name == nil || *name == "" {
		return false
	}

	_, err := keyring.Get(service, *name)
	return err == nil
}

// FIX this is for testing purpose only, production backend server handles user registration
func (a *App) RegisterUser(email string) (*db.User, error) {
	user, err := services.CreateUser("123", email)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// TODO production call to backend server for auth
func (a *App) LoginUser(email string) (*models.AuthUser, error) {
	user, err := services.OfflineLogin(email)
	if err != nil {
		return nil, err
	}

	authUser := &models.AuthUser{
		LocalID: user.ID,
		Email:   user.Email,
	}

	attendanceID, err := services.LoginAttendance(authUser.LocalID)
	if attendanceID == nil {
		return nil, err
	}

	a.AttendanceID = attendanceID
	a.AuthUser = authUser
	config.OnlineMode = false

	return authUser, nil
}

func (a *App) FirebaseLoginUser(email, password string) (*models.AuthUser, error) {
	user, err := services.FirebaseLogin(email, password)
	if err != nil {
		return nil, err
	}
	// TODO in the future include returning the profile

	authUser := &models.AuthUser{
		LocalID: user.LocalID,
		Email:   user.Email,
	}

	attendanceID, err := services.LoginAttendance(authUser.LocalID)
	if attendanceID == nil {
		return nil, err
	}

	a.AttendanceID = attendanceID
	a.AuthUser = authUser

	a.SyncService.Connect(a.ctx)
	// a.SyncService.SyncTasks()
	return authUser, nil
}

// TODO recheck if this will sustain online login method
func (a *App) BackendFirebaseSync(user_id, email, accessToken, refreshToken, expiresIn string) error {
	user := &models.FirebaseLoginResponse{
		LocalID:      user_id,
		Email:        email,
		IDToken:      accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}

	authUser := &models.AuthUser{
		LocalID: user.LocalID,
		Email:   user.Email,
	}
	// Creates a local representation of your account
	services.CreateUser(user.LocalID, user.Email)

	attendanceID, err := services.LoginAttendance(authUser.LocalID)
	if attendanceID == nil {
		return err
	}

	keyring.Set("refresh", email, refreshToken)
	now := time.Now()

	a.AttendanceID = attendanceID
	a.AuthUser = authUser
	a.SyncService.Backblaze.AuthUser = user
	a.SyncService.AuthUser = user
	a.SyncService.IdToken = &user.IDToken
	a.SyncService.IssuedAt = &now

	// TODO handle checking connection and only one connection attempt at a time
	go func() {
		a.SyncService.Connect(a.ctx)
		a.SyncService.SyncCredentials()
		// a.SyncService.SyncTasks()
	}()

	config.OnlineMode = true
	return nil
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
	record, err := a.ScreenshotService.GetLatestRecordsWithScreenshots(a.AuthUser.LocalID, taskId)
	if err != nil {
		return nil, err
	}

	a.WorkingTaskID = &record.Record.TaskID
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

// This is used to get the tasks that have recently used so that it can be quickly seen.
// Tasks Records displayed will be ordered by how recent they are
func (a *App) GetLastTaskRecords() ([]models.ExpTaskRecord, error) {

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
