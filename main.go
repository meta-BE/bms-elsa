package main

import (
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// macOSで.appバンドル起動時にLANGが未設定だとpbcopy/pbpasteが文字化けする (wails#4132)
	if os.Getenv("LANG") == "" {
		os.Setenv("LANG", "ja_JP.UTF-8")
	}

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
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
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
			app.DiffImportHandler,
			app.DuplicateHandler,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
