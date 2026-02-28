package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
	internalapp "github.com/meta-BE/bms-elsa/internal/app"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

type App struct {
	ctx         context.Context
	db          *sql.DB
	SongHandler *internalapp.SongHandler
	IRHandler   *internalapp.IRHandler
}

func NewApp() *App {
	return &App{}
}

// Init はDB初期化・マイグレーション・DI組み立てを行う。
// wails.Run()の前に呼ぶことで、Bind時にハンドラーが非nilであることを保証する。
func (a *App) Init() error {
	elsaDBPath := elsaDBPath()
	db, err := sql.Open("sqlite", elsaDBPath)
	if err != nil {
		return fmt.Errorf("elsa.db open: %w", err)
	}
	// ATTACH DATABASEはコネクション単位なので、同一コネクションを使い回す
	db.SetMaxOpenConns(1)
	a.db = db

	if err := persistence.RunMigrations(db); err != nil {
		return fmt.Errorf("migration: %w", err)
	}

	// songdata.db を ATTACH（パスは仮。将来的に設定画面で指定）
	songdataPath := songdataDBPath()
	if songdataPath != "" {
		if err := persistence.AttachSongdata(db, songdataPath); err != nil {
			// ATTACHできなくても起動は継続（楽曲一覧は空になる）
			fmt.Fprintf(os.Stderr, "songdata.db attach: %v\n", err)
		}
	}

	// DI組み立て
	elsaRepo := persistence.NewElsaRepository(db)
	songdataReader := persistence.NewSongdataReader(db, elsaRepo)
	irClient := gateway.NewLR2IRClient()

	listSongs := usecase.NewListSongsUseCase(songdataReader)
	getSongDetail := usecase.NewGetSongDetailUseCase(songdataReader)
	updateSongMeta := usecase.NewUpdateSongMetaUseCase(elsaRepo)
	updateChartMeta := usecase.NewUpdateChartMetaUseCase(elsaRepo)
	lookupIR := usecase.NewLookupIRUseCase(irClient, elsaRepo)

	a.SongHandler = internalapp.NewSongHandler(listSongs, getSongDetail, updateSongMeta)
	a.IRHandler = internalapp.NewIRHandler(lookupIR, updateChartMeta)

	return nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.SongHandler.SetContext(ctx)
	a.IRHandler.SetContext(ctx)
}

func (a *App) shutdown(ctx context.Context) {
	if a.db != nil {
		a.db.Close()
	}
}

// Config はアプリケーション設定
type Config struct {
	SongdataDBPath string `json:"songdataDBPath"`
}

// loadConfig は実行ファイルと同じディレクトリの config.json を読み込む。
// ファイルが存在しない場合はゼロ値の Config を返す。
func loadConfig() Config {
	path := filepath.Join(appDir(), "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "config.json のパースに失敗しました: %v\n", err)
		return Config{}
	}
	return cfg
}

// elsaDBPath は実行ファイルと同じディレクトリの elsa.db パスを返す
func elsaDBPath() string {
	return filepath.Join(appDir(), "elsa.db")
}

// appDir は実行ファイルと同じディレクトリを返す。
// config.jsonやelsa.dbの保存先として使用する。
func appDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// songdataDBPath はsongdata.dbのパスを返す。
// 優先順位: config.json → ~/.beatoraja/ → ~/beatoraja/
func songdataDBPath() string {
	cfg := loadConfig()
	if cfg.SongdataDBPath != "" {
		if _, err := os.Stat(cfg.SongdataDBPath); err == nil {
			return cfg.SongdataDBPath
		}
		fmt.Fprintf(os.Stderr, "config.json の songdataDBPath が見つかりません: %s\n", cfg.SongdataDBPath)
	}

	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".beatoraja", "songdata.db"),
		filepath.Join(home, "beatoraja", "songdata.db"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
