# BMS Search連携によるイベント情報管理 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** イベント情報の管理をURLパターンマッチング方式からBMS Search API連携に切り替え、eventマスターテーブルを導入する。

**Architecture:** eventマスターテーブルにBMS Searchのexhibition情報を保持し、song_metaからevent_idで参照する。BMS Search APIクライアント（Go HTTP）で譜面→BMS→exhibitionの連鎖的取得を行い、Wails Eventsで進捗通知するバックグラウンド同期を実装する。旧event_mapping方式は完全に廃止する。

**Tech Stack:** Go 1.24, Wails v2, Svelte 4, TypeScript, SQLite

---

## ファイル構成

| 操作 | ファイル | 責務 |
|------|---------|------|
| 新規 | `cmd/gen-events/main.go` | event.csv生成CLIツール |
| 新規 | `internal/adapter/persistence/events.csv` | eventマスター初期データ |
| 新規 | `internal/adapter/gateway/bmssearch_client.go` | BMS Search APIクライアント |
| 新規 | `internal/adapter/gateway/bmssearch_client_test.go` | テスト |
| 新規 | `internal/usecase/sync_bmssearch.go` | BMS Search同期ユースケース |
| 新規 | `internal/app/event_handler.go` | イベント管理Wailsバインディング |
| 新規 | `frontend/src/settings/EventManager.svelte` | イベントマスター管理UI |
| 変更 | `internal/adapter/persistence/migrations.go` | eventテーブル追加、song_meta変更、event_mapping削除 |
| 変更 | `internal/adapter/persistence/elsa_repository.go` | song_meta CRUD変更、event CRUD追加 |
| 変更 | `internal/domain/model/song.go` | Event型追加、SongMeta変更、EventMapping削除 |
| 変更 | `internal/domain/model/repository.go` | MetaRepositoryインターフェース変更 |
| 変更 | `internal/app/song_handler.go` | UpdateSongMetaシグネチャ変更 |
| 変更 | `internal/usecase/update_song_meta.go` | シグネチャ変更 |
| 変更 | `app.go` | DI変更（EventHandler追加、InferenceHandler削除） |
| 変更 | `frontend/src/views/SongDetail.svelte` | オートコンプリート付きイベント選択 |
| 変更 | `frontend/src/views/SongTable.svelte` | 表示ロジック変更 |
| 変更 | `frontend/src/App.svelte` | EventHandler追加、InferenceHandler削除 |
| 削除 | `internal/adapter/persistence/event_mappings.csv` | 旧マッピングCSV |
| 削除 | `internal/usecase/infer_meta.go` | 旧推測ユースケース |
| 削除 | `internal/usecase/infer_meta_test.go` | 旧テスト |
| 削除 | `internal/app/inference_handler.go` | 旧ハンドラー |
| 削除 | `frontend/src/settings/EventMappingManager.svelte` | 旧マッピング管理UI |
| 削除 | `frontend/src/settings/InferenceModal.svelte` | 旧推測モーダル |
| 自動生成 | `frontend/wailsjs/go/app/EventHandler.{js,d.ts}` | Wailsバインディング |

---

### Task 1: event.csv生成CLIツール

BMS Search APIの全exhibition一覧と既存event_mappings.csvをマージして、event.csvを生成するCLIツールを作成する。生成後にユーザーが手動でshort_nameを確認・編集する想定。

**Files:**
- Create: `cmd/gen-events/main.go`
- Output: `internal/adapter/persistence/events.csv`

- [ ] **Step 1: CLIツールの骨格を作成**

```go
// cmd/gen-events/main.go
package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

const baseURL = "https://api.bmssearch.net/v1"

type Exhibition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ExhibitionDetail struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Terms     *Terms `json:"terms"`
	CreatedAt string `json:"createdAt"`
}

type Terms struct {
	Entry *Period `json:"entry"`
}

type Period struct {
	StartsAt string `json:"startsAt"`
}

func main() {
	fmt.Println("BMS Search からイベント一覧を取得中...")
	exhibitions, err := fetchAllExhibitions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "取得失敗: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%d 件のイベントを取得\n", len(exhibitions))

	// 既存event_mappings.csvを読み込み（short_name用）
	oldMappings, err := loadOldMappings("internal/adapter/persistence/event_mappings.csv")
	if err != nil {
		fmt.Fprintf(os.Stderr, "event_mappings.csv 読み込み失敗: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("既存マッピング: %d 件\n", len(oldMappings))

	// events.csvに出力
	outPath := "internal/adapter/persistence/events.csv"
	if err := writeEventsCSV(outPath, exhibitions, oldMappings); err != nil {
		fmt.Fprintf(os.Stderr, "CSV書き込み失敗: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("出力完了: %s\n", outPath)
}

func fetchAllExhibitions() ([]ExhibitionDetail, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	var all []ExhibitionDetail
	offset := 0
	limit := 100

	for {
		url := fmt.Sprintf("%s/exhibitions/search?offset=%d&limit=%d", baseURL, offset, limit)
		resp, err := client.Get(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var batch []ExhibitionDetail
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, fmt.Errorf("JSON parse: %w", err)
		}

		all = append(all, batch...)
		fmt.Printf("  %d 件取得 (offset=%d)\n", len(batch), offset)

		if len(batch) < limit {
			break
		}
		offset += limit
		time.Sleep(500 * time.Millisecond)
	}
	return all, nil
}

type oldMapping struct {
	EventName   string
	ReleaseYear int
}

func loadOldMappings(path string) (map[string]oldMapping, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	// event_name → oldMapping（重複はスキップ、最初のものを採用）
	result := make(map[string]oldMapping)
	for i, rec := range records {
		if i == 0 || len(rec) != 3 {
			continue
		}
		year, _ := strconv.Atoi(rec[2])
		name := rec[1]
		if _, exists := result[name]; !exists {
			result[name] = oldMapping{EventName: name, ReleaseYear: year}
		}
	}
	return result, nil
}

func writeEventsCSV(path string, exhibitions []ExhibitionDetail, oldMappings map[string]oldMapping) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// ヘッダー
	w.Write([]string{"bms_search_id", "name", "short_name", "release_year"})

	// BMS Search のイベントを出力
	for _, ex := range exhibitions {
		year := extractYear(ex)
		shortName := ex.Name // デフォルトは正式名称
		w.Write([]string{ex.ID, ex.Name, shortName, strconv.Itoa(year)})
	}

	// 既存マッピングで BMS Search に存在しないもの（bms_search_id なし）を追加
	// この部分は手動マージ用。生成後にユーザーが確認・編集する。
	for name, m := range oldMappings {
		w.Write([]string{"", name, name, strconv.Itoa(m.ReleaseYear)})
		_ = name
	}

	return nil
}

func extractYear(ex ExhibitionDetail) int {
	// terms.entry.startsAt からyearを抽出。なければcreatedAtから。
	if ex.Terms != nil && ex.Terms.Entry != nil && ex.Terms.Entry.StartsAt != "" {
		t, err := time.Parse(time.RFC3339, ex.Terms.Entry.StartsAt)
		if err == nil {
			return t.Year()
		}
	}
	if ex.CreatedAt != "" {
		t, err := time.Parse(time.RFC3339, ex.CreatedAt)
		if err == nil {
			return t.Year()
		}
	}
	return 0
}
```

- [ ] **Step 2: CLIツールを実行してevents.csvを生成**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go run cmd/gen-events/main.go`
Expected: `internal/adapter/persistence/events.csv` が生成される

- [ ] **Step 3: 生成されたCSVを確認・手動編集**

ユーザーがCSVのshort_nameを確認し、既知のイベントの短縮名を手動で設定する。
例: `THE BMS OF FIGHTERS 2015 -Time Travelers-` → short_name を `BOF:TT` に編集。

- [ ] **Step 4: コミット**

```bash
git add cmd/gen-events/main.go internal/adapter/persistence/events.csv
git commit -m "feat: event.csv生成CLIツールを追加"
```

---

### Task 2: ドメインモデルの変更

Event型の追加、SongMeta型の変更、EventMapping型の削除、MetaRepositoryインターフェースの変更。

**Files:**
- Modify: `internal/domain/model/song.go`
- Modify: `internal/domain/model/repository.go`

- [ ] **Step 1: Event型を追加し、SongMeta・EventMappingを変更**

`internal/domain/model/song.go` を以下のように変更:

```go
// Event 型を追加（SongMetaの前に配置）
type Event struct {
	ID          int
	BMSSearchID *string
	Name        string
	ShortName   string
	ReleaseYear int
}

// SongMeta を変更（EventName → EventID、BMSSearchID追加）
type SongMeta struct {
	FolderHash  string
	ReleaseYear *int
	EventID     *int
	BMSSearchID *string // BMS SearchのBMS ID
}

// EventMapping 型を削除（既存の定義を削除）
// SongIRURLs 型も削除（推測フローで使用していたがBMS Search同期では不要）
```

- [ ] **Step 2: MetaRepositoryインターフェースを変更**

`internal/domain/model/repository.go` から event_mapping 関連メソッドを削除し、event 関連メソッドを追加:

```go
// 以下を削除:
// ListEventMappings(ctx context.Context) ([]EventMapping, error)
// UpsertEventMapping(ctx context.Context, m EventMapping) error
// DeleteEventMapping(ctx context.Context, id int) error
// ListUnsetSongsWithIRURLs(ctx context.Context) ([]SongIRURLs, error)

// 以下を追加:
ListEvents(ctx context.Context) ([]Event, error)
GetEventByBMSSearchID(ctx context.Context, bmsSearchID string) (*Event, error)
UpsertEventByBMSSearchID(ctx context.Context, e Event) error
UpdateEventShortName(ctx context.Context, id int, shortName string) error
ListFoldersWithoutEvent(ctx context.Context) ([]string, error)  // event_id未設定のfolder_hash一覧
UpdateSongMetaEvent(ctx context.Context, folderHash string, eventID int, bmsSearchID string) error
```

- [ ] **Step 3: ビルド確認**

この時点ではインターフェースの実装が欠けているためビルドは通らない。次のTaskで実装する。

- [ ] **Step 4: コミット**

```bash
git add internal/domain/model/song.go internal/domain/model/repository.go
git commit -m "refactor: Event型追加、SongMeta変更、EventMapping削除"
```

---

### Task 3: DBマイグレーションの変更

eventテーブル新規作成、song_metaにevent_id・bms_search_id追加・event_name削除、event_mapping削除。events.csvの埋め込み投入。

**Files:**
- Modify: `internal/adapter/persistence/migrations.go`
- Delete: `internal/adapter/persistence/event_mappings.csv`

- [ ] **Step 1: migrations.goを変更**

eventテーブル作成、song_metaのALTER（SQLiteはDROP COLUMNをサポートしないため、テーブル再作成が必要）、event_mapping削除、events.csv投入を追加:

```go
// event テーブル作成
_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS event (
        id             INTEGER PRIMARY KEY AUTOINCREMENT,
        bms_search_id  TEXT UNIQUE,
        name           TEXT NOT NULL,
        short_name     TEXT NOT NULL,
        release_year   INTEGER NOT NULL,
        created_at     TEXT NOT NULL DEFAULT (datetime('now')),
        updated_at     TEXT NOT NULL DEFAULT (datetime('now'))
    )
`)

// song_meta のevent_name → event_id 移行（テーブル再作成）
// SQLiteはALTER TABLE DROP COLUMNをサポートしないため、再作成で対応
_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS song_meta_new (
        id              INTEGER PRIMARY KEY AUTOINCREMENT,
        folder_hash     TEXT NOT NULL UNIQUE,
        release_year    INTEGER,
        event_id        INTEGER REFERENCES event(id),
        bms_search_id   TEXT,
        created_at      TEXT NOT NULL DEFAULT (datetime('now')),
        updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
    )
`)
// 既存データの移行（event_nameは捨てる、release_yearは維持）
_, err = db.Exec(`
    INSERT OR IGNORE INTO song_meta_new (id, folder_hash, release_year, created_at, updated_at)
    SELECT id, folder_hash, release_year, created_at, updated_at
    FROM song_meta
`)
_, err = db.Exec(`DROP TABLE IF EXISTS song_meta`)
_, err = db.Exec(`ALTER TABLE song_meta_new RENAME TO song_meta`)

// event_mapping テーブル削除
_, err = db.Exec(`DROP TABLE IF EXISTS event_mapping`)

// events.csv の投入
// //go:embed events.csv を使用し、event_mappings.csv と同じパターンで読み込み
```

- [ ] **Step 2: events.csvの埋め込みを設定**

```go
//go:embed events.csv
var eventsCSV string
```

既存の `event_mappings.csv` の `//go:embed` と `seedEventMappings` 関数を `events.csv` の `//go:embed` と `seedEvents` 関数に置き換える。

```go
func seedEvents(db *sql.DB) error {
    r := csv.NewReader(strings.NewReader(eventsCSV))
    records, err := r.ReadAll()
    if err != nil {
        return fmt.Errorf("events.csv parse: %w", err)
    }
    for i, rec := range records {
        if i == 0 || len(rec) != 4 {
            continue
        }
        bmsSearchID := rec[0] // 空文字ならNULL
        name := rec[1]
        shortName := rec[2]
        releaseYear, _ := strconv.Atoi(rec[3])

        var bmsSearchIDVal any
        if bmsSearchID != "" {
            bmsSearchIDVal = bmsSearchID
        }

        _, err := db.Exec(
            `INSERT OR IGNORE INTO event (bms_search_id, name, short_name, release_year) VALUES (?, ?, ?, ?)`,
            bmsSearchIDVal, name, shortName, releaseYear,
        )
        if err != nil {
            return fmt.Errorf("event insert: %w", err)
        }
    }
    return nil
}
```

- [ ] **Step 3: event_mappings.csvを削除**

```bash
rm internal/adapter/persistence/event_mappings.csv
```

- [ ] **Step 4: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: まだインターフェース実装が不完全のため、ビルドエラーが出る可能性あり（次Taskで解消）

- [ ] **Step 5: コミット**

```bash
git add internal/adapter/persistence/migrations.go internal/adapter/persistence/events.csv
git rm internal/adapter/persistence/event_mappings.csv
git commit -m "feat: eventテーブル追加、song_meta変更、event_mapping削除のマイグレーション"
```

---

### Task 4: リポジトリ層の変更

event CRUD、song_meta CRUD変更、旧event_mapping CRUD削除。

**Files:**
- Modify: `internal/adapter/persistence/elsa_repository.go`

- [ ] **Step 1: event関連CRUDを追加**

```go
func (r *ElsaRepository) ListEvents(ctx context.Context) ([]model.Event, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, bms_search_id, name, short_name, release_year FROM event ORDER BY release_year DESC, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []model.Event
	for rows.Next() {
		var e model.Event
		if err := rows.Scan(&e.ID, &e.BMSSearchID, &e.Name, &e.ShortName, &e.ReleaseYear); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *ElsaRepository) GetEventByBMSSearchID(ctx context.Context, bmsSearchID string) (*model.Event, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, bms_search_id, name, short_name, release_year FROM event WHERE bms_search_id = ?`,
		bmsSearchID)
	var e model.Event
	if err := row.Scan(&e.ID, &e.BMSSearchID, &e.Name, &e.ShortName, &e.ReleaseYear); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

func (r *ElsaRepository) UpsertEventByBMSSearchID(ctx context.Context, e model.Event) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO event (bms_search_id, name, short_name, release_year)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(bms_search_id) DO UPDATE SET
		   name = excluded.name,
		   updated_at = datetime('now')`,
		e.BMSSearchID, e.Name, e.ShortName, e.ReleaseYear)
	return err
}

func (r *ElsaRepository) UpdateEventShortName(ctx context.Context, id int, shortName string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE event SET short_name = ?, updated_at = datetime('now') WHERE id = ?`,
		shortName, id)
	return err
}
```

- [ ] **Step 2: song_meta関連CRUDを変更**

`GetSongMeta` を変更（event_name → event_id + bms_search_id）:

```go
func (r *ElsaRepository) GetSongMeta(ctx context.Context, folderHash string) (*model.SongMeta, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT folder_hash, release_year, event_id, bms_search_id FROM song_meta WHERE folder_hash = ?`,
		folderHash)
	var meta model.SongMeta
	if err := row.Scan(&meta.FolderHash, &meta.ReleaseYear, &meta.EventID, &meta.BMSSearchID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &meta, nil
}
```

`UpsertSongMeta` を変更:

```go
func (r *ElsaRepository) UpsertSongMeta(ctx context.Context, meta model.SongMeta) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO song_meta (folder_hash, release_year, event_id, bms_search_id)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(folder_hash) DO UPDATE SET
		   release_year  = excluded.release_year,
		   event_id      = excluded.event_id,
		   bms_search_id = excluded.bms_search_id,
		   updated_at    = datetime('now')`,
		meta.FolderHash, meta.ReleaseYear, meta.EventID, meta.BMSSearchID)
	return err
}
```

BMS Search同期用のメソッドを追加:

```go
func (r *ElsaRepository) ListFoldersWithoutEvent(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT s.folder FROM songdata.song s
		 LEFT JOIN song_meta sm ON s.folder = sm.folder_hash
		 WHERE sm.event_id IS NULL OR sm.folder_hash IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var folders []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return nil, err
		}
		folders = append(folders, f)
	}
	return folders, nil
}

func (r *ElsaRepository) UpdateSongMetaEvent(ctx context.Context, folderHash string, eventID int, bmsSearchID string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO song_meta (folder_hash, event_id, bms_search_id)
		 VALUES (?, ?, ?)
		 ON CONFLICT(folder_hash) DO UPDATE SET
		   event_id      = excluded.event_id,
		   bms_search_id = excluded.bms_search_id,
		   updated_at    = datetime('now')`,
		folderHash, eventID, bmsSearchID)
	return err
}
```

- [ ] **Step 3: 旧event_mapping関連CRUDを削除**

以下のメソッドを `elsa_repository.go` から削除:
- `ListEventMappings`
- `UpsertEventMapping`
- `DeleteEventMapping`
- `ListUnsetSongsWithIRURLs`

- [ ] **Step 4: 楽曲一覧クエリの変更**

`SongdataReader` の楽曲一覧取得クエリで、`song_meta.event_name` を `event.short_name` に変更する必要がある。`songdata_reader.go` の `ListSongs` メソッドのJOINを更新:

```sql
LEFT JOIN song_meta sm ON sg.folder_hash = sm.folder_hash
LEFT JOIN event ev ON sm.event_id = ev.id
```

表示カラム:
```sql
ev.short_name AS event_name,
COALESCE(ev.release_year, sm.release_year) AS release_year
```

- [ ] **Step 5: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: まだusecase/handler層で旧コードを参照しているためエラーの可能性あり

- [ ] **Step 6: コミット**

```bash
git add internal/adapter/persistence/elsa_repository.go
git commit -m "feat: eventテーブルCRUD追加、song_metaのevent_id対応、旧event_mapping削除"
```

---

### Task 5: BMS Search APIクライアント

LR2IRClientと同じパターンで、BMS Search APIのHTTPクライアントを実装する。

**Files:**
- Create: `internal/adapter/gateway/bmssearch_client.go`
- Create: `internal/adapter/gateway/bmssearch_client_test.go`

- [ ] **Step 1: BMSSearchClient構造体とメソッドを実装**

```go
// internal/adapter/gateway/bmssearch_client.go
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	defaultBMSSearchBaseURL   = "https://api.bmssearch.net/v1"
	bmsSearchRequestInterval  = 500 * time.Millisecond
)

// BMSSearchPattern はGET /patterns/{md5}のレスポンス
type BMSSearchPattern struct {
	BMS struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"bms"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

// BMSSearchBMS はGET /bmses/{id}のレスポンス
type BMSSearchBMS struct {
	ID          string              `json:"id"`
	Exhibition  *BMSSearchExhibition `json:"exhibition"`
	PublishedAt string              `json:"publishedAt"`
}

// BMSSearchExhibition はBMS内のexhibitionフィールド
type BMSSearchExhibition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type BMSSearchClient struct {
	client   *http.Client
	baseURL  string
	mu       sync.Mutex
	lastReq  time.Time
	interval time.Duration
}

func NewBMSSearchClient() *BMSSearchClient {
	return &BMSSearchClient{
		client:   &http.Client{Timeout: 30 * time.Second},
		baseURL:  defaultBMSSearchBaseURL,
		interval: bmsSearchRequestInterval,
	}
}

func NewBMSSearchClientWithBaseURL(baseURL string) *BMSSearchClient {
	c := NewBMSSearchClient()
	c.baseURL = baseURL
	return c
}

// LookupPatternByMD5 はMD5で譜面を検索する。未登録の場合はnil, nilを返す。
func (c *BMSSearchClient) LookupPatternByMD5(ctx context.Context, md5 string) (*BMSSearchPattern, error) {
	c.rateLimit()
	url := fmt.Sprintf("%s/patterns/%s", c.baseURL, md5)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("BMS Search API error: HTTP %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var pattern BMSSearchPattern
	if err := json.Unmarshal(body, &pattern); err != nil {
		return nil, fmt.Errorf("BMS Search pattern parse: %w", err)
	}
	return &pattern, nil
}

// LookupBMS はBMS IDでBMS詳細を取得する。
func (c *BMSSearchClient) LookupBMS(ctx context.Context, bmsID string) (*BMSSearchBMS, error) {
	c.rateLimit()
	url := fmt.Sprintf("%s/bmses/%s", c.baseURL, bmsID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("BMS Search API error: HTTP %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var bms BMSSearchBMS
	if err := json.Unmarshal(body, &bms); err != nil {
		return nil, fmt.Errorf("BMS Search BMS parse: %w", err)
	}
	return &bms, nil
}

// FetchAllExhibitions は全イベントをページネーションで取得する。
func (c *BMSSearchClient) FetchAllExhibitions(ctx context.Context) ([]BMSSearchExhibitionDetail, error) {
	var all []BMSSearchExhibitionDetail
	offset := 0
	limit := 100

	for {
		c.rateLimit()
		url := fmt.Sprintf("%s/exhibitions/search?offset=%d&limit=%d", c.baseURL, offset, limit)

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("BMS Search exhibitions error: HTTP %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var batch []BMSSearchExhibitionDetail
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, fmt.Errorf("exhibitions parse: %w", err)
		}

		all = append(all, batch...)
		if len(batch) < limit {
			break
		}
		offset += limit
	}
	return all, nil
}

// BMSSearchExhibitionDetail はGET /exhibitions/searchのレスポンス要素
type BMSSearchExhibitionDetail struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Terms *struct {
		Entry *struct {
			StartsAt string `json:"startsAt"`
		} `json:"entry"`
	} `json:"terms"`
	CreatedAt string `json:"createdAt"`
}

func (c *BMSSearchClient) rateLimit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	elapsed := time.Since(c.lastReq)
	if elapsed < c.interval {
		time.Sleep(c.interval - elapsed)
	}
	c.lastReq = time.Now()
}
```

- [ ] **Step 2: テストを作成**

```go
// internal/adapter/gateway/bmssearch_client_test.go
package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLookupPatternByMD5_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/patterns/abc123" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"bms":{"id":"BMS-1","title":"Test Song"},"title":"Test Song [ANOTHER]","artist":"TestArtist"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := NewBMSSearchClientWithBaseURL(srv.URL)
	pattern, err := client.LookupPatternByMD5(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pattern == nil {
		t.Fatal("expected pattern, got nil")
	}
	if pattern.BMS.ID != "BMS-1" {
		t.Errorf("expected BMS ID 'BMS-1', got '%s'", pattern.BMS.ID)
	}
}

func TestLookupPatternByMD5_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer srv.Close()

	client := NewBMSSearchClientWithBaseURL(srv.URL)
	pattern, err := client.LookupPatternByMD5(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pattern != nil {
		t.Fatalf("expected nil, got %+v", pattern)
	}
}

func TestLookupBMS_WithExhibition(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bmses/BMS-1" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id":"BMS-1","exhibition":{"id":"EX-1","name":"Test Event"},"publishedAt":"2024-01-01T00:00:00Z"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := NewBMSSearchClientWithBaseURL(srv.URL)
	bms, err := client.LookupBMS(context.Background(), "BMS-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bms == nil {
		t.Fatal("expected BMS, got nil")
	}
	if bms.Exhibition == nil || bms.Exhibition.ID != "EX-1" {
		t.Error("expected exhibition EX-1")
	}
}
```

- [ ] **Step 3: テスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/adapter/gateway/ -run TestLookup -v`
Expected: PASS

- [ ] **Step 4: コミット**

```bash
git add internal/adapter/gateway/bmssearch_client.go internal/adapter/gateway/bmssearch_client_test.go
git commit -m "feat: BMS Search APIクライアントを追加"
```

---

### Task 6: BMS Search同期ユースケース

フォルダ単位でBMS Search APIを呼び、event_idとbms_search_idをsong_metaに設定するバックグラウンド処理。

**Files:**
- Create: `internal/usecase/sync_bmssearch.go`

- [ ] **Step 1: SyncBMSSearchUseCase を実装**

```go
// internal/usecase/sync_bmssearch.go
package usecase

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

type SyncBMSSearchProgress struct {
	Current int
	Total   int
}

type SyncBMSSearchResult struct {
	Total     int
	Synced    int
	NotFound  int
	Failed    int
	Cancelled bool
}

type SyncBMSSearchUseCase struct {
	bmsClient    *gateway.BMSSearchClient
	metaRepo     model.MetaRepository
	songdataRepo interface {
		ListMD5sByFolder(ctx context.Context, folderHash string) ([]string, error)
	}
}

func NewSyncBMSSearchUseCase(
	bmsClient *gateway.BMSSearchClient,
	metaRepo model.MetaRepository,
	songdataRepo interface {
		ListMD5sByFolder(ctx context.Context, folderHash string) ([]string, error)
	},
) *SyncBMSSearchUseCase {
	return &SyncBMSSearchUseCase{
		bmsClient:    bmsClient,
		metaRepo:     metaRepo,
		songdataRepo: songdataRepo,
	}
}

func (u *SyncBMSSearchUseCase) Execute(
	ctx context.Context,
	folders []string,
	progressFn func(SyncBMSSearchProgress),
) (*SyncBMSSearchResult, error) {
	result := &SyncBMSSearchResult{Total: len(folders)}

	// bms.id → BMSSearchBMS のキャッシュ（同一BMS内の複数フォルダで重複リクエスト防止）
	bmsCache := make(map[string]*gateway.BMSSearchBMS)

	for i, folderHash := range folders {
		select {
		case <-ctx.Done():
			result.Cancelled = true
			return result, nil
		default:
		}

		if progressFn != nil {
			progressFn(SyncBMSSearchProgress{Current: i + 1, Total: len(folders)})
		}

		err := u.syncFolder(ctx, folderHash, bmsCache)
		if err != nil {
			result.Failed++
			continue
		}
		// syncFolderはevent設定有無にかかわらずエラーなしで返る
		// event設定されたかどうかは内部で判定
		result.Synced++
	}

	return result, nil
}

func (u *SyncBMSSearchUseCase) syncFolder(
	ctx context.Context,
	folderHash string,
	bmsCache map[string]*gateway.BMSSearchBMS,
) error {
	// フォルダ内の全MD5を取得
	md5s, err := u.songdataRepo.ListMD5sByFolder(ctx, folderHash)
	if err != nil {
		return err
	}

	// 最初にヒットしたMD5でBMS情報を取得
	for _, md5 := range md5s {
		pattern, err := u.bmsClient.LookupPatternByMD5(ctx, md5)
		if err != nil {
			continue
		}
		if pattern == nil {
			continue
		}

		bmsID := pattern.BMS.ID

		// BMSキャッシュ確認
		bms, cached := bmsCache[bmsID]
		if !cached {
			bms, err = u.bmsClient.LookupBMS(ctx, bmsID)
			if err != nil {
				continue
			}
			bmsCache[bmsID] = bms
		}

		if bms == nil {
			continue
		}

		// exhibition → event_id の解決
		if bms.Exhibition != nil {
			event, err := u.metaRepo.GetEventByBMSSearchID(ctx, bms.Exhibition.ID)
			if err != nil {
				continue
			}
			if event != nil {
				return u.metaRepo.UpdateSongMetaEvent(ctx, folderHash, event.ID, bmsID)
			}
		}

		// exhibitionがなくてもbms_search_idは保存
		return u.metaRepo.UpdateSongMetaEvent(ctx, folderHash, 0, bmsID)
	}

	return nil
}
```

- [ ] **Step 2: コミット**

```bash
git add internal/usecase/sync_bmssearch.go
git commit -m "feat: BMS Search同期ユースケースを追加"
```

---

### Task 7: EventHandler（Wailsバインディング）

イベント管理とBMS Search同期のWailsバインディングを実装する。IR一括取得（ir_handler.go）と同じ非同期パターン。

**Files:**
- Create: `internal/app/event_handler.go`

- [ ] **Step 1: EventHandler を実装**

```go
// internal/app/event_handler.go
package app

import (
	"context"
	"sync"
	"time"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
	"github.com/meta-BE/bms-elsa/internal/app/dto"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type EventHandler struct {
	ctx          context.Context
	metaRepo     model.MetaRepository
	bmsClient    *gateway.BMSSearchClient
	syncUseCase  *usecase.SyncBMSSearchUseCase

	mu         sync.Mutex
	syncing    bool
	cancelFunc context.CancelFunc
	progress   struct{ current, total int }
}

func NewEventHandler(
	metaRepo model.MetaRepository,
	bmsClient *gateway.BMSSearchClient,
	syncUseCase *usecase.SyncBMSSearchUseCase,
) *EventHandler {
	return &EventHandler{
		metaRepo:    metaRepo,
		bmsClient:   bmsClient,
		syncUseCase: syncUseCase,
	}
}

func (h *EventHandler) SetContext(ctx context.Context) { h.ctx = ctx }

// ListEvents はイベント一覧を返す
func (h *EventHandler) ListEvents() ([]dto.EventDTO, error) {
	events, err := h.metaRepo.ListEvents(h.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.EventDTO, len(events))
	for i, e := range events {
		result[i] = dto.EventDTO{
			ID:          e.ID,
			BMSSearchID: e.BMSSearchID,
			Name:        e.Name,
			ShortName:   e.ShortName,
			ReleaseYear: e.ReleaseYear,
		}
	}
	return result, nil
}

// UpdateEventShortName はイベントの短縮名を更新する
func (h *EventHandler) UpdateEventShortName(id int, shortName string) error {
	return h.metaRepo.UpdateEventShortName(h.ctx, id, shortName)
}

// RefreshEventsFromBMSSearch はBMS Searchから新規イベントを取得してeventテーブルに追加する
func (h *EventHandler) RefreshEventsFromBMSSearch() (int, error) {
	exhibitions, err := h.bmsClient.FetchAllExhibitions(h.ctx)
	if err != nil {
		return 0, err
	}

	added := 0
	for _, ex := range exhibitions {
		existing, err := h.metaRepo.GetEventByBMSSearchID(h.ctx, ex.ID)
		if err != nil {
			continue
		}
		if existing != nil {
			// 既存: nameのみ更新
			h.metaRepo.UpsertEventByBMSSearchID(h.ctx, model.Event{
				BMSSearchID: &ex.ID,
				Name:        ex.Name,
				ShortName:   existing.ShortName,
				ReleaseYear: existing.ReleaseYear,
			})
			continue
		}

		// 新規: short_nameはnameのコピー
		year := extractExhibitionYear(ex)
		h.metaRepo.UpsertEventByBMSSearchID(h.ctx, model.Event{
			BMSSearchID: &ex.ID,
			Name:        ex.Name,
			ShortName:   ex.Name,
			ReleaseYear: year,
		})
		added++
	}
	return added, nil
}

// StartBMSSearchSync はBMS Search同期をバックグラウンドで開始する
func (h *EventHandler) StartBMSSearchSync() error {
	h.mu.Lock()
	if h.syncing {
		h.mu.Unlock()
		return nil
	}
	h.syncing = true
	h.mu.Unlock()

	folders, err := h.metaRepo.ListFoldersWithoutEvent(h.ctx)
	if err != nil {
		h.mu.Lock()
		h.syncing = false
		h.mu.Unlock()
		return err
	}

	ctx, cancel := context.WithCancel(h.ctx)
	h.mu.Lock()
	h.cancelFunc = cancel
	h.progress.current = 0
	h.progress.total = len(folders)
	h.mu.Unlock()

	go func() {
		defer func() {
			cancel()
			h.mu.Lock()
			h.syncing = false
			h.cancelFunc = nil
			h.mu.Unlock()
		}()

		result, _ := h.syncUseCase.Execute(ctx, folders, func(p usecase.SyncBMSSearchProgress) {
			h.mu.Lock()
			h.progress.current = p.Current
			h.mu.Unlock()
			wailsRuntime.EventsEmit(h.ctx, "bmssearch:sync-progress", map[string]int{
				"current": p.Current,
				"total":   p.Total,
			})
		})

		doneData := map[string]any{"cancelled": false}
		if result != nil {
			doneData["total"] = result.Total
			doneData["synced"] = result.Synced
			doneData["notFound"] = result.NotFound
			doneData["failed"] = result.Failed
			doneData["cancelled"] = result.Cancelled
		}
		wailsRuntime.EventsEmit(h.ctx, "bmssearch:sync-done", doneData)
	}()

	return nil
}

// StopBMSSearchSync は同期を中断する
func (h *EventHandler) StopBMSSearchSync() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancelFunc != nil {
		h.cancelFunc()
	}
}

// IsSyncing は同期実行中かどうかを返す
func (h *EventHandler) IsSyncing() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.syncing
}

func extractExhibitionYear(ex gateway.BMSSearchExhibitionDetail) int {
	if ex.Terms != nil && ex.Terms.Entry != nil && ex.Terms.Entry.StartsAt != "" {
		t, err := time.Parse(time.RFC3339, ex.Terms.Entry.StartsAt)
		if err == nil {
			return t.Year()
		}
	}
	if ex.CreatedAt != "" {
		t, err := time.Parse(time.RFC3339, ex.CreatedAt)
		if err == nil {
			return t.Year()
		}
	}
	return 0
}
```

- [ ] **Step 2: DTOにEventDTOを追加**

`internal/app/dto/dto.go` に追加:

```go
type EventDTO struct {
	ID          int     `json:"id"`
	BMSSearchID *string `json:"bmsSearchId"`
	Name        string  `json:"name"`
	ShortName   string  `json:"shortName"`
	ReleaseYear int     `json:"releaseYear"`
}
```

- [ ] **Step 3: コミット**

```bash
git add internal/app/event_handler.go internal/app/dto/dto.go
git commit -m "feat: EventHandler（イベント管理・BMS Search同期）を追加"
```

---

### Task 8: 旧コードの削除とDI変更

InferenceHandler、infer_meta、EventMappingManager、InferenceModalを削除し、app.goのDIを変更する。

**Files:**
- Delete: `internal/app/inference_handler.go`
- Delete: `internal/usecase/infer_meta.go`
- Delete: `internal/usecase/infer_meta_test.go`
- Delete: `frontend/src/settings/EventMappingManager.svelte`
- Delete: `frontend/src/settings/InferenceModal.svelte`
- Modify: `internal/app/song_handler.go`
- Modify: `internal/usecase/update_song_meta.go`
- Modify: `app.go`

- [ ] **Step 1: 旧ファイルを削除**

```bash
rm internal/app/inference_handler.go
rm internal/usecase/infer_meta.go
rm internal/usecase/infer_meta_test.go
rm frontend/src/settings/EventMappingManager.svelte
rm frontend/src/settings/InferenceModal.svelte
```

- [ ] **Step 2: SongHandler.UpdateSongMeta のシグネチャを変更**

`internal/app/song_handler.go`:

```go
// 旧:
// func (h *SongHandler) UpdateSongMeta(folderHash string, releaseYear *int, eventName *string) error

// 新: eventNameの代わりにeventIDを受け取る
func (h *SongHandler) UpdateSongMeta(folderHash string, releaseYear *int, eventID *int) error {
	return h.updateMeta.Execute(h.ctx, model.SongMeta{
		FolderHash:  folderHash,
		ReleaseYear: releaseYear,
		EventID:     eventID,
	})
}
```

- [ ] **Step 3: app.goのDIを変更**

`app.go` のApp構造体から `InferenceHandler` を削除し、`EventHandler` を追加:

```go
// App構造体:
// InferenceHandler を削除
EventHandler *internalapp.EventHandler  // 追加

// Init() 内:
// inferMeta 関連を削除
// 以下を追加:
bmsSearchClient := gateway.NewBMSSearchClient()
syncBMSSearch := usecase.NewSyncBMSSearchUseCase(bmsSearchClient, elsaRepo, songdataReader)
a.EventHandler = internalapp.NewEventHandler(elsaRepo, bmsSearchClient, syncBMSSearch)

// startup() 内:
// a.InferenceHandler.SetContext(ctx) を削除
a.EventHandler.SetContext(ctx)  // 追加

// main.go の wails.Run Bind にも EventHandler を追加、InferenceHandler を削除
```

- [ ] **Step 4: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 5: コミット**

```bash
git add -A
git commit -m "refactor: 旧event_mapping関連コード削除、EventHandler追加、DI変更"
```

---

### Task 9: Wailsバインディング再生成

**Files:**
- 自動生成: `frontend/wailsjs/go/app/EventHandler.{js,d.ts}`

- [ ] **Step 1: バインディング再生成**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails generate module`

- [ ] **Step 2: 生成結果確認**

`frontend/wailsjs/go/app/EventHandler.d.ts` に以下が含まれることを確認:
- `ListEvents`
- `UpdateEventShortName`
- `RefreshEventsFromBMSSearch`
- `StartBMSSearchSync`
- `StopBMSSearchSync`
- `IsSyncing`

- [ ] **Step 3: コミット**

```bash
git add -f frontend/wailsjs/go/app/EventHandler.js frontend/wailsjs/go/app/EventHandler.d.ts
git commit -m "chore: Wailsバインディング再生成（EventHandler追加）"
```

---

### Task 10: フロントエンド — EventManager（イベントマスター管理UI）

EventMappingManagerの代替。イベント一覧表示、short_nameインライン編集、BMS Search更新ボタン。

**Files:**
- Create: `frontend/src/settings/EventManager.svelte`

- [ ] **Step 1: EventManager.svelte を作成**

```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { ListEvents, UpdateEventShortName, RefreshEventsFromBMSSearch } from '../../wailsjs/go/app/EventHandler'

  const dispatch = createEventDispatcher()

  let dialog: HTMLDialogElement
  let mouseDownOnBackdrop = false
  let events: any[] = []
  let refreshing = false
  let refreshResult = ''

  export async function open() {
    refreshResult = ''
    await loadEvents()
    dialog.showModal()
  }

  async function loadEvents() {
    try {
      events = (await ListEvents()) || []
    } catch (e) {
      events = []
    }
  }

  async function handleShortNameChange(id: number, value: string) {
    if (!value.trim()) return
    await UpdateEventShortName(id, value.trim())
  }

  async function handleRefreshFromBMSSearch() {
    refreshing = true
    refreshResult = ''
    try {
      const added = await RefreshEventsFromBMSSearch()
      refreshResult = `${added}件の新規イベントを追加しました`
      await loadEvents()
    } catch (e: any) {
      refreshResult = e?.message || '更新に失敗しました'
    } finally {
      refreshing = false
    }
  }

  function handleClose() {
    dialog.close()
    dispatch('close')
  }
</script>

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<dialog bind:this={dialog} class="modal"
  on:mousedown|self={() => mouseDownOnBackdrop = true}
  on:click|self={() => { if (mouseDownOnBackdrop) { dialog.close(); dispatch('close') } mouseDownOnBackdrop = false }}>
  <div class="modal-box max-w-4xl max-h-[80vh]">
    <h3 class="text-lg font-bold mb-4">イベントマスター管理</h3>

    <div class="flex items-center gap-2 mb-4">
      <button class="btn btn-sm btn-outline" on:click={handleRefreshFromBMSSearch} disabled={refreshing}>
        {refreshing ? 'BMS Search取得中...' : 'BMS Searchから更新'}
      </button>
      {#if refreshResult}
        <span class="text-sm text-success">{refreshResult}</span>
      {/if}
    </div>

    {#if events.length > 0}
      <div class="overflow-y-auto max-h-[50vh]">
        <table class="table table-xs">
          <thead class="sticky top-0 bg-base-100">
            <tr>
              <th>正式名称</th>
              <th>短縮名</th>
              <th class="w-16">年</th>
            </tr>
          </thead>
          <tbody>
            {#each events as ev}
              <tr>
                <td class="text-xs">{ev.name}</td>
                <td>
                  <input
                    class="input input-xs input-bordered w-full"
                    value={ev.shortName}
                    on:blur={(e) => handleShortNameChange(ev.id, e.currentTarget.value)}
                  />
                </td>
                <td class="text-xs">{ev.releaseYear}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}
      <p class="text-sm text-base-content/50">イベントが登録されていません</p>
    {/if}

    <div class="modal-action">
      <button class="btn" on:click={handleClose}>閉じる</button>
    </div>
  </div>
</dialog>
```

- [ ] **Step 2: コミット**

```bash
git add frontend/src/settings/EventManager.svelte
git commit -m "feat: EventManager（イベントマスター管理UI）を追加"
```

---

### Task 11: フロントエンド — SongDetail（オートコンプリート付きイベント選択）

event_nameの自由テキスト入力をeventマスターからのオートコンプリート付きドロップダウンに変更する。

**Files:**
- Modify: `frontend/src/views/SongDetail.svelte`

- [ ] **Step 1: importとイベント一覧の取得を追加**

```typescript
import { ListEvents } from '../../wailsjs/go/app/EventHandler'

let allEvents: any[] = []
let eventSuggestions: any[] = []
let eventSearchText = ''
let showEventDropdown = false

onMount(async () => {
  allEvents = (await ListEvents()) || []
})
```

- [ ] **Step 2: イベント選択UIを変更**

既存のevent_name入力:
```svelte
<input id="event-input" class="input input-xs input-bordered w-32" bind:value={editEventName} on:blur={saveMeta} />
```

変更後（オートコンプリート付き）:
```svelte
<div class="relative">
  <input
    class="input input-xs input-bordered w-40"
    value={eventSearchText}
    on:input={(e) => {
      eventSearchText = e.currentTarget.value
      eventSuggestions = allEvents.filter(ev =>
        ev.shortName.toLowerCase().includes(eventSearchText.toLowerCase()) ||
        ev.name.toLowerCase().includes(eventSearchText.toLowerCase())
      ).slice(0, 10)
      showEventDropdown = eventSuggestions.length > 0
    }}
    on:focus={() => {
      if (eventSearchText) {
        eventSuggestions = allEvents.filter(ev =>
          ev.shortName.toLowerCase().includes(eventSearchText.toLowerCase())
        ).slice(0, 10)
        showEventDropdown = eventSuggestions.length > 0
      }
    }}
    on:blur={() => setTimeout(() => showEventDropdown = false, 200)}
    placeholder="イベント検索..."
  />
  {#if showEventDropdown}
    <ul class="absolute z-50 bg-base-100 border border-base-300 rounded shadow-lg mt-1 max-h-40 overflow-y-auto w-64">
      {#each eventSuggestions as ev}
        <li>
          <button
            class="w-full text-left px-2 py-1 text-xs hover:bg-base-200"
            on:mousedown|preventDefault={() => {
              selectEvent(ev.id)
              eventSearchText = ev.shortName
              showEventDropdown = false
            }}
          >
            <span class="font-semibold">{ev.shortName}</span>
            {#if ev.shortName !== ev.name}
              <span class="text-base-content/50 ml-1">({ev.name})</span>
            {/if}
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>
```

- [ ] **Step 3: saveMeta を変更**

```typescript
async function selectEvent(eventID: number) {
  if (!detail) return
  await UpdateSongMeta(detail.folderHash, editReleaseYear ? parseInt(editReleaseYear) : null, eventID)
  await loadDetail(detail.folderHash)
}
```

- [ ] **Step 4: release_yearの表示ロジックを変更**

event_id設定時はevent.release_yearを読み取り専用で表示:

```svelte
{#if detail.eventId}
  <span class="text-xs">{detail.releaseYear}</span>
{:else}
  <input id="year-input" class="input input-xs input-bordered w-16" type="number"
         bind:value={editReleaseYear} on:blur={saveMeta} />
{/if}
```

- [ ] **Step 5: コミット**

```bash
git add frontend/src/views/SongDetail.svelte
git commit -m "feat: SongDetailのイベント選択をオートコンプリート付きドロップダウンに変更"
```

---

### Task 12: フロントエンド — SongTable表示変更とApp.svelte統合

SongTableの表示ロジック変更、App.svelteでのEventHandler統合、InferenceHandler参照削除。

**Files:**
- Modify: `frontend/src/views/SongTable.svelte`
- Modify: `frontend/src/App.svelte`

- [ ] **Step 1: SongTable.svelte の表示ロジック確認**

SongTableはバックエンドからDTOを受け取って表示するだけなので、バックエンドの`ListSongs`クエリが正しくevent.short_nameとCOALESCE(e.release_year, sm.release_year)を返していれば、フロントエンド側の変更は最小限。

DTOの `eventName` と `releaseYear` フィールドが引き続き使われるため、バックエンドのクエリ変更（Task 4）で対応済み。

- [ ] **Step 2: App.svelte の変更**

- InferenceHandler関連のimportを削除
- InferenceModal関連のコンポーネントを削除
- EventMappingManager → EventManager に変更
- 「メタ推測」ボタンを「BMS Search同期」ボタンに変更（または削除してEventManager内に統合）

具体的な変更:
```svelte
<!-- 旧 -->
<script>
  import EventMappingManager from './settings/EventMappingManager.svelte'
  import InferenceModal from './settings/InferenceModal.svelte'
  import { RunAutoInference } from '../wailsjs/go/app/InferenceHandler'
</script>

<!-- 新 -->
<script>
  import EventManager from './settings/EventManager.svelte'
  import { StartBMSSearchSync, StopBMSSearchSync } from '../wailsjs/go/app/EventHandler'
</script>
```

- [ ] **Step 3: BMS Search同期ボタンの追加**

楽曲一覧のヘッダーに「BMS Search同期」ボタンを追加（IR取得ボタンと同様のパターン）。進捗表示はWails Events `bmssearch:sync-progress` / `bmssearch:sync-done` をリッスン。

- [ ] **Step 4: コミット**

```bash
git add frontend/src/views/SongTable.svelte frontend/src/App.svelte
git commit -m "feat: SongTable表示変更、App.svelteにEventHandler統合"
```

---

### Task 13: 最終確認

- [ ] **Step 1: go build 確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 2: テスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./...`
Expected: PASS（旧infer_meta_testは削除済み）

- [ ] **Step 3: wails dev で統合動作確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails dev`

確認事項:
1. 楽曲一覧でEvent/Yearカラムが正しく表示される
2. 楽曲詳細でオートコンプリート付きイベント選択が動作する
3. 設定画面のイベントマスター管理でshort_nameが編集できる
4. 「BMS Searchから更新」ボタンで新規イベントが取得される
5. BMS Search同期が進捗表示付きで動作する
6. 同期後に楽曲のイベント情報が正しく表示される
