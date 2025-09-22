package app

import (
	"context"
	"fesapp/backend/services"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx               context.Context
	ScreenshotService *services.ScreenshotService
	windowOpen        bool
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

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
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
