package app

import (
	"context"
	"errors"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/services"
	"fmt"
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
	windowOpen        bool
	AuthUser          *services.AuthUser
	WorkingTaskID     *string
	AttendanceID      *uint
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		ScreenshotService: services.NewScreenshotService(),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	db.Init(ctx)
	runtime.LogPrint(ctx, "App is starting")
	a.windowOpen = true

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
	a.ScreenshotService.FolderName = localStorageValue
	a.WorkingTaskID = &taskId
	record, err := services.StartTaskRecord(a.AuthUser.UserID, taskId, *a.AttendanceID)
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
	a.ScreenshotService.TaskRecordID = nil

	err := services.StopTaskRecord(
		a.AuthUser.UserID,
		taskId,
	)
	if err != nil {
		fmt.Println("StopTaskRecord error:", err)
	}
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
func (a *App) LoginUser(email, password string) (*services.AuthUser, error) {
	user, err := services.Login(email, password)
	if err != nil {
		return nil, err
	}

	authUser := &services.AuthUser{
		UserID: user.ID,
		Email:  user.Email,
	}

	a.AuthUser = authUser
	return authUser, nil
}

func (a *App) FirebaseLoginUser(email, password string) (*services.AuthUser, error) {
	user, err := services.FirebaseLogin(email, password)
	if err != nil {
		return nil, err
	}
	// TODO in the future include returning the profile

	authUser := &services.AuthUser{
		UserID: user.LocalID,
		Email:  user.Email,
	}

	attendanceID, err := services.LoginAttendance(authUser.UserID)
	if attendanceID == nil {
		return nil, err
	}

	taskRecord, err := services.GetActiveTaskRecord(authUser.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fmt.Print("No active record")
		} else {
			fmt.Printf("Error getting active record %v", err)
		}
	}

	a.AttendanceID = attendanceID
	a.AuthUser = authUser
	if taskRecord != nil {

		a.WorkingTaskID = &taskRecord.TaskID
		a.ScreenshotService.TaskID = a.WorkingTaskID
		a.ScreenshotService.TaskRecordID = &taskRecord.ID
		a.ScreenshotService.FolderName = fmt.Sprintf("%d", time.Now().UnixMilli())
		a.ScreenshotService.Start(a.ctx)
	}

	return authUser, nil
}

func (a *App) LogoutUser() error {
	//TODO logout logic involves saving and syncing data to firebase if available as well as store logout time

	if a.WorkingTaskID != nil {
		a.StopTracking(*a.WorkingTaskID)
	}

	if err := services.LogoutAttendance(*a.AttendanceID); err != nil {
		return err
	}

	return nil
}

func (a *App) GetLastTaskRecord(taskId string) (*models.TaskRecordWithScreenshots, error) {
	record, err := a.ScreenshotService.GetLatestRecordsWithScreenshots(a.AuthUser.UserID, taskId)
	if err != nil {
		return nil, err
	}

	a.WorkingTaskID = &record.Record.TaskID
	return record, nil
}
func (a *App) CreateTask(title, client string) (*models.ExpTask, error) {

	task, err := services.CreateTask(a.AuthUser.UserID, title, client, nil)
	if err != nil {
		return nil, err
	}

	expTask := task.ToExpTask()
	return &expTask, nil
}

func (a *App) GetTasks() ([]models.ExpTask, error) {
	if a.AuthUser == nil {
		return nil, fmt.Errorf("GetTasks error: AuthUser is nil")
	}

	tasks, err := services.GetTasks(a.AuthUser.UserID)
	if err != nil {
		return nil, err
	}

	expTasks := models.ToExpTasks(tasks)
	return expTasks, nil
}
