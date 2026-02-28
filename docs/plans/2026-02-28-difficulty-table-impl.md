# BMS難易度表の取り込み・表示 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** BMS難易度表のデータをelsa.dbに保存し、設定画面から管理、譜面詳細に難易度ラベルを表示する

**Architecture:** elsa.dbに2テーブル（difficulty_table, difficulty_table_entry）を追加。App構造体に難易度表CRUD + HTTP取得ロジックを追加。Settings.svelteに管理UI、SongDetail.svelteにバッジ表示を追加。

**Tech Stack:** Go 1.24, Wails v2, Svelte 4, DaisyUI v5, net/http (HTML/JSON取得), SQLite

---

### Task 1: DBマイグレーションに難易度表テーブルを追加

**Files:**
- Modify: `internal/adapter/persistence/migrations.go`

**Step 1: migrations.goにCREATE TABLEを追加**

`RunMigrations` の `statements` スライスに以下を追加:

```go
`CREATE TABLE IF NOT EXISTS difficulty_table (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    url         TEXT NOT NULL UNIQUE,
    header_url  TEXT NOT NULL,
    data_url    TEXT NOT NULL,
    name        TEXT NOT NULL,
    symbol      TEXT NOT NULL,
    fetched_at  TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
)`,
`CREATE TABLE IF NOT EXISTS difficulty_table_entry (
    table_id    INTEGER NOT NULL REFERENCES difficulty_table(id) ON DELETE CASCADE,
    md5         TEXT NOT NULL,
    level       TEXT NOT NULL,
    title       TEXT,
    artist      TEXT,
    url         TEXT,
    url_diff    TEXT,
    PRIMARY KEY (table_id, md5)
)`,
`CREATE INDEX IF NOT EXISTS idx_dte_md5 ON difficulty_table_entry(md5)`,
```

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: ビルド成功

**Step 3: コミット**

```bash
git add internal/adapter/persistence/migrations.go
git commit -m "難易度表用テーブル（difficulty_table, difficulty_table_entry）をマイグレーションに追加"
```

### Task 2: 難易度表リポジトリ層を実装

**Files:**
- Create: `internal/adapter/persistence/difficulty_table_repository.go`

**Step 1: DifficultyTableRepositoryを作成**

```go
package persistence

import (
	"context"
	"database/sql"
	"time"
)

// DifficultyTable は難易度表マスタ
type DifficultyTable struct {
	ID        int
	URL       string
	HeaderURL string
	DataURL   string
	Name      string
	Symbol    string
	FetchedAt *time.Time
}

// DifficultyTableEntry は難易度表の譜面エントリ
type DifficultyTableEntry struct {
	TableID int
	MD5     string
	Level   string
	Title   string
	Artist  string
	URL     string
	URLDiff string
}

// DifficultyLabel は譜面に紐づく難易度ラベル（JOINで取得）
type DifficultyLabel struct {
	TableName string
	Symbol    string
	Level     string
}

type DifficultyTableRepository struct {
	db *sql.DB
}

func NewDifficultyTableRepository(db *sql.DB) *DifficultyTableRepository {
	return &DifficultyTableRepository{db: db}
}

func (r *DifficultyTableRepository) ListTables(ctx context.Context) ([]DifficultyTable, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, url, header_url, data_url, name, symbol, fetched_at
		FROM difficulty_table
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []DifficultyTable
	for rows.Next() {
		var t DifficultyTable
		var fetchedAt sql.NullString
		if err := rows.Scan(&t.ID, &t.URL, &t.HeaderURL, &t.DataURL, &t.Name, &t.Symbol, &fetchedAt); err != nil {
			return nil, err
		}
		if fetchedAt.Valid {
			parsed, _ := time.Parse(timeLayout, fetchedAt.String)
			t.FetchedAt = &parsed
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

func (r *DifficultyTableRepository) InsertTable(ctx context.Context, t DifficultyTable) (int, error) {
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO difficulty_table (url, header_url, data_url, name, symbol, fetched_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'))
	`, t.URL, t.HeaderURL, t.DataURL, t.Name, t.Symbol)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return int(id), err
}

func (r *DifficultyTableRepository) UpdateTable(ctx context.Context, t DifficultyTable) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE difficulty_table
		SET header_url = ?, data_url = ?, name = ?, symbol = ?, fetched_at = datetime('now'), updated_at = datetime('now')
		WHERE id = ?
	`, t.HeaderURL, t.DataURL, t.Name, t.Symbol, t.ID)
	return err
}

func (r *DifficultyTableRepository) DeleteTable(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM difficulty_table WHERE id = ?`, id)
	return err
}

func (r *DifficultyTableRepository) ReplaceEntries(ctx context.Context, tableID int, entries []DifficultyTableEntry) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM difficulty_table_entry WHERE table_id = ?`, tableID); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO difficulty_table_entry (table_id, md5, level, title, artist, url, url_diff)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		if _, err := stmt.ExecContext(ctx, tableID, e.MD5, e.Level, e.Title, e.Artist, e.URL, e.URLDiff); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *DifficultyTableRepository) CountEntries(ctx context.Context, tableID int) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM difficulty_table_entry WHERE table_id = ?`, tableID).Scan(&count)
	return count, err
}

func (r *DifficultyTableRepository) GetLabelsByMD5(ctx context.Context, md5 string) ([]DifficultyLabel, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT dt.name, dt.symbol, dte.level
		FROM difficulty_table_entry dte
		JOIN difficulty_table dt ON dt.id = dte.table_id
		WHERE dte.md5 = ?
		ORDER BY dt.name
	`, md5)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var labels []DifficultyLabel
	for rows.Next() {
		var l DifficultyLabel
		if err := rows.Scan(&l.TableName, &l.Symbol, &l.Level); err != nil {
			return nil, err
		}
		labels = append(labels, l)
	}
	return labels, rows.Err()
}

// GetLabelsByMD5s は複数md5の難易度ラベルをまとめて取得する（N+1回避）
func (r *DifficultyTableRepository) GetLabelsByMD5s(ctx context.Context, md5s []string) (map[string][]DifficultyLabel, error) {
	if len(md5s) == 0 {
		return nil, nil
	}

	// プレースホルダ構築
	placeholders := make([]string, len(md5s))
	args := make([]interface{}, len(md5s))
	for i, m := range md5s {
		placeholders[i] = "?"
		args[i] = m
	}

	query := `
		SELECT dte.md5, dt.name, dt.symbol, dte.level
		FROM difficulty_table_entry dte
		JOIN difficulty_table dt ON dt.id = dte.table_id
		WHERE dte.md5 IN (` + joinStrings(placeholders, ",") + `)
		ORDER BY dt.name
	`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]DifficultyLabel)
	for rows.Next() {
		var md5 string
		var l DifficultyLabel
		if err := rows.Scan(&md5, &l.TableName, &l.Symbol, &l.Level); err != nil {
			return nil, err
		}
		result[md5] = append(result[md5], l)
	}
	return result, rows.Err()
}

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
```

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: ビルド成功

**Step 3: コミット**

```bash
git add internal/adapter/persistence/difficulty_table_repository.go
git commit -m "難易度表のリポジトリ層を追加"
```

### Task 3: HTML/JSONフェッチャーを実装

**Files:**
- Create: `internal/adapter/gateway/difficulty_table_fetcher.go`

**Step 1: フェッチャーを作成**

HTML取得 → metaパース → header.json取得 → body JSON取得のロジック。

```go
package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// DifficultyTableHeader はheader.jsonの構造
type DifficultyTableHeader struct {
	Name    string `json:"name"`
	Symbol  string `json:"symbol"`
	DataURL string `json:"data_url"`
}

// DifficultyTableBodyEntry はbody JSONの1エントリ
type DifficultyTableBodyEntry struct {
	MD5     string `json:"md5"`
	Level   string `json:"level"`
	Title   string `json:"title"`
	Artist  string `json:"artist"`
	URL     string `json:"url"`
	URLDiff string `json:"url_diff"`
}

type DifficultyTableFetcher struct {
	client *http.Client
}

func NewDifficultyTableFetcher() *DifficultyTableFetcher {
	return &DifficultyTableFetcher{client: &http.Client{}}
}

var bmstableMetaRe = regexp.MustCompile(`<meta\s+name=["']bmstable["']\s+content=["']([^"']+)["']`)

// FetchHeaderURL はHTMLからbmstableメタタグのURLを取得する
func (f *DifficultyTableFetcher) FetchHeaderURL(tableURL string) (string, error) {
	body, err := f.get(tableURL)
	if err != nil {
		return "", fmt.Errorf("HTML取得失敗: %w", err)
	}

	matches := bmstableMetaRe.FindStringSubmatch(body)
	if len(matches) < 2 {
		return "", fmt.Errorf("bmstableメタタグが見つかりません")
	}

	return resolveURL(tableURL, matches[1])
}

// FetchHeader はheader.jsonを取得する
func (f *DifficultyTableFetcher) FetchHeader(headerURL string) (*DifficultyTableHeader, error) {
	body, err := f.get(headerURL)
	if err != nil {
		return nil, fmt.Errorf("header.json取得失敗: %w", err)
	}

	var header DifficultyTableHeader
	if err := json.Unmarshal([]byte(body), &header); err != nil {
		return nil, fmt.Errorf("header.jsonパース失敗: %w", err)
	}

	// data_urlを絶対URLに変換
	absDataURL, err := resolveURL(headerURL, header.DataURL)
	if err != nil {
		return nil, fmt.Errorf("data_url解決失敗: %w", err)
	}
	header.DataURL = absDataURL

	return &header, nil
}

// FetchBody はbody JSONを取得する
func (f *DifficultyTableFetcher) FetchBody(dataURL string) ([]DifficultyTableBodyEntry, error) {
	body, err := f.get(dataURL)
	if err != nil {
		return nil, fmt.Errorf("body JSON取得失敗: %w", err)
	}

	var entries []DifficultyTableBodyEntry
	if err := json.Unmarshal([]byte(body), &entries); err != nil {
		return nil, fmt.Errorf("body JSONパース失敗: %w", err)
	}

	return entries, nil
}

func (f *DifficultyTableFetcher) get(targetURL string) (string, error) {
	resp, err := f.client.Get(targetURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, targetURL)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// resolveURL はbaseURLに対してrefを解決する
func resolveURL(baseURL, ref string) (string, error) {
	// 既に絶対URLならそのまま返す
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref, nil
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	refURL, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(refURL).String(), nil
}
```

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: ビルド成功

**Step 3: コミット**

```bash
git add internal/adapter/gateway/difficulty_table_fetcher.go
git commit -m "難易度表のHTML/JSONフェッチャーを追加"
```

### Task 4: AppにDifficultyTable APIメソッドを追加

**Files:**
- Modify: `app.go`
- Modify: `main.go`

**Step 1: app.goにフィールドとメソッドを追加**

App構造体にフィールドを追加:

```go
type App struct {
	ctx         context.Context
	db          *sql.DB
	SongHandler *internalapp.SongHandler
	IRHandler   *internalapp.IRHandler
	dtRepo      *persistence.DifficultyTableRepository
	dtFetcher   *gateway.DifficultyTableFetcher
}
```

`Init()` メソッド内でDI:

```go
a.dtRepo = persistence.NewDifficultyTableRepository(db)
a.dtFetcher = gateway.NewDifficultyTableFetcher()
```

DTO型とAPIメソッドを追加:

```go
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
			s := t.FetchedAt.Format("2006-01-02 15:04")
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
```

importに追加:

```go
"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
```

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: ビルド成功

**Step 3: コミット**

```bash
git add app.go
git commit -m "難易度表の追加・削除・更新APIをAppに追加"
```

### Task 5: GetSongDetailに難易度ラベルを組み込む

**Files:**
- Modify: `internal/app/dto/dto.go`
- Modify: `internal/adapter/persistence/songdata_reader.go`

**Step 1: DTOにDifficultyLabelを追加**

`dto.go` に型追加:

```go
type DifficultyLabelDTO struct {
	TableName string `json:"tableName"`
	Symbol    string `json:"symbol"`
	Level     string `json:"level"`
}
```

`ChartDTO` にフィールド追加:

```go
DifficultyLabels []DifficultyLabelDTO `json:"difficultyLabels,omitempty"`
```

**Step 2: SongdataReaderにDifficultyTableRepositoryを注入**

`SongdataReader` にフィールド追加:

```go
type SongdataReader struct {
	db       *sql.DB
	metaRepo *ElsaRepository
	dtRepo   *DifficultyTableRepository
}

func NewSongdataReader(db *sql.DB, metaRepo *ElsaRepository, dtRepo *DifficultyTableRepository) *SongdataReader {
	return &SongdataReader{db: db, metaRepo: metaRepo, dtRepo: dtRepo}
}
```

**Step 3: GetSongByFolderで難易度ラベルを取得**

`GetSongByFolder` メソッドの chart_meta付与ループの後に追加:

```go
// 難易度ラベルを一括取得（N+1回避）
md5s := make([]string, len(charts))
for i, c := range charts {
	md5s[i] = c.MD5
}
labelsMap, err := r.dtRepo.GetLabelsByMD5s(ctx, md5s)
if err != nil {
	return nil, fmt.Errorf("GetSongByFolder GetLabelsByMD5s: %w", err)
}
for i := range charts {
	charts[i].DifficultyLabels = labelsMap[charts[i].MD5]
}
```

**Step 4: Chartモデルにフィールド追加**

`internal/domain/model/song.go` の `Chart` に追加:

```go
DifficultyLabels []DifficultyLabel
```

`DifficultyLabel` 型を追加:

```go
type DifficultyLabel struct {
	TableName string
	Symbol    string
	Level     string
}
```

**Step 5: DTOの変換でラベルをマッピング**

`dto.go` の `ChartToDTO` でラベルを変換:

```go
if c.DifficultyLabels != nil {
	d.DifficultyLabels = make([]DifficultyLabelDTO, len(c.DifficultyLabels))
	for i, l := range c.DifficultyLabels {
		d.DifficultyLabels[i] = DifficultyLabelDTO{
			TableName: l.TableName, Symbol: l.Symbol, Level: l.Level,
		}
	}
}
```

**Step 6: app.goのDI箇所を更新**

`Init()` 内の `NewSongdataReader` 呼び出しを更新:

```go
songdataReader := persistence.NewSongdataReader(db, elsaRepo, a.dtRepo)
```

**Step 7: persistenceのGetLabelsByMD5sの戻り値型をmodel.DifficultyLabelに変更**

`difficulty_table_repository.go` の `GetLabelsByMD5s` が返す型をモデル層の型に合わせる。
importに `"github.com/meta-BE/bms-elsa/internal/domain/model"` を追加し、`DifficultyLabel` → `model.DifficultyLabel` に変更。
persistence層の `DifficultyLabel` 型定義は削除。

**Step 8: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: ビルド成功

**Step 9: コミット**

```bash
git add internal/domain/model/song.go internal/app/dto/dto.go internal/adapter/persistence/songdata_reader.go internal/adapter/persistence/difficulty_table_repository.go app.go
git commit -m "GetSongDetailに難易度ラベルを組み込み"
```

### Task 6: Settings.svelteに難易度表管理UIを追加

**Files:**
- Modify: `frontend/src/Settings.svelte`

**Step 1: 難易度表管理セクションを追加**

scriptセクションにimportと状態変数を追加:

```typescript
import { ListDifficultyTables, AddDifficultyTable, RemoveDifficultyTable, RefreshAllDifficultyTables } from '../wailsjs/go/main/App'

let tables: any[] = []
let newTableURL = ''
let addError = ''
let refreshResults: any[] | null = null
let refreshing = false
let adding = false

async function loadTables() {
  try {
    tables = await ListDifficultyTables() || []
  } catch (e) {
    tables = []
  }
}

async function handleAddTable() {
  if (!newTableURL.trim()) return
  addError = ''
  adding = true
  try {
    await AddDifficultyTable(newTableURL.trim())
    newTableURL = ''
    await loadTables()
  } catch (e: any) {
    addError = e?.message || '追加に失敗しました'
  } finally {
    adding = false
  }
}

async function handleRemoveTable(id: number) {
  await RemoveDifficultyTable(id)
  await loadTables()
}

async function handleRefreshAll() {
  refreshing = true
  refreshResults = null
  try {
    refreshResults = await RefreshAllDifficultyTables()
    await loadTables()
  } catch (e: any) {
    refreshResults = [{ tableName: '', success: false, error: e?.message || '更新に失敗しました' }]
  } finally {
    refreshing = false
  }
}
```

`open()` 関数内に `loadTables()` 呼び出しを追加:

```typescript
export async function open() {
  saved = false
  error = ''
  refreshResults = null
  try {
    const cfg = await GetConfig()
    songdataDBPath = cfg.songdataDBPath || ''
  } catch (e) {
    songdataDBPath = ''
  }
  await loadTables()
  dialog.showModal()
}
```

テンプレートの `modal-action` の前に難易度表セクションを追加:

```svelte
<div class="divider"></div>
<h3 class="text-lg font-bold mb-2">難易度表</h3>

<!-- 登録済みテーブル一覧 -->
{#if tables.length > 0}
  <div class="overflow-x-auto">
    <table class="table table-xs">
      <thead>
        <tr>
          <th>名前</th>
          <th>記号</th>
          <th>譜面数</th>
          <th>最終取得</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {#each tables as t}
          <tr>
            <td>{t.name}</td>
            <td>{t.symbol}</td>
            <td>{t.entryCount}</td>
            <td class="text-xs text-base-content/50">{t.fetchedAt || '未取得'}</td>
            <td>
              <button class="btn btn-ghost btn-xs text-error" on:click={() => handleRemoveTable(t.id)}>削除</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{:else}
  <p class="text-sm text-base-content/50">難易度表が登録されていません</p>
{/if}

<!-- 追加フォーム -->
<div class="flex gap-2 mt-2">
  <input
    type="text"
    class="input input-bordered input-sm flex-1"
    bind:value={newTableURL}
    placeholder="https://stellabms.xyz/st/table.html"
    on:keydown={(e) => e.key === 'Enter' && handleAddTable()}
  />
  <button class="btn btn-sm btn-outline" on:click={handleAddTable} disabled={adding}>
    {adding ? '追加中...' : '追加'}
  </button>
</div>
{#if addError}
  <div class="alert alert-error mt-2 py-1 text-sm">{addError}</div>
{/if}

<!-- 全更新ボタン -->
{#if tables.length > 0}
  <button class="btn btn-sm btn-outline mt-2" on:click={handleRefreshAll} disabled={refreshing}>
    {refreshing ? '更新中...' : '全て更新'}
  </button>
{/if}

<!-- 更新結果 -->
{#if refreshResults}
  <div class="mt-2 text-sm space-y-1">
    {#each refreshResults as r}
      <div class="flex items-center gap-2">
        <span class={r.success ? 'text-success' : 'text-error'}>{r.success ? '✓' : '✗'}</span>
        <span>{r.tableName}</span>
        {#if r.success}
          <span class="text-base-content/50">{r.entryCount}件</span>
        {:else}
          <span class="text-error">{r.error}</span>
        {/if}
      </div>
    {/each}
  </div>
{/if}
```

**Step 2: コミット**

```bash
git add frontend/src/Settings.svelte
git commit -m "設定画面に難易度表管理UIを追加"
```

### Task 7: SongDetail.svelteに難易度バッジを表示

**Files:**
- Modify: `frontend/src/SongDetail.svelte`

**Step 1: 譜面一覧に難易度ラベルバッジを追加**

譜面一覧の各行（`☆{chart.level}` の後、`md5` の前）にバッジを追加:

```svelte
<span class="w-8">☆{chart.level}</span>
{#if chart.difficultyLabels?.length}
  {#each chart.difficultyLabels as label}
    <span class="badge badge-sm badge-outline" title={label.tableName}>{label.symbol}{label.level}</span>
  {/each}
{/if}
<span class="flex-1 truncate text-base-content/50">{chart.md5.slice(0, 8)}...</span>
```

**Step 2: コミット**

```bash
git add frontend/src/SongDetail.svelte
git commit -m "譜面一覧に難易度表バッジを表示"
```

### Task 8: ビルド・動作確認

**Step 1: wails devで動作確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails dev`

確認項目:
1. 設定画面を開くと難易度表セクションが表示される
2. URL入力して「追加」→ 難易度表が登録される
3. テーブル一覧に名前・記号・譜面数が表示される
4. 「全て更新」→ 結果が成功/失敗で表示される
5. 「削除」→ テーブルが消える
6. 譜面詳細で難易度ラベルバッジが表示される（該当譜面のみ）
