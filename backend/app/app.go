package app

import (
	"context"
	"fesapp/backend/services"
	"fmt"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx        context.Context
	Screenshot *services.ScreenshotService
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		Screenshot: services.NewScreenshotService(),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) Shutdown(ctx context.Context) {
	a.ctx = ctx

	// cleanup logic here
	runtime.EventsEmit(ctx, "app_shutdown")
	fmt.Println("App is shutting down")
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) TakeScreenshot() []string {

	folderName := time.Now().Format("20060102_150405")

	screenshots, err := services.Screenshot(folderName)
	if err != nil {
		panic(err)
	}

	return screenshots
}

func (a *App) GetMonitorCount() int {
	return services.GetMonitorCount()
}

func (a *App) StartTracking() {
	a.Screenshot.Start(a.ctx)
}

func (a *App) StopTracking() {
	a.Screenshot.Stop()
}
