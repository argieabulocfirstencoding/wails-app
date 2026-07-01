package app

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
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
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
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
	Connectivity        *service_classes.ConnectivityStateService
	windowOpen          bool
	AuthUser            *models.AuthUser
	Logger              *utils.LoggerService

	WorkingTask   *models.Task
	Attendance    *models.Attendance
	WorkingTaskID *string
	AttendanceID  *uint
}

// CurrentVersion will be injected from wails.json or ldflags
var CurrentVersion = "2.4.5"

const attendanceResyncWindow = 48 * time.Hour

type UpdateInfo struct {
	Exists      bool   `json:"exists"`
	Version     string `json:"version"`
	DownloadUrl string `json:"downloadUrl"`
}

type githubReleaseAsset struct {
	BrowserDownloadURL string `json:"browser_download_url"`
	Name               string `json:"name"`
}

type githubRelease struct {
	TagName string               `json:"tag_name"`
	Assets  []githubReleaseAsset `json:"assets"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	connectivity := service_classes.NewConnectivityStateService()
	firebaseSyncService := services.NewFirebaseSyncService()
	logger := utils.NewLoggerService()

	screenshotService := service_classes.NewScreenshotService(connectivity)
	syncService := service_classes.NewSyncService(connectivity)
	authService := service_classes.NewAuthService(connectivity)

	app := &App{
		ScreenshotService:   screenshotService,
		FirebaseSyncService: firebaseSyncService,
		SyncService:         syncService,
		AuthService:         authService,
		Connectivity:        connectivity,
		Logger:              logger,
	}

	authService.SyncService = syncService
	authService.AttendanceService.SyncService = syncService
	authService.TaskService.WorkingTaskID = &app.WorkingTaskID
	authService.TaskService.Attendance = &app.Attendance
	authService.TaskService.ScreenshotService = screenshotService
	authService.TaskService.SyncService = syncService
	screenshotService.SyncService = syncService
	syncService.Attendance = &app.Attendance
	syncService.TaskRecord = &authService.TaskService.TaskRecord

	return app
}

// Embed the tray icon
//
//go:embed assets/icon.ico
var trayIcon []byte

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.AuthService.TaskService.Connect(ctx)

	env := runtime.Environment(ctx)
	if err := utils.ConfigureLogger(env.BuildType); err != nil {
		runtime.LogErrorf(ctx, "failed to configure file logger: %v", err)
	}

	if _, err := db.Init(ctx); err != nil {
		runtime.LogErrorf(ctx, "database initialization failed: %v", err)
		utils.Logger.Printf("database initialization failed: %v", err)
		_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
			Type:    runtime.ErrorDialog,
			Title:   "Startup Error",
			Message: "Unable to initialize local database. The app will now close.",
		})
		runtime.Quit(ctx)
		return
	}

	runtime.LogPrint(ctx, "App is starting")
	a.windowOpen = true

	utils.GoSafe("app.startup.systray", func() {
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

			utils.GoSafe("app.startup.systray.menu_loop", func() {
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
			})
		}, func() {})
	})

	// a.SyncService.Backblaze.Connect(a.ctx)
}

func (a *App) Shutdown(ctx context.Context) {
	a.ctx = ctx

	if a.ScreenshotService != nil {
		a.ScreenshotService.Stop()
	}

	// Shutdown should stop local tracking once and leave the record retryable on next launch.
	a.Connectivity.SetOnline(false)
	if a.WorkingTaskID != nil {
		taskID := *a.WorkingTaskID
		a.StopTracking(taskID)
		safeRuntimeLogPrintf(ctx, "Closing task: %s", taskID)
	}

	a.SyncService.Shutdown()

	func() {
		defer func() {
			_ = recover()
		}()
		systray.Quit()
	}()

	safeRuntimeLogPrint(ctx, "App is shutdown")
}

func (a *App) GetMonitorCount() int {
	return services.GetMonitorCount()
}

func (a *App) StartTracking(taskId, localStorageValue string, padagan bool) error {
	if err := a.AuthService.TaskService.StartTracking(taskId, localStorageValue, padagan); err != nil {
		return a.stableClientError("start_tracking", err)
	}
	return nil
}

func (a *App) StopTracking(taskId string) {
	a.AuthService.TaskService.StopTracking(taskId)
}

// NOTE this is offline login
func (a *App) LoginUser(email string) (*models.OfflineLoginProps, error) {
	correlationID := utils.NewCorrelationID("login_offline")
	a.Logger.Infof("op=login_offline_start correlation_id=%s email=%s", correlationID, strings.TrimSpace(email))
	offlineData, attendance, authUser, err := services.LoginUser(email)
	if err != nil {
		a.Logger.Warnf("op=login_offline_failed correlation_id=%s err=%v", correlationID, err)
		return nil, err
	}
	a.Connectivity.SetOnline(false)

	a.AttendanceID = &attendance.ID
	a.Attendance = attendance
	a.AuthUser = authUser
	a.SyncService.SetAuthUser(authUser)
	a.SyncService.Attendance = &a.Attendance
	a.SyncService.TaskRecord = &a.AuthService.TaskService.TaskRecord
	a.AuthService.TaskService.Attendance = &a.Attendance
	a.AuthService.TaskService.Connect(a.ctx)
	a.Logger.Infof("op=login_offline_success correlation_id=%s user_id=%s attendance_id=%d", correlationID, authUser.LocalID, attendance.ID)

	return offlineData, nil
}

func (a *App) LoginUserOnline(email string, password string) (*models.Attendance, error) {
	correlationID := utils.NewCorrelationID("login_online")
	a.Logger.Infof("op=login_online_start correlation_id=%s email=%s", correlationID, strings.TrimSpace(email))
	attendance, authUser, err := a.AuthService.Login(a.ctx, email, password)
	if err != nil {
		a.Logger.Warnf("op=login_online_failed correlation_id=%s err=%v", correlationID, err)
		return nil, err
	}

	a.AttendanceID = &attendance.ID
	a.Attendance = attendance
	a.AuthUser = authUser

	a.Connectivity.SetOnline(true)
	a.AuthService.TaskService.Attendance = &a.Attendance
	a.SyncService.SetAuthUser(authUser)
	a.SyncService.Attendance = &a.Attendance
	a.SyncService.TaskRecord = &a.AuthService.TaskService.TaskRecord
	utils.GoSafe("app.login_online.backblaze_init", func() {
		if err := a.initializeBackblaze(); err != nil {
			a.Logger.Warnf("op=login_online_backblaze_init_failed user_id=%s err=%v", authUser.LocalID, err)
		}
	})
	a.SyncService.KickoffAutoSyncOnLogin(attendance)
	a.Logger.Infof("op=login_online_success correlation_id=%s user_id=%s attendance_id=%d", correlationID, authUser.LocalID, attendance.ID)

	return attendance, nil
}

// REFACTOR this is not used anymore, was used on firebase integration
// TODO recheck if this will sustain online login method
func (a *App) BackendFirebaseSync(user_id, email, accessToken, refreshToken, expiresIn string) (*models.Attendance, error) {
	return nil, types.NewStableError("LEGACY_SYNC_DISABLED", "Legacy Firebase sync is disabled in this release.")
}

func (a *App) FetchUserDetails() (*models.AuthUser, error) {
	authUser, err := a.requireAuthenticatedUser()
	if err != nil {
		return nil, err
	}

	user, err := services.GetUserFromId(authUser.LocalID)
	if err != nil {
		return nil, a.stableClientError("fetch_user_details", err)
	}

	details := &models.AuthUser{
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

	return details, nil
}

func (a *App) LogoutUser() error {
	//TODO logout logic involves saving and syncing data to firebase if available as well as store logout time
	err := a.AuthService.Logout()
	if err == nil {
		a.Connectivity.SetOnline(false)
		a.clearSessionState()
	}
	return err
}

func (a *App) GetActiveTaskRecord() *string {
	if _, err := a.requireAuthenticatedUser(); err != nil {
		a.clearActiveTaskState(true)
		return nil
	}

	// NOTE this needs to be refactor, no logic should be applied here
	taskRecord, err := a.AuthService.TaskService.GetActiveTaskRecord(a.AuthUser.LocalID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Logger.Print("No active record")
			a.clearActiveTaskState(true)
			return nil
		} else {
			utils.Logger.Printf("Error getting active record %v", err)
			a.clearActiveTaskState(false)
			return nil
		}
	}

	if taskRecord != nil {
		a.WorkingTaskID = &taskRecord.TaskID
		if a.ScreenshotService != nil {
			a.ScreenshotService.TaskID = a.WorkingTaskID
			a.ScreenshotService.TaskRecordID = &taskRecord.ID
			a.ScreenshotService.FolderName = fmt.Sprintf("%d", time.Now().UnixMilli())
			a.ScreenshotService.Start(a.ctx)
		}
	}

	return a.WorkingTaskID
}

func (a *App) GetLastTaskRecord(taskId string) (*models.TaskRecordWithScreenshots, error) {
	authUser, err := a.requireAuthenticatedUser()
	if err != nil {
		return nil, err
	}

	record, err := a.ScreenshotService.GetLatestRecordsWithScreenshots(authUser.LocalID, taskId)
	if err != nil {
		return nil, a.stableClientError("get_last_task_record", err)
	}
	// Removed erro handling for record beause most times when screenshots are deleted they throws an error here messing up "break".
	// So move forward disregarding if break is found or not.
	return record, nil
}

func (a *App) SetTaskRecordNotes(taskRecordId uint, notes string) {
	a.AuthService.TaskService.SetTaskRecordNotes(taskRecordId, notes)
}

func (a *App) CreateTask(title, client string) (*models.ExpTask, error) {
	authUser, err := a.requireAuthenticatedUser()
	if err != nil {
		return nil, err
	}

	task, err := services.CreateTask(authUser.LocalID, title, client, nil)
	if err != nil {
		return nil, a.stableClientError("create_task", err)
	}

	expTask := task.ToExpTask()
	return &expTask, nil
}

func (a *App) GetTasks(isOnline bool) ([]models.ExpTask, error) {
	tasks, err := a.AuthService.TaskService.GetTasks(isOnline)
	if err != nil {
		return nil, a.stableClientError("get_tasks", err)
	}
	return tasks, nil
}

func (a *App) FetchActiveTask() error {
	if _, err := a.requireAuthenticatedUser(); err != nil {
		return err
	}
	if a.Attendance == nil {
		return types.NewStableError("ATTENDANCE_NOT_FOUND", "No active attendance found.")
	}

	_, err := a.SyncService.SyncActiveTaskRecord()
	if err != nil {
		return a.stableClientError("fetch_active_task", err)
	}

	return nil
}

func (a *App) GetActivityLogRecord(dateStr string) ([]models.ActivityLogRecord, error) {
	logUserID := "<nil>"
	queryUserID := ""
	if a.AuthUser != nil {
		queryUserID = strings.TrimSpace(a.AuthUser.LocalID)
		if queryUserID != "" {
			logUserID = queryUserID
		}
	}

	utils.Logger.Printf("app.GetActivityLogRecord: start date=%q user_id=%q", dateStr, logUserID)
	records, err := services.GetAttendanceAndRecords(dateStr, queryUserID)
	if err != nil {
		utils.Logger.Printf("app.GetActivityLogRecord: failed date=%q user_id=%q err=%v", dateStr, logUserID, err)
		return nil, err
	}

	utils.Logger.Printf("app.GetActivityLogRecord: success date=%q user_id=%q count=%d", dateStr, logUserID, len(records))
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
	return a.SyncUnsyncedAttendance()
}

// Not used yet
func (a *App) SyncUnsyncedRecords() error {
	if err := a.AuthService.SyncService.SyncUnsyncedRecords(); err != nil {
		return a.stableClientError("sync_unsynced_records", err)
	}
	return nil
}

func (a *App) SyncUnsyncedTaskRecord(taskrecordId uint) error {
	// FIX change to attempt sync record
	if err := a.AuthService.TaskService.SyncService.SyncTaskRecord(taskrecordId); err != nil {
		return a.stableClientError("sync_unsynced_task_record", types.WrapOperation("sync.task_record", taskrecordId, a.safeUserID(), err))
	}

	return nil
}

func (a *App) SyncUnsyncedAttendance() error {
	if a.AttendanceID == nil {
		return types.NewStableError("ATTENDANCE_NOT_FOUND", "No active attendance to sync.")
	}

	// For now syncs current attendance
	attendance, err := services.GetAttendanceFromID(*a.AttendanceID)
	if err != nil {
		return a.stableClientError("sync_unsynced_attendance_fetch", err)
	}

	// FIX change to attempt sync attendance
	if err := a.AuthService.TaskService.SyncService.SyncAttendance(attendance); err != nil {
		return a.stableClientError("sync_unsynced_attendance", types.WrapOperation("sync.attendance", attendance.ID, attendance.UserID, err))
	}

	return nil
}

func (a *App) SyncAttendanceByID(attendanceID uint) error {
	if a.AuthUser == nil || a.AuthUser.LocalID == "" {
		return types.NewStableError("AUTH_REQUIRED", "cannot resync attendance without authenticated user")
	}

	attendance, err := services.GetAttendanceFromID(attendanceID)
	if err != nil {
		return a.stableClientError("sync_attendance_by_id_fetch", types.WrapOperation("sync.attendance.fetch", attendanceID, a.safeUserID(), err))
	}

	if attendance.UserID != a.AuthUser.LocalID {
		return types.NewStableError("ATTENDANCE_OWNERSHIP_DENIED", "cannot resync attendance owned by another user")
	}

	if !canResyncAttendanceByTime(attendanceResyncReferenceTime(attendance), time.Now()) {
		return types.NewStableError("ATTENDANCE_RESYNC_WINDOW_EXPIRED", "cannot resync login/logout older than 2 days")
	}

	if err := a.AuthService.TaskService.SyncService.SyncAttendance(attendance); err != nil {
		return a.stableClientError("sync_attendance_by_id", types.WrapOperation("sync.attendance", attendance.ID, attendance.UserID, err))
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
	if a.AttendanceID == nil {
		return nil, types.NewStableError("ATTENDANCE_NOT_FOUND", "No active attendance found.")
	}

	attendance, err := services.GetAttendanceFromID(*a.AttendanceID)
	if err != nil {
		return nil, a.stableClientError("get_current_attendance", types.WrapOperation("attendance.get_current", *a.AttendanceID, a.safeUserID(), err))
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
	records, err := a.AuthService.TaskService.GetLastTaskRecords()
	if err != nil {
		return nil, a.stableClientError("get_last_task_records", err)
	}
	return records, nil
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
	release, err := fetchLatestTrustedRelease()
	if err != nil {
		a.logUpdaterDecision("check_updates_release_error", map[string]any{"error": err.Error()})
		return UpdateInfo{Exists: false}
	}

	vCurrent, err := semver.Parse(strings.TrimSpace(CurrentVersion))
	if err != nil {
		utils.Logger.Printf("check updates: invalid current version=%q err=%v", CurrentVersion, err)
		return UpdateInfo{Exists: false}
	}

	// Handle cases where TagName might not have 'v' prefix
	remoteTag := strings.TrimSpace(release.TagName)
	if remoteTag == "" {
		return UpdateInfo{Exists: false}
	}

	if strings.HasPrefix(strings.ToLower(remoteTag), "v") {
		remoteTag = strings.TrimSpace(remoteTag[1:])
	}

	if remoteTag == "" {
		return UpdateInfo{Exists: false}
	}

	vRemote, err := semver.Parse(remoteTag)
	if err != nil {
		utils.Logger.Printf("check updates: invalid remote version tag=%q err=%v", release.TagName, err)
		return UpdateInfo{Exists: false}
	}

	asset, err := pickTrustedInstallerAsset(release.Assets)
	if err != nil {
		a.logUpdaterDecision("check_updates_no_installer_asset", map[string]any{"error": err.Error()})
		return UpdateInfo{Exists: false}
	}

	downloadURL := strings.TrimSpace(asset.BrowserDownloadURL)
	if downloadURL == "" {
		return UpdateInfo{Exists: false}
	}

	if vRemote.GT(vCurrent) {
		a.logUpdaterDecision("check_updates_update_available", map[string]any{
			"current_version": CurrentVersion,
			"remote_version":  release.TagName,
			"asset_name":      asset.Name,
		})
		return UpdateInfo{
			Exists:      true,
			Version:     release.TagName,
			DownloadUrl: downloadURL,
		}
	}

	a.logUpdaterDecision("check_updates_up_to_date", map[string]any{
		"current_version": CurrentVersion,
		"remote_version":  release.TagName,
	})
	return UpdateInfo{Exists: false}
}

func (a *App) DownloadAndInstall(url string) error {
	release, err := fetchLatestTrustedRelease()
	if err != nil {
		a.logUpdaterDecision("download_install_release_error", map[string]any{"error": err.Error()})
		return err
	}

	asset, err := pickTrustedInstallerAsset(release.Assets)
	if err != nil {
		a.logUpdaterDecision("download_install_no_installer_asset", map[string]any{"error": err.Error()})
		return err
	}

	requestedURL := strings.TrimSpace(url)
	if requestedURL != "" && !strings.EqualFold(requestedURL, strings.TrimSpace(asset.BrowserDownloadURL)) {
		a.logUpdaterDecision("download_install_rejected_non_latest_asset", map[string]any{
			"requested_url": requestedURL,
			"allowed_url":   asset.BrowserDownloadURL,
		})
		return fmt.Errorf("installer URL rejected")
	}

	installerURL := strings.TrimSpace(asset.BrowserDownloadURL)
	if err := validateTrustedDownloadURL(installerURL); err != nil {
		a.logUpdaterDecision("download_install_rejected_untrusted_url", map[string]any{
			"url":   installerURL,
			"error": err.Error(),
		})
		return err
	}

	installerName := filepath.Base(strings.TrimSpace(asset.Name))
	if installerName == "." || installerName == string(filepath.Separator) || installerName == "" {
		installerName = "fes_installer.exe"
	}

	tempPath := filepath.Join(os.TempDir(), installerName)
	out, err := os.Create(tempPath)
	if err != nil {
		return err
	}
	defer out.Close()

	req, err := http.NewRequest(http.MethodGet, installerURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/octet-stream")

	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		a.logUpdaterDecision("download_install_http_status_rejected", map[string]any{
			"url":    installerURL,
			"status": resp.StatusCode,
		})
		return fmt.Errorf("installer download failed with status: %d", resp.StatusCode)
	}

	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if contentType == "" || strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/json") {
		a.logUpdaterDecision("download_install_content_type_rejected", map[string]any{
			"url":          installerURL,
			"content_type": contentType,
		})
		return fmt.Errorf("invalid installer content type: %s", contentType)
	}

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(out, hasher), resp.Body)
	if err != nil {
		return err
	}
	if written <= 0 {
		return fmt.Errorf("empty installer payload")
	}

	if err := verifyInstallerChecksumIfAvailable(release.Assets, installerName, hasher.Sum(nil)); err != nil {
		a.logUpdaterDecision("download_install_checksum_failed", map[string]any{
			"asset_name": installerName,
			"error":      err.Error(),
		})
		return err
	}

	a.logUpdaterDecision("download_install_verified", map[string]any{
		"version":    release.TagName,
		"asset_name": installerName,
		"url":        installerURL,
		"size_bytes": written,
	})

	cmd := exec.Command(tempPath, "/S")
	err = cmd.Start()
	if err != nil {
		a.logUpdaterDecision("download_install_execute_failed", map[string]any{
			"path":  tempPath,
			"error": err.Error(),
		})
		return err
	}

	a.logUpdaterDecision("download_install_execute_started", map[string]any{
		"path": tempPath,
	})

	if a.ctx != nil {
		go func(ctx context.Context) {
			time.Sleep(250 * time.Millisecond)
			runtime.Quit(ctx)
		}(a.ctx)
	}

	return nil
}

func fetchLatestTrustedRelease() (*githubRelease, error) {
	const latestURL = "https://api.github.com/repos/FES-Paul/FES-APP-TRACKER/releases/latest"
	req, err := http.NewRequest(http.MethodGet, latestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release lookup failed status=%d", resp.StatusCode)
	}

	if ct := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type"))); ct != "" && !strings.Contains(ct, "application/json") {
		return nil, fmt.Errorf("release lookup invalid content type: %s", ct)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}

	return &rel, nil
}

func pickTrustedInstallerAsset(assets []githubReleaseAsset) (*githubReleaseAsset, error) {
	for i := range assets {
		name := strings.ToLower(strings.TrimSpace(assets[i].Name))
		if !strings.HasSuffix(name, ".exe") {
			continue
		}
		if strings.Contains(name, "sha256") || strings.Contains(name, "checksum") || strings.Contains(name, ".sig") {
			continue
		}
		if err := validateTrustedDownloadURL(assets[i].BrowserDownloadURL); err != nil {
			continue
		}
		return &assets[i], nil
	}
	return nil, fmt.Errorf("no trusted installer asset found")
}

func validateTrustedDownloadURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("untrusted scheme")
	}
	host := strings.ToLower(strings.TrimSpace(u.Host))
	if host != "github.com" && host != "objects.githubusercontent.com" && host != "release-assets.githubusercontent.com" {
		return fmt.Errorf("untrusted host")
	}

	p := path.Clean(u.Path)
	if host == "github.com" && !strings.Contains(p, "/FES-Paul/FES-APP-TRACKER/releases/download/") {
		return fmt.Errorf("untrusted github path")
	}
	return nil
}

func verifyInstallerChecksumIfAvailable(assets []githubReleaseAsset, installerName string, actualHash []byte) error {
	var checksumURL string
	for i := range assets {
		name := strings.ToLower(strings.TrimSpace(assets[i].Name))
		if strings.Contains(name, "sha256") || strings.Contains(name, "checksums") {
			checksumURL = strings.TrimSpace(assets[i].BrowserDownloadURL)
			break
		}
	}

	if checksumURL == "" {
		return nil
	}
	if err := validateTrustedDownloadURL(checksumURL); err != nil {
		return err
	}

	resp, err := http.Get(checksumURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("checksum download status=%d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return err
	}
	lines := strings.Split(string(body), "\n")
	expected := ""
	lowerInstaller := strings.ToLower(strings.TrimSpace(installerName))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		filename := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(fields[1], "*")))
		if filename == lowerInstaller {
			expected = strings.ToLower(strings.TrimSpace(fields[0]))
			break
		}
	}
	if expected == "" {
		return fmt.Errorf("checksum entry not found for installer")
	}

	actual := fmt.Sprintf("%x", actualHash)
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("checksum mismatch")
	}
	return nil
}

func (a *App) logUpdaterDecision(event string, details map[string]any) {
	raw, _ := json.Marshal(details)
	utils.Logger.Printf("updater: event=%s details=%s", event, string(raw))
	if a.ctx != nil {
		runtime.LogPrintf(a.ctx, "updater: event=%s details=%s", event, string(raw))
	}
}

func (a *App) stableClientError(op string, err error) error {
	if err == nil {
		return nil
	}

	if stable, ok := err.(*types.StableError); ok {
		return stable
	}

	utils.Logger.Printf("app error op=%s user_id=%s err=%v", op, a.safeUserID(), err)
	if a.ctx != nil {
		runtime.LogErrorf(a.ctx, "app error op=%s user_id=%s err=%v", op, a.safeUserID(), err)
	}

	return types.NewStableError("OPERATION_FAILED", "Operation failed. Please try again.")
}

func (a *App) safeUserID() string {
	if a.AuthUser == nil {
		return ""
	}
	return a.AuthUser.LocalID
}

func (a *App) requireAuthenticatedUser() (*models.AuthUser, error) {
	if a.AuthUser == nil || strings.TrimSpace(a.AuthUser.LocalID) == "" {
		return nil, types.NewStableError("AUTH_REQUIRED", "Login required.")
	}
	return a.AuthUser, nil
}

func (a *App) clearActiveTaskState(stopScreenshot bool) {
	a.WorkingTaskID = nil

	if a.AuthService != nil && a.AuthService.TaskService != nil {
		a.AuthService.TaskService.TaskRecord = nil
	}

	if a.ScreenshotService != nil {
		if stopScreenshot {
			a.ScreenshotService.Stop()
		}
		a.ScreenshotService.TaskID = nil
		a.ScreenshotService.TaskRecordID = nil
	}
}

func (a *App) clearSessionState() {
	a.clearActiveTaskState(true)
	a.Attendance = nil
	a.AttendanceID = nil
	a.AuthUser = nil

	if a.SyncService != nil {
		a.SyncService.SetAuthUser(nil)
	}
}

func (a *App) initializeBackblaze() error {
	if a == nil || a.SyncService == nil || a.SyncService.Backblaze == nil {
		return nil
	}
	if !a.Connectivity.IsOnline() {
		return nil
	}
	if err := a.AuthService.TaskService.SyncService.FesClientService.GetBucketKeys(); err != nil {
		return err
	}
	return a.AuthService.TaskService.SyncService.Backblaze.Connect(a.ctx)
}

func safeRuntimeLogPrint(ctx context.Context, message string) {
	if ctx == nil {
		return
	}

	defer func() {
		_ = recover()
	}()
	runtime.LogPrint(ctx, message)
}

func safeRuntimeLogPrintf(ctx context.Context, format string, args ...any) {
	if ctx == nil {
		return
	}

	defer func() {
		_ = recover()
	}()
	runtime.LogPrintf(ctx, format, args...)
}
