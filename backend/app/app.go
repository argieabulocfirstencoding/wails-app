package app

import (
	"context"
	"fesapp/backend/services"
	"fmt"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) TakeScreenshot() []string {
	screenshots, err := services.Screenshot()
	if err != nil {
		panic(err)
	}

	return screenshots
}

func (a *App) GetMonitorCount() int {

	return services.GetMonitorCount()
}
