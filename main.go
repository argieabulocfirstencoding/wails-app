package main

import (
	"embed"
	"os"

	"fesapp/backend/app"

	"github.com/joho/godotenv"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {

	godotenv.Load()

	_, exists := os.LookupEnv("DEVELOPMENT")
	// 2. Determine the unique ID based on the environment using build tags
	uniqueID := "com.firstencodingservices.myapp"

	if exists {
		uniqueID = uniqueID + ".development"
	}

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
