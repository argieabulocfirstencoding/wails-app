package app

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	config "fesapp/backend"
	"fesapp/backend/db"
	"fesapp/backend/db/models"
	"fesapp/backend/service_classes"
	"fesapp/backend/services"
	"fesapp/backend/types"
	"fesapp/backend/utils"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zalando/go-keyring"
	"gorm.io/gorm"

	"github.com/blang/semver"
)

// App struct
type App struct {
	ctx                 context.Context
	ScreenshotService   *service_classes.ScreenshotService
	FirebaseSyncService *services.FirebaseSyncService
	AuthService         *service_classes.AuthService
	SyncService         *service_classes.SyncService
	windowOpen          bool
	AuthUser            *models.AuthUser
	Logger              *utils.LoggerService

	WorkingTask   *models.Task
	Attendance    *models.Attendance
	WorkingTaskID *string
	AttendanceID  *uint
}

// CurrentVersion will be injected from wails.json or ldflags
var CurrentVersion = "2.3.4"

const attendanceResyncWindow = 48 * time.Hour

type UpdateInfo struct {
	Exists      bool   `json:"exists"`
	Version     string `json:"version"`
	DownloadUrl string `json:"downloadUrl"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	firebaseSyncService := services.NewFirebaseSyncService()
	logger := utils.NewLoggerService()

	screenshotService := service_classes.NewScreenshotService()
	syncService := service_classes.NewSyncService()
	authService := service_classes.NewAuthService()

	app := &App{
		ScreenshotService:   screenshotService,
		FirebaseSyncService: firebaseSyncService,
		SyncService:         syncService,
		AuthService:         authService,
		Logger:              logger,
	}

	authService.SyncService = syncService

	authService.TaskService.Connect(app.ctx)
	authService.TaskService.WorkingTaskID = &app.WorkingTaskID
	authService.TaskService.Attendance = &app.Attendance
	authService.TaskService.ScreenshotService = screenshotService
	authService.TaskService.SyncService = syncService

	syncService.Connect(app.ctx)

	return app
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

	env := runtime.Environment(ctx)
	if err := utils.ConfigureLogger(env.BuildType); err != nil {
		runtime.LogErrorf(ctx, "failed to configure file logger: %v", err)
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				utils.Logger.Printf("Recovered from panic in systray: %v", r)
			}
		}()

		systray.Run(func() {
			// icon is now guaranteed to be valid
			systray.SetIcon(trayIcon)

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

	a.SyncService.Shutdown()
	runtime.LogPrint(ctx, "App is shutdown")
}

func (a *App) GetMonitorCount() int {
	return services.GetMonitorCount()
}

func (a *App) StartTracking(taskId, localStorageValue string, padagan bool) {
	a.AuthService.TaskService.StartTracking(taskId, localStorageValue, padagan)

}

func (a *App) StopTracking(taskId string) {
	a.AuthService.TaskService.StopTracking(taskId)
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

// NOTE this is offline login
func (a *App) LoginUser(email string) (*models.OfflineLoginProps, error) {
	offlineData, attendance, authUser, err := services.LoginUser(email)
	if err != nil {
		return nil, err
	}

	a.AttendanceID = &attendance.ID
	a.AuthUser = authUser
	a.AuthService.TaskService.SyncService.AuthUser = authUser
	a.AuthService.TaskService.Attendance = &attendance
	a.AuthService.TaskService.Connect(a.ctx)

	return offlineData, nil
}

func (a *App) LoginUserOnline(email string, password string) (*models.Attendance, error) {
	attendance, authUser, err := a.AuthService.Login(a.ctx, email, password)
	if err != nil {
		return nil, err
	}
	a.AttendanceID = &attendance.ID
	a.Attendance = attendance
	a.AuthUser = authUser

	config.OnlineMode = true

	return attendance, nil
}

// REFACTOR this is not used anymore, was used on firebase integration
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
			a.FirebaseSyncService.SyncAttendance(attendance)
		}()
	}

	keyring.Set("refresh", email, refreshToken)
	now := time.Now()

	a.AttendanceID = attendanceID
	a.AuthUser = authUser
	// a.SyncService.Backblaze.AuthUser = user
	a.FirebaseSyncService.AuthUser = user
	a.FirebaseSyncService.IdToken = &user.IDToken
	a.FirebaseSyncService.IssuedAt = &now

	// TODO handle checking connection and only one connection attempt at a time
	go func() {
		a.FirebaseSyncService.Connect(a.ctx)
		a.FirebaseSyncService.SyncUser(*authUser)
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
	return a.AuthService.Logout()
}

func (a *App) GetActiveTaskRecord() *string {
	// NOTE this needs to be refactor, no logic should be applied here
	taskRecord, err := a.AuthService.TaskService.GetActiveTaskRecord(a.AuthUser.LocalID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Logger.Print("No active record")

		} else {
			utils.Logger.Printf("Error getting active record %v", err)
			return nil
		}
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

func (a *App) SetTaskRecordNotes(taskRecordId uint, notes string) {
	a.AuthService.TaskService.SetTaskRecordNotes(taskRecordId, notes)
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
	return a.AuthService.TaskService.GetTasks(isOnline)
}

func (a *App) FetchActiveTask() error {
	_, err := a.SyncService.SyncActiveTaskRecord()
	if err != nil {
		return err
	}

	return nil
}

func (a *App) GetActivityLogRecord(dateStr string) ([]models.ActivityLogRecord, error) {
	userID := "<nil>"
	if a.AuthUser != nil {
		userID = a.AuthUser.LocalID
	}

	utils.Logger.Printf("app.GetActivityLogRecord: start date=%q user_id=%q", dateStr, userID)
	records, err := services.GetAttendanceAndRecords(dateStr)
	if err != nil {
		utils.Logger.Printf("app.GetActivityLogRecord: failed date=%q user_id=%q err=%v", dateStr, userID, err)
		return nil, err
	}

	utils.Logger.Printf("app.GetActivityLogRecord: success date=%q user_id=%q count=%d", dateStr, userID, len(records))
	return records, nil
}

func (a *App) OpenUpwork(taskID string) error {
	a.SyncService.FesClientService.FetchUpworkCreds(a.ctx, taskID)

	return nil
}

func (a *App) OpenFesWebApp() error {
	return a.SyncService.FesClientService.OpenFesWebApp()
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
	if err := a.FirebaseSyncService.SyncAllAttendance(); err != nil {
		return err
	}

	return nil
}

// Not used yet
func (a *App) SyncUnsyncedRecords() error {

	return a.AuthService.SyncService.SyncUnsyncedRecords()
}

func (a *App) SyncUnsyncedTaskRecord(taskrecordId uint) error {
	// FIX change to attempt sync record
	if err := a.AuthService.TaskService.SyncService.SyncTaskRecord(taskrecordId); err != nil {
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
	if err := a.AuthService.TaskService.SyncService.SyncAttendance(attendance); err != nil {
		return err
	}

	return nil
}

func (a *App) SyncAttendanceByID(attendanceID uint) error {
	if a.AuthUser == nil || a.AuthUser.LocalID == "" {
		return fmt.Errorf("cannot resync attendance without authenticated user")
	}

	attendance, err := services.GetAttendanceFromID(attendanceID)
	if err != nil {
		return err
	}

	if attendance.UserID != a.AuthUser.LocalID {
		return fmt.Errorf("cannot resync attendance owned by another user")
	}

	if !canResyncAttendanceByTime(attendanceResyncReferenceTime(attendance), time.Now()) {
		return fmt.Errorf("cannot resync login/logout older than 2 days")
	}

	if err := a.AuthService.TaskService.SyncService.SyncAttendance(attendance); err != nil {
		return err
	}

	return nil
}

func attendanceResyncReferenceTime(attendance *models.Attendance) time.Time {
	if attendance == nil {
		return time.Time{}
	}

	if attendance.TimeOut != nil {
		return *attendance.TimeOut
	}

	return attendance.TimeIn
}

func canResyncAttendanceByTime(attendanceTime, now time.Time) bool {
	if attendanceTime.IsZero() {
		return false
	}

	elapsed := now.Sub(attendanceTime)
	if elapsed < 0 {
		return true
	}

	return elapsed < attendanceResyncWindow
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
	userID := "<nil>"
	if a.AuthUser != nil {
		userID = a.AuthUser.LocalID
	}

	utils.Logger.Printf("app.GetTotalClientRecords: start date=%q user_id=%q", dateStr, userID)
	records, err := services.GetTodaySummary(dateStr)
	if err != nil {
		utils.Logger.Printf("app.GetTotalClientRecords: failed date=%q user_id=%q err=%v", dateStr, userID, err)
		return nil, err
	}

	utils.Logger.Printf("app.GetTotalClientRecords: success date=%q user_id=%q count=%d", dateStr, userID, len(records))
	return records, nil
}

// This is used to get the tasks that have recently used so that it can be quickly seen.
// Tasks Records displayed will be ordered by how recent they are
func (a *App) GetLastTaskRecords() ([]models.ExpTaskRecord, error) {
	return a.AuthService.TaskService.GetLastTaskRecords()
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

func (a *App) GetAppCurrentVersion() string {
	return CurrentVersion
}

// FIX BELOW IS ABOUT AUTO UPDATE PROMPTS BUT NOT WORKING CORRECTLY

func (a *App) CheckForUpdates() UpdateInfo {
	// FIX: Added /releases/latest to the end of the URL
	url := "https://api.github.com/repos/FES-Paul/FES-APP-TRACKER/releases/latest"

	res, err := http.Get(url)
	if err != nil || res.StatusCode != 200 {
		return UpdateInfo{Exists: false}
	}
	defer res.Body.Close()

	var target struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			BrowserDownloadURL string `json:"browser_download_url"`
			Name               string `json:"name"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(res.Body).Decode(&target); err != nil {
		return UpdateInfo{Exists: false}
	}

	vCurrent := semver.MustParse(CurrentVersion)
	// Handle cases where TagName might not have 'v' prefix
	remoteTag := target.TagName
	if remoteTag[0] == 'v' {
		remoteTag = remoteTag[1:]
	}

	vRemote, err := semver.Parse(remoteTag)

	if err == nil && vRemote.GT(vCurrent) && len(target.Assets) > 0 {
		return UpdateInfo{
			Exists:      true,
			Version:     target.TagName,
			DownloadUrl: target.Assets[0].BrowserDownloadURL,
		}
	}

	return UpdateInfo{Exists: false}
}

func (a *App) DownloadAndInstall(url string) error {
	// 1. Create a path for the temporary installer
	tempPath := os.TempDir() + "\\fes_installer.exe"
	out, err := os.Create(tempPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// 2. Download the file
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	// 3. Launch the installer
	// We use /S for a silent install so it's "Automatic"
	cmd := exec.Command(tempPath, "/S")
	err = cmd.Start()
	if err != nil {
		return err
	}

	// 4. Exit the app so the installer can replace the files
	os.Exit(0)
	return nil
}
