package main

import (
	"embed"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func hasArg(flag string) bool {
	for _, a := range os.Args[1:] {
		if strings.EqualFold(strings.TrimSpace(a), flag) {
			return true
		}
	}
	return false
}

func main() {
	app := NewApp()
	startMinimized := hasArg("--minimized")

	err := wails.Run(&options.App{
		Title:       "NetPulse",
		Width:       1220,
		Height:      780,
		MinWidth:    980,
		MinHeight:   640,
		StartHidden: startMinimized,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "netpulse-desktop-8d6b4f2c-19e1-4f3a-9c35-2f1e7a50c901",
			OnSecondInstanceLaunch: app.onSecondInstance,
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 9, G: 13, B: 22, A: 255},
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			Theme:                windows.Dark,
			WebviewIsTransparent: false,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
