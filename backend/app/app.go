package app

import (
	"context"
	"fesapp/backend/db"
	"fesapp/backend/services"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/zalando/go-keyring"
	"gorm.io/gorm"
)

// App struct
type App struct {
	ctx               context.Context
	ScreenshotService *services.ScreenshotService
	Database          *gorm.DB
	windowOpen        bool
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		ScreenshotService: services.NewScreenshotService(),
		Database:          db.Init(),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	runtime.LogPrint(ctx, "App is starting")
	a.windowOpen = true

	// --- Tray setup ---
	trayMenu := menu.NewMenu()

	// Toggle window show/hide
	trayMenu.AddText("Toggle Window", nil, func(_ *menu.CallbackData) {
		if a.windowOpen {
			runtime.WindowHide(ctx)
		} else {
			runtime.WindowShow(ctx)
			runtime.WindowUnminimise(ctx)
		}
	})

	trayMenu.AddSeparator()

	// Quit option
	trayMenu.AddText("Quit", nil, func(_ *menu.CallbackData) {
		runtime.Quit(ctx)
	})

}

func (a *App) Shutdown(ctx context.Context) {
	a.ctx = ctx

	// cleanup logic here
	runtime.EventsEmit(ctx, "app_shutdown")
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

func (a *App) StartTracking(localStorageValue string) {
	a.ScreenshotService.FolderName = localStorageValue
	a.ScreenshotService.Start(a.ctx)
}

func (a *App) StopTracking() {
	a.ScreenshotService.Stop()
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
		LocalID: user.ID,
		Email:   user.Email,
	}

	return authUser, nil
}

func (a *App) FirebaseLoginUser(email, password string) (*services.AuthUser, error) {
	user, err := services.FirebaseLogin(email, password)
	if err != nil {
		return nil, err
	}
	// TODO in the future include returning the profile

	authUser := &services.AuthUser{
		LocalID: user.LocalID,
		Email:   user.Email,
	}

	return authUser, nil
}

func (a *App) LogoutUser() {
	//TODO logout logic involves saving and syncing data to firebase if available as well as store logout time

}
