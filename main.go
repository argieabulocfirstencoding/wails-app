package main

import (
	"context"
	"embed"

	"fesapp/backend/app"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {

	ctx := context.Background()

	// 2. Check the environment build type
	env := runtime.Environment(ctx)

	// 3. Define the base ID and an empty suffix
	baseID := "com.firstencodingservicess.myapp.DEVPAUL"
	suffix := ""

	// 4. Conditionally add a suffix for non-production builds (e.g., "dev")
	if env.BuildType != "production" {
		// Use a distinct suffix for development to prevent conflicts
		// You can use a static string like ".DEV" or include a username/pid for more safety
		suffix = ".devmode"
	}

	uniqueID := baseID + suffix

	// Create an instance of the app structure
	app := app.NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "FES App Tracker",
		Width:  390,
		Height: 780,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.Startup,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               uniqueID,
			OnSecondInstanceLaunch: app.OnSecondInstanceLaunch,
		},
		OnShutdown:        app.Shutdown,
		HideWindowOnClose: true,
		Bind: []interface{}{
			app,
		},
		DisableResize: true,
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
