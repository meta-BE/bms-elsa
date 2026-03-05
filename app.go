package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	_ "modernc.org/sqlite"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
	internalapp "github.com/meta-BE/bms-elsa/internal/app"
	"github.com/meta-BE/bms-elsa/internal/domain/similarity"
	"github.com/meta-BE/bms-elsa/internal/usecase"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx                    context.Context
	db                     *sql.DB
	SongHandler            *internalapp.SongHandler
	IRHandler              *internalapp.IRHandler
	InferenceHandler       *internalapp.InferenceHandler
	RewriteHandler         *internalapp.RewriteHandler
	ChartHandler           *internalapp.ChartHandler
	DifficultyTableHandler *internalapp.DifficultyTableHandler
	songReader             *persistence.SongdataReader
	elsaRepo               *persistence.ElsaRepository
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
	a.elsaRepo = elsaRepo
	dtRepo := persistence.NewDifficultyTableRepository(db)
	dtFetcher := gateway.NewDifficultyTableFetcher()
	songdataReader := persistence.NewSongdataReader(db, elsaRepo, dtRepo)
	a.songReader = songdataReader
	irClient := gateway.NewLR2IRClient()

	listSongs := usecase.NewListSongsUseCase(songdataReader)
	getSongDetail := usecase.NewGetSongDetailUseCase(songdataReader)
	updateSongMeta := usecase.NewUpdateSongMetaUseCase(elsaRepo)
	updateChartMeta := usecase.NewUpdateChartMetaUseCase(elsaRepo)
	lookupIR := usecase.NewLookupIRUseCase(irClient, elsaRepo)
	bulkFetchIR := usecase.NewBulkFetchIRUseCase(irClient, elsaRepo)

	a.SongHandler = internalapp.NewSongHandler(listSongs, getSongDetail, updateSongMeta)
	a.IRHandler = internalapp.NewIRHandler(lookupIR, bulkFetchIR, updateChartMeta, elsaRepo)

	inferMeta := usecase.NewInferSongMetaUseCase(elsaRepo)
	a.InferenceHandler = internalapp.NewInferenceHandler(inferMeta, elsaRepo)

	inferWorkingURLs := usecase.NewInferWorkingURLUseCase(elsaRepo)
	a.RewriteHandler = internalapp.NewRewriteHandler(inferWorkingURLs, elsaRepo)

	a.ChartHandler = internalapp.NewChartHandler(songdataReader, elsaRepo)
	estimateInstallLocation := usecase.NewEstimateInstallLocationUseCase(songdataReader, elsaRepo)
	a.DifficultyTableHandler = internalapp.NewDifficultyTableHandler(dtRepo, dtFetcher, songdataReader, estimateInstallLocation)

	return nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.SongHandler.SetContext(ctx)
	a.IRHandler.SetContext(ctx)
	a.InferenceHandler.SetContext(ctx)
	a.RewriteHandler.SetContext(ctx)
	a.ChartHandler.SetContext(ctx)
	a.DifficultyTableHandler.SetContext(ctx)
}

func (a *App) shutdown(ctx context.Context) {
	if a.db != nil {
		a.db.Close()
	}
}

// OpenURL はシステムブラウザでURLを開く（Wails v2.11.0のURL検証が~等を拒否するためバイパス）
func (a *App) OpenURL(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

// OpenFolder は譜面ファイルのパスを受け取り、その親ディレクトリをOSのファイルマネージャで開く
func (a *App) OpenFolder(filePath string) error {
	dir := filepath.Dir(filePath)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("フォルダにアクセスできません: %s (%w)", dir, err)
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", dir).Start()
	case "windows":
		return exec.Command("explorer", dir).Start()
	default:
		return exec.Command("xdg-open", dir).Start()
	}
}

// GetConfig は現在のconfig.jsonを読んで返す
func (a *App) GetConfig() Config {
	return loadConfig()
}

// SaveConfig はconfig.jsonに設定を書き込む
func (a *App) SaveConfig(cfg Config) error {
	path := filepath.Join(appDir(), "config.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("config write: %w", err)
	}
	return nil
}

// SelectFile はOSネイティブのファイル選択ダイアログを開き、選択されたパスを返す
func (a *App) SelectFile() (string, error) {
	return wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "songdata.db を選択",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "SQLite Database", Pattern: "*.db"},
		},
	})
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

// ScanDuplicates は楽曲の重複スキャンを実行する
func (a *App) ScanDuplicates() ([]similarity.DuplicateGroup, error) {
	groups, err := a.songReader.ListSongGroupsForDuplicateScan(a.ctx)
	if err != nil {
		return nil, err
	}

	songs := make([]similarity.SongInfo, len(groups))
	for i, g := range groups {
		songs[i] = similarity.SongInfo{
			FolderHash: g.FolderHash,
			Title:      g.Title,
			Artist:     g.Artist,
			Genre:      g.Genre,
			MinBPM:     g.MinBPM,
			MaxBPM:     g.MaxBPM,
			ChartCount: g.ChartCount,
			Path:       g.Path,
		}
	}

	return similarity.FindDuplicateGroups(songs, 60), nil
}
