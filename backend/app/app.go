package app

import (
	"context"
	"errors"
	config "fesapp/backend"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/services"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/getlantern/systray"
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

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	db.Init(ctx)
	db.DB.Exec("PRAGMA journal_mode = WAL;")
	db.DB.Exec("PRAGMA busy_timeout = 5000;")

	runtime.LogPrint(ctx, "App is starting")
	a.windowOpen = true

	// System Tray startup
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("Recovered from panic in systray:", r)
			}
		}()

		systray.Run(func() {

			exePath, _ := os.Executable()
			root := filepath.Dir(exePath)
			iconPath := filepath.Join(root, "icon.ico")
			icon, _ := os.ReadFile(iconPath)

			env := runtime.Environment(ctx)

			systray.SetIcon(icon)
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
						systray.Quit()
						runtime.Quit(ctx)
						return
					}
				}
			}()
		}, func() {
			// cleanup on exit
		})
	}()
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

func (a *App) StartTracking(taskId, localStorageValue string) {
	if (a.ScreenshotService.Running) {
		a.ScreenshotService.Stop()
	}
	a.ScreenshotService.FolderName = localStorageValue
	a.WorkingTaskID = &taskId
	record, err := services.StartTaskRecord(a.AuthUser.LocalID, taskId, *a.AttendanceID)
	if err != nil {
		fmt.Print(err)
	}
	a.ScreenshotService.TaskID = a.WorkingTaskID
	a.ScreenshotService.TaskRecordID = &record.ID
	a.ScreenshotService.Start(a.ctx)
}

func (a *App) StopTracking(taskId string) {
	if a.AuthUser == nil {
		fmt.Println("StopTracking error: AuthUser is nil")
		return
	}

	a.ScreenshotService.Stop()
	err := services.StopTaskRecord(
		a.AuthUser.LocalID,
		taskId,
	)
	if err != nil {
		fmt.Println("StopTaskRecord error:", err)
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
		taskRecord, err := services.GetTaskRecordFormID(*a.ScreenshotService.TaskRecordID)
		if err != nil {
			log.Printf("error retrieving task record: %v\n", err)
		}
		log.Printf("Syncing record: %v\n", taskRecord)

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

	a.SyncService.Connect(a.ctx, user)
	a.SyncService.SyncTasks()
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

	a.AttendanceID = attendanceID
	a.AuthUser = authUser

	go func() {
		a.SyncService.Connect(a.ctx, user)
		a.SyncService.SyncTasks()
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
		log.Printf("Attendance ID: %d", *a.AttendanceID)
		if err := services.LogoutAttendance(*a.AttendanceID); err != nil {
			return err
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
			fmt.Print("No active record")
		} else {
			fmt.Printf("Error getting active record %v", err)
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

func (a *App) GetTasks() ([]models.ExpTask, error) {
	a.SyncService.SyncTasks()

	if a.AuthUser == nil {
		return nil, fmt.Errorf("GetTasks error: AuthUser is nil")
	}

	tasks, err := services.GetTasks(a.AuthUser.LocalID)
	if err != nil {
		return nil, err
	}

	expTasks := models.ToExpTasks(tasks)
	return expTasks, nil
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

func (a *App) SyncAttendance() error {
	if err := a.SyncService.SyncAllAttendance(); err != nil {
		return err
	}

	return nil
}

func (a *App) SyncUnsyncedRecords() error {
	if err := a.SyncService.SyncUnsyncedRecords(); err != nil {
		return err
	}

	return nil
}
