package main

import (
	"embed"
	"fmt"

	"fesapp/backend/app"

	"github.com/joho/godotenv"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// loads .env in project root
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found")
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
		BackgroundColour:  &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:         app.Startup,
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

