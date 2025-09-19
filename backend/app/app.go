package app

import (
	"context"
	"fesapp/backend/services"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx               context.Context
	ScreenshotService *services.ScreenshotService
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
