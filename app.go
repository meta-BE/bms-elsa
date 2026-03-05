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
	"github.com/meta-BE/bms-elsa/internal/app/dto"
	"github.com/meta-BE/bms-elsa/internal/domain/similarity"
	"github.com/meta-BE/bms-elsa/internal/usecase"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx         context.Context
	db          *sql.DB
	SongHandler      *internalapp.SongHandler
	IRHandler        *internalapp.IRHandler
	InferenceHandler *internalapp.InferenceHandler
	RewriteHandler   *internalapp.RewriteHandler
	dtRepo      *persistence.DifficultyTableRepository
	dtFetcher   *gateway.DifficultyTableFetcher
	songReader  *persistence.SongdataReader
	elsaRepo    *persistence.ElsaRepository
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
	a.dtRepo = persistence.NewDifficultyTableRepository(db)
	a.dtFetcher = gateway.NewDifficultyTableFetcher()
	songdataReader := persistence.NewSongdataReader(db, elsaRepo, a.dtRepo)
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

	return nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.SongHandler.SetContext(ctx)
	a.IRHandler.SetContext(ctx)
	a.InferenceHandler.SetContext(ctx)
	a.RewriteHandler.SetContext(ctx)
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

type DifficultyTableDTO struct {
	ID         int     `json:"id"`
	URL        string  `json:"url"`
	Name       string  `json:"name"`
	Symbol     string  `json:"symbol"`
	EntryCount int     `json:"entryCount"`
	FetchedAt  *string `json:"fetchedAt"`
}

type RefreshResult struct {
	TableName  string `json:"tableName"`
	Success    bool   `json:"success"`
	EntryCount int    `json:"entryCount"`
	Error      string `json:"error,omitempty"`
}

type DifficultyTableEntryDTO struct {
	MD5            string `json:"md5"`
	Level          string `json:"level"`
	Title          string `json:"title"`
	Artist         string `json:"artist"`
	URL            string `json:"url"`
	URLDiff        string `json:"urlDiff"`
	Status         string `json:"status"`
	InstalledCount int    `json:"installedCount"`
}

func (a *App) GetDifficultyTableEntry(tableID int, md5 string) (*DifficultyTableEntryDTO, error) {
	entry, err := a.dtRepo.GetEntry(a.ctx, tableID, md5)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	counts, err := a.songReader.CountChartsByMD5s(a.ctx, []string{md5})
	if err != nil {
		return nil, err
	}

	count := 0
	if counts != nil {
		count = counts[md5]
	}
	status := "not_installed"
	if count == 1 {
		status = "installed"
	} else if count > 1 {
		status = "duplicate"
	}

	result := DifficultyTableEntryDTO{
		MD5: entry.MD5, Level: entry.Level, Title: entry.Title, Artist: entry.Artist,
		URL: entry.URL, URLDiff: entry.URLDiff,
		Status: status, InstalledCount: count,
	}
	return &result, nil
}

func (a *App) ListDifficultyTableEntries(tableID int) ([]DifficultyTableEntryDTO, error) {
	entries, err := a.dtRepo.ListEntries(a.ctx, tableID)
	if err != nil {
		return nil, err
	}

	md5s := make([]string, len(entries))
	for i, e := range entries {
		md5s[i] = e.MD5
	}

	counts, err := a.songReader.CountChartsByMD5s(a.ctx, md5s)
	if err != nil {
		return nil, err
	}

	result := make([]DifficultyTableEntryDTO, len(entries))
	for i, e := range entries {
		count := 0
		if counts != nil {
			count = counts[e.MD5]
		}
		status := "not_installed"
		if count == 1 {
			status = "installed"
		} else if count > 1 {
			status = "duplicate"
		}
		result[i] = DifficultyTableEntryDTO{
			MD5: e.MD5, Level: e.Level, Title: e.Title, Artist: e.Artist,
			URL: e.URL, URLDiff: e.URLDiff,
			Status: status, InstalledCount: count,
		}
	}
	return result, nil
}

// ListCharts はsongdata.dbの全譜面一覧を返す
func (a *App) ListCharts() ([]dto.ChartListItemDTO, error) {
	charts, err := a.songReader.ListAllCharts(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.ChartListItemDTO, len(charts))
	for i, c := range charts {
		result[i] = dto.ChartListItemDTO{
			MD5:        c.MD5,
			Title:      c.Title,
			Subtitle:   c.Subtitle,
			Artist:     c.Artist,
			SubArtist:  c.SubArtist,
			Genre:      c.Genre,
			MinBPM:     c.MinBPM,
			MaxBPM:     c.MaxBPM,
			Difficulty: c.Difficulty,
			HasIRMeta:  c.HasIRMeta,
		}
		if c.EventName != nil {
			result[i].EventName = *c.EventName
		}
		if c.ReleaseYear != nil {
			result[i].ReleaseYear = *c.ReleaseYear
		}
	}
	return result, nil
}

func (a *App) GetChartDetailByMD5(md5 string) (*dto.ChartDTO, error) {
	chart, err := a.songReader.GetChartByMD5(a.ctx, md5)
	if err != nil {
		return nil, err
	}
	if chart == nil {
		return nil, nil
	}
	result := dto.ChartToDTO(*chart)
	return &result, nil
}

// GetChartMetaByMD5 はchart_metaテーブルからIR情報のみを取得する（未導入譜面用）
func (a *App) GetChartMetaByMD5(md5 string) (*dto.ChartIRMetaDTO, error) {
	meta, err := a.elsaRepo.GetChartMeta(a.ctx, md5)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, nil
	}
	result := dto.ChartIRMetaToDTO(*meta)
	return &result, nil
}

func (a *App) ListDifficultyTables() ([]DifficultyTableDTO, error) {
	tables, err := a.dtRepo.ListTables(a.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]DifficultyTableDTO, len(tables))
	for i, t := range tables {
		count, _ := a.dtRepo.CountEntries(a.ctx, t.ID)
		var fetchedAt *string
		if t.FetchedAt != nil {
			s := t.FetchedAt.Local().Format("2006-01-02 15:04")
			fetchedAt = &s
		}
		result[i] = DifficultyTableDTO{
			ID: t.ID, URL: t.URL, Name: t.Name, Symbol: t.Symbol,
			EntryCount: count, FetchedAt: fetchedAt,
		}
	}
	return result, nil
}

func (a *App) AddDifficultyTable(tableURL string) error {
	// 1. HTMLからheader URLを取得
	headerURL, err := a.dtFetcher.FetchHeaderURL(tableURL)
	if err != nil {
		return err
	}

	// 2. header.jsonを取得
	header, err := a.dtFetcher.FetchHeader(headerURL)
	if err != nil {
		return err
	}

	// 3. body JSONを取得
	entries, err := a.dtFetcher.FetchBody(header.DataURL)
	if err != nil {
		return err
	}

	// 4. DBに保存
	tableID, err := a.dtRepo.InsertTable(a.ctx, persistence.DifficultyTable{
		URL: tableURL, HeaderURL: headerURL, DataURL: header.DataURL,
		Name: header.Name, Symbol: header.Symbol,
	})
	if err != nil {
		return err
	}

	// 5. エントリを保存
	dbEntries := make([]persistence.DifficultyTableEntry, len(entries))
	for i, e := range entries {
		dbEntries[i] = persistence.DifficultyTableEntry{
			TableID: tableID, MD5: e.MD5, Level: e.Level,
			Title: e.Title, Artist: e.Artist,
			URL: e.URL, URLDiff: e.URLDiff,
		}
	}
	return a.dtRepo.ReplaceEntries(a.ctx, tableID, dbEntries)
}

func (a *App) RemoveDifficultyTable(id int) error {
	return a.dtRepo.DeleteTable(a.ctx, id)
}

func (a *App) RefreshDifficultyTable(id int) RefreshResult {
	tables, err := a.dtRepo.ListTables(a.ctx)
	if err != nil {
		return RefreshResult{Success: false, Error: err.Error()}
	}

	var target *persistence.DifficultyTable
	for _, t := range tables {
		if t.ID == id {
			target = &t
			break
		}
	}
	if target == nil {
		return RefreshResult{Success: false, Error: "テーブルが見つかりません"}
	}

	return a.refreshTable(*target)
}

func (a *App) RefreshAllDifficultyTables() []RefreshResult {
	tables, err := a.dtRepo.ListTables(a.ctx)
	if err != nil {
		return []RefreshResult{{Success: false, Error: err.Error()}}
	}

	results := make([]RefreshResult, len(tables))
	for i, t := range tables {
		results[i] = a.refreshTable(t)
	}
	return results
}

func (a *App) refreshTable(t persistence.DifficultyTable) RefreshResult {
	result := RefreshResult{TableName: t.Name}

	// header再取得（data_url変更に追従）
	header, err := a.dtFetcher.FetchHeader(t.HeaderURL)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	// data_urlが変わっていれば更新
	if header.DataURL != t.DataURL {
		t.DataURL = header.DataURL
	}
	t.Name = header.Name
	t.Symbol = header.Symbol

	// body取得
	entries, err := a.dtFetcher.FetchBody(t.DataURL)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	// テーブルメタ更新
	if err := a.dtRepo.UpdateTable(a.ctx, t); err != nil {
		result.Error = err.Error()
		return result
	}

	// エントリ全件置換
	dbEntries := make([]persistence.DifficultyTableEntry, len(entries))
	for i, e := range entries {
		dbEntries[i] = persistence.DifficultyTableEntry{
			TableID: t.ID, MD5: e.MD5, Level: e.Level,
			Title: e.Title, Artist: e.Artist,
			URL: e.URL, URLDiff: e.URLDiff,
		}
	}
	if err := a.dtRepo.ReplaceEntries(a.ctx, t.ID, dbEntries); err != nil {
		result.Error = err.Error()
		return result
	}

	result.Success = true
	result.EntryCount = len(entries)
	return result
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
