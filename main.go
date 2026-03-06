package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	if err := app.Init(); err != nil {
		println("Init error:", err.Error())
		return
	}

	err := wails.Run(&options.App{
		Title:  "BMS ELSA",
		Width:  1280,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
			app.SongHandler,
			app.IRHandler,
			app.InferenceHandler,
			app.RewriteHandler,
			app.ChartHandler,
			app.DifficultyTableHandler,
			app.ScanHandler,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
