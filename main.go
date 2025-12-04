package main

import (
	"context"
	"embed"
	"fmt"

	"fesapp/backend/app"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {

	// 1. Get the context
	ctx := context.Background()

	// 2. Determine the unique ID based on the environment using build tags
	baseID := "com.firstencodingservicess.myapp"
	suffix := getUniqueIDSuffix(ctx)
	uniqueID := baseID + suffix

	fmt.Printf("uniqueID: %s", uniqueID)

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

// getUniqueIDSuffix provides the DEV suffix.
func getUniqueIDSuffix(ctx context.Context) string {
	// A simple, static suffix for development is usually enough.
	suffix := ".devmode"
	
	environment := runtime.Environment(ctx)

	if environment.BuildType == "production" {
		return ""
	}

	return suffix
}
