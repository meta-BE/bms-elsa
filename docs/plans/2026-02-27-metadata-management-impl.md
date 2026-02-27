# メタデータ管理 MVP 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** songdata.dbの読み取り + elsa.dbでの追加メタデータ管理 + LR2IRスクレイピング + 楽曲一覧UIを実装する

**Architecture:** SQLite ATTACH DATABASEでsongdata.dbを直接参照し、elsa.dbに追加メタデータのみ保存。クリーンアーキテクチャ（domain → usecase → port ← adapter → app）。Wails v2でフロントエンドにバインド。

**Tech Stack:** Go 1.23, Wails v2, modernc.org/sqlite, Svelte 4, TanStack Table/Virtual, DaisyUI 5

**設計ドキュメント:** `docs/plans/2026-02-27-metadata-management-design.md`

**参考ドキュメント:**
- `docs/beatoraja-songdata-schema.md` — songdata.dbスキーマ
- `docs/lr2ir-structure.md` — LR2IRレスポンス構造
- `testdata/songdata.db` — テスト用実データ（15,437レコード）

---

## Task 1: 依存追加 + ドメインモデル

**Files:**
- Modify: `go.mod`
- Create: `internal/domain/model/song.go`
- Create: `internal/domain/model/repository.go`
- Create: `internal/port/ir_client.go`

**Step 1: SQLite依存を追加**

```bash
cd /path/to/bms-elsa
go get modernc.org/sqlite
```

golang.org/x/textは既にindirect依存にあるが、Shift_JIS変換で直接使うため明示的に追加:

```bash
go get golang.org/x/text
```

net/htmlはGo標準ライブラリにないため追加:

```bash
go get golang.org/x/net
```

**Step 2: ドメインモデルを作成**

`internal/domain/model/song.go`:

```go
package model

import "time"

// Song は楽曲（フォルダ単位のグルーピング）
type Song struct {
	FolderHash string
	Title      string // 代表譜面から取得
	Artist     string
	Genre      string
	MinBPM     float64
	MaxBPM     float64
	Charts     []Chart
	// elsa.db メタデータ
	ReleaseYear *int
	EventName   *string
}

// Chart は譜面（個々のBMSファイル）
type Chart struct {
	MD5        string
	SHA256     string
	Title      string
	Artist     string
	SubArtist  string
	Genre      string
	Mode       int
	Difficulty int
	Level      int
	MinBPM     float64
	MaxBPM     float64
	Path       string
	// elsa.db メタデータ
	IRMeta *ChartIRMeta
}

// SongMeta は楽曲レベルの追加メタデータ
type SongMeta struct {
	FolderHash  string
	ReleaseYear *int
	EventName   *string
}

// ChartIRMeta はLR2IR + 動作URLメタデータ
type ChartIRMeta struct {
	MD5            string
	SHA256         string
	Tags           []string
	LR2IRBodyURL   string
	LR2IRDiffURL   string
	LR2IRNotes     string
	WorkingBodyURL string
	WorkingDiffURL string
	FetchedAt      *time.Time
}
```

**Step 3: リポジトリインターフェースを作成**

`internal/domain/model/repository.go`:

```go
package model

import "context"

// ListOptions は楽曲一覧取得のオプション
type ListOptions struct {
	Page     int
	PageSize int
	SortBy   string
	SortDesc bool
	Search   string // title, artist, genreを横断検索
}

// DuplicateGroup は同一md5の譜面グループ
type DuplicateGroup struct {
	MD5    string
	Charts []Chart
}

// SongRepository はsongdata.dbから楽曲・譜面を読み取る（読み取り専用）
type SongRepository interface {
	// ListSongs は楽曲一覧を返す。songdata.db + elsa.dbのJOIN結果。
	// 戻り値: 楽曲スライス, 総数, エラー
	ListSongs(ctx context.Context, opts ListOptions) ([]Song, int, error)
	// GetSongByFolder はフォルダハッシュから楽曲詳細（全譜面含む）を返す
	GetSongByFolder(ctx context.Context, folderHash string) (*Song, error)
}

// MetaRepository はelsa.dbのメタデータCRUD
type MetaRepository interface {
	GetSongMeta(ctx context.Context, folderHash string) (*SongMeta, error)
	UpsertSongMeta(ctx context.Context, meta SongMeta) error
	GetChartMeta(ctx context.Context, md5, sha256 string) (*ChartIRMeta, error)
	UpsertChartMeta(ctx context.Context, meta ChartIRMeta) error
	BulkUpsertChartMeta(ctx context.Context, metas []ChartIRMeta) error
}
```

**Step 4: IRClient ポートを作成**

`internal/port/ir_client.go`:

```go
package port

import "context"

// IRResponse はLR2IRの情報セクションのパース結果
type IRResponse struct {
	Registered bool
	Genre      string
	Title      string
	Artist     string
	BPM        string
	Level      string
	Keys       string
	JudgeRank  string
	Tags       []string
	BodyURL    string
	DiffURL    string
	Notes      string
}

// IRClient はLR2IRへのアクセスインターフェース
type IRClient interface {
	LookupByMD5(ctx context.Context, md5 string) (*IRResponse, error)
}
```

**Step 5: ビルド確認**

```bash
go build ./...
```

**Step 6: コミット**

```bash
git add go.mod go.sum internal/domain/model/song.go internal/domain/model/repository.go internal/port/ir_client.go
git commit -m "feat: ドメインモデルとリポジトリインターフェースを追加"
```

---

## Task 2: elsa.db マイグレーション

**Files:**
- Create: `internal/adapter/persistence/migrations.go`
- Create: `internal/adapter/persistence/migrations_test.go`

**Step 1: マイグレーションのテストを作成**

`internal/adapter/persistence/migrations_test.go`:

```go
package persistence_test

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
)

func TestRunMigrations(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := persistence.RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// song_metaテーブルが存在することを確認
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM song_meta").Scan(&count)
	if err != nil {
		t.Fatalf("song_meta table not found: %v", err)
	}

	// chart_metaテーブルが存在することを確認
	err = db.QueryRow("SELECT COUNT(*) FROM chart_meta").Scan(&count)
	if err != nil {
		t.Fatalf("chart_meta table not found: %v", err)
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// 2回実行してもエラーにならない
	if err := persistence.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	if err := persistence.RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations should be idempotent: %v", err)
	}
}
```

**Step 2: テストが失敗することを確認**

```bash
go test ./internal/adapter/persistence/ -v
```

Expected: コンパイルエラー（persistence.RunMigrationsが未定義）

**Step 3: マイグレーション実装**

`internal/adapter/persistence/migrations.go`:

```go
package persistence

import "database/sql"

// RunMigrations はelsa.dbのスキーマを作成する。冪等。
func RunMigrations(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS song_meta (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			folder_hash   TEXT NOT NULL UNIQUE,
			release_year  INTEGER,
			event_name    TEXT,
			created_at    TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS chart_meta (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			md5              TEXT NOT NULL,
			sha256           TEXT NOT NULL,
			lr2ir_tags       TEXT,
			lr2ir_body_url   TEXT,
			lr2ir_diff_url   TEXT,
			lr2ir_notes      TEXT,
			lr2ir_fetched_at TEXT,
			working_body_url TEXT,
			working_diff_url TEXT,
			created_at       TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at       TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(md5, sha256)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_song_meta_folder_hash ON song_meta(folder_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_chart_meta_md5 ON chart_meta(md5)`,
		`CREATE INDEX IF NOT EXISTS idx_chart_meta_sha256 ON chart_meta(sha256)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
```

**Step 4: テストがパスすることを確認**

```bash
go test ./internal/adapter/persistence/ -v
```

**Step 5: コミット**

```bash
git add internal/adapter/persistence/migrations.go internal/adapter/persistence/migrations_test.go
git commit -m "feat: elsa.dbマイグレーション実装"
```

---

## Task 3: MetaRepository（elsa.db CRUD）

**Files:**
- Create: `internal/adapter/persistence/elsa_repository.go`
- Create: `internal/adapter/persistence/elsa_repository_test.go`

**Step 1: テストを作成**

`internal/adapter/persistence/elsa_repository_test.go`:

SongMeta の Upsert/Get、ChartMeta の Upsert/Get/BulkUpsert をテスト。
各テストはインメモリSQLiteを使い、RunMigrationsでスキーマ作成後にCRUD操作を検証。

テストケース:
- `TestUpsertAndGetSongMeta` — 挿入 → 取得 → 更新 → 取得を検証
- `TestGetSongMeta_NotFound` — 存在しないfolder_hashでnilが返ること
- `TestUpsertAndGetChartMeta` — 挿入 → 取得。Tags []stringのシリアライズ/デシリアライズ
- `TestBulkUpsertChartMeta` — 複数レコード一括挿入 → 各レコード取得

**Step 2: テストが失敗することを確認**

```bash
go test ./internal/adapter/persistence/ -run TestUpsert -v
```

**Step 3: ElsaRepository実装**

`internal/adapter/persistence/elsa_repository.go`:

```go
package persistence

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

type ElsaRepository struct {
	db *sql.DB
}

func NewElsaRepository(db *sql.DB) *ElsaRepository {
	return &ElsaRepository{db: db}
}
```

メソッド:
- `GetSongMeta`: `SELECT ... FROM song_meta WHERE folder_hash = ?`
- `UpsertSongMeta`: `INSERT ... ON CONFLICT(folder_hash) DO UPDATE SET ...`
- `GetChartMeta`: `SELECT ... FROM chart_meta WHERE md5 = ? AND sha256 = ?`、タグはカンマ区切りからstrings.Splitで[]stringへ
- `UpsertChartMeta`: `INSERT ... ON CONFLICT(md5, sha256) DO UPDATE SET ...`、タグはstrings.Joinでカンマ区切りへ
- `BulkUpsertChartMeta`: トランザクション内で各レコードをUpsert

**Step 4: テストがパスすることを確認**

```bash
go test ./internal/adapter/persistence/ -v
```

**Step 5: コミット**

```bash
git add internal/adapter/persistence/elsa_repository.go internal/adapter/persistence/elsa_repository_test.go
git commit -m "feat: MetaRepository実装（elsa.db CRUD）"
```

---

## Task 4: SongdataReader（songdata.db ATTACH + 読み取り）

**Files:**
- Create: `internal/adapter/persistence/songdata_reader.go`
- Create: `internal/adapter/persistence/songdata_reader_test.go`

**前提:** `testdata/songdata.db` が存在すること（15,437レコード）

**Step 1: テストを作成**

`internal/adapter/persistence/songdata_reader_test.go`:

testdata/songdata.dbをATTACHしたインメモリelsa.dbで動作確認。

テストケース:
- `TestListSongs_Default` — デフォルト条件で楽曲一覧取得。返却数 > 0、TotalCount > 0を確認
- `TestListSongs_Paging` — Page=1, PageSize=10 で10件以下が返ること。Page=2でoffsetが効くこと
- `TestListSongs_Search` — 既知のタイトル（testdata/songdata.dbに存在する曲名）で検索してヒットすること
- `TestGetSongByFolder` — ListSongsで取得したfolderHashを使い、詳細取得。Charts配列がlen > 0

テストのセットアップ:
1. `sql.Open("sqlite", ":memory:")` でelsa.dbを開く
2. `RunMigrations(db)` でスキーマ作成
3. `ATTACH DATABASE 'testdata/songdata.db' AS songdata` を実行
4. SongdataReader + ElsaRepositoryを生成

注意: テストファイルからtestdata/songdata.dbへのパスは `../../../testdata/songdata.db` になる。
`os.Getwd()` やテストヘルパーで解決する。

**Step 2: テストが失敗することを確認**

```bash
go test ./internal/adapter/persistence/ -run TestListSongs -v
```

**Step 3: SongdataReader実装**

`internal/adapter/persistence/songdata_reader.go`:

```go
package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

type SongdataReader struct {
	db *sql.DB // elsa.db接続（songdata.dbをATTACH済み）
}

func NewSongdataReader(db *sql.DB) *SongdataReader {
	return &SongdataReader{db: db}
}

// AttachSongdata はsongdata.dbをATTACHする
func AttachSongdata(db *sql.DB, songdataPath string) error {
	_, err := db.Exec(fmt.Sprintf("ATTACH DATABASE '%s' AS songdata", songdataPath))
	return err
}
```

`ListSongs` の実装方針:
- songdata.songテーブルをfolder単位でGROUP BY
- 代表譜面（MIN(rowid)等）からtitle, artist, genre, bpm を取得
- LEFT JOINでsong_meta, chart_metaを結合
- ソート・ページング・検索をSQLで処理
- COUNT(*) OVER() で総件数を取得

```sql
-- 楽曲一覧の基本クエリ（folder単位のグルーピング）
WITH song_groups AS (
    SELECT
        s.folder,
        MIN(s.title) AS title,
        MIN(s.artist) AS artist,
        MIN(s.genre) AS genre,
        MIN(s.minbpm) AS min_bpm,
        MAX(s.maxbpm) AS max_bpm,
        COUNT(*) AS chart_count
    FROM songdata.song s
    GROUP BY s.folder
)
SELECT
    sg.folder, sg.title, sg.artist, sg.genre,
    sg.min_bpm, sg.max_bpm, sg.chart_count,
    sm.release_year, sm.event_name,
    EXISTS(SELECT 1 FROM chart_meta cm
           JOIN songdata.song s2 ON s2.md5 = cm.md5 AND s2.sha256 = cm.sha256
           WHERE s2.folder = sg.folder) AS has_ir_meta,
    COUNT(*) OVER() AS total_count
FROM song_groups sg
LEFT JOIN song_meta sm ON sm.folder_hash = sg.folder
WHERE (? = '' OR sg.title LIKE '%' || ? || '%'
       OR sg.artist LIKE '%' || ? || '%'
       OR sg.genre LIKE '%' || ? || '%')
ORDER BY sg.title
LIMIT ? OFFSET ?
```

`GetSongByFolder`:
- folder一致の全song行を取得 → Chart配列に変換
- 各ChartにLEFT JOINでchart_metaを結合
- song_metaも取得してSongに統合

**Step 4: テストがパスすることを確認**

```bash
go test ./internal/adapter/persistence/ -v
```

**Step 5: コミット**

```bash
git add internal/adapter/persistence/songdata_reader.go internal/adapter/persistence/songdata_reader_test.go
git commit -m "feat: SongdataReader実装（songdata.db ATTACH + 読み取り）"
```

---

## Task 5: LR2IRパーサー

**Files:**
- Create: `internal/adapter/gateway/lr2ir_parser.go`
- Create: `internal/adapter/gateway/lr2ir_parser_test.go`
- Create: `internal/adapter/gateway/testdata/registered_full.html`
- Create: `internal/adapter/gateway/testdata/registered_minimal.html`
- Create: `internal/adapter/gateway/testdata/unregistered.html`

HTMLパースロジックをHTTP通信から分離し、テスタブルにする。

**Step 1: テストフィクスチャを作成**

`docs/lr2ir-structure.md` の検証結果を元に、3パターンのHTMLフィクスチャを作成。
実際のLR2IRレスポンスをcurlで取得し、UTF-8変換した状態で保存する。

パターン:
- `registered_full.html` — パターン3相当（タグ、本体URL、差分URL、備考すべてあり）
- `registered_minimal.html` — パターン1相当（URL空、タグ空、備考行なし）
- `unregistered.html` — パターン4相当（「この曲は登録されていません。」）

HTMLフィクスチャの取得:

```bash
curl -s 'http://www.dream-pro.info/~lavalse/LR2IR/search.cgi?mode=ranking&bmsmd5=c7fb88a21280b2d0de8f477036f43225' | iconv -f SHIFT_JIS -t UTF-8 > internal/adapter/gateway/testdata/registered_full.html
curl -s 'http://www.dream-pro.info/~lavalse/LR2IR/search.cgi?mode=ranking&bmsmd5=d91af3b677cd97d8dbed7ab3e3bae244' | iconv -f SHIFT_JIS -t UTF-8 > internal/adapter/gateway/testdata/registered_minimal.html
```

未登録フィクスチャは存在しないmd5で取得:

```bash
curl -s 'http://www.dream-pro.info/~lavalse/LR2IR/search.cgi?mode=ranking&bmsmd5=0000000000000000000000000000000f' | iconv -f SHIFT_JIS -t UTF-8 > internal/adapter/gateway/testdata/unregistered.html
```

**Step 2: パーサーテストを作成**

`internal/adapter/gateway/lr2ir_parser_test.go`:

テストケース:
- `TestParseLR2IR_Full` — registered_full.htmlをパース。Genre="DEMENTIA PROVECTO", Title="Anima Mundi -Incipiens Finis- [EX]", Tags=["Stella","st2"], BodyURL/DiffURL/Notes がそれぞれ期待値と一致
- `TestParseLR2IR_Minimal` — registered_minimal.htmlをパース。Registered=true, Tags空, BodyURL空, DiffURL空, Notes空
- `TestParseLR2IR_Unregistered` — unregistered.htmlをパース。Registered=false

**Step 3: テストが失敗することを確認**

```bash
go test ./internal/adapter/gateway/ -v
```

**Step 4: パーサー実装**

`internal/adapter/gateway/lr2ir_parser.go`:

```go
package gateway

import (
	"strings"

	"golang.org/x/net/html"

	"github.com/meta-BE/bms-elsa/internal/port"
)

// ParseLR2IRResponse はLR2IRのHTMLレスポンス（UTF-8変換済み）をパースする
func ParseLR2IRResponse(body string) (*port.IRResponse, error) {
	// ...
}
```

パースロジック（docs/lr2ir-structure.mdに基づく）:
1. `この曲は登録されていません。` を含むか → Registered=false で即リターン
2. golang.org/x/net/htmlのTokenizerで走査
3. `<h4>` → Genre, `<h1>` → Title, `<h2>` → Artist
4. `<h3>` の中身が "情報" → 以降の `<table>` をパース
5. 情報テーブルの各 `<tr>` を処理:
   - 1行目: BPM, レベル, 鍵盤数, 判定ランク（th/tdペア4組）
   - 2行目: タグ（`<a>`タグのテキスト、keyword=が空でないもの）
   - 3行目: 本体URL（`<a>`タグのhref）
   - 4行目: 差分URL（`<a>`タグのhref）
   - 5行目（任意）: 備考（tdのテキスト）
6. `&amp;` をHTMLエンティティデコード

**Step 5: テストがパスすることを確認**

```bash
go test ./internal/adapter/gateway/ -v
```

**Step 6: コミット**

```bash
git add internal/adapter/gateway/lr2ir_parser.go internal/adapter/gateway/lr2ir_parser_test.go internal/adapter/gateway/testdata/
git commit -m "feat: LR2IR HTMLパーサー実装"
```

---

## Task 6: LR2IR HTTPクライアント

**Files:**
- Create: `internal/adapter/gateway/lr2ir_client.go`
- Create: `internal/adapter/gateway/lr2ir_client_test.go`

**Step 1: テストを作成**

`internal/adapter/gateway/lr2ir_client_test.go`:

net/http/httptestでモックサーバーを立て、Shift_JISレスポンスを返す。

テストケース:
- `TestLR2IRClient_Registered` — モックサーバーがShift_JISのHTMLを返す → UTF-8変換 + パースが正常動作
- `TestLR2IRClient_RateLimit` — 連続呼び出しで1秒以上のインターバルが入ること
- `TestLR2IRClient_ContextCancel` — context.Cancelでリクエストがキャンセルされること

**Step 2: テストが失敗することを確認**

```bash
go test ./internal/adapter/gateway/ -run TestLR2IRClient -v
```

**Step 3: LR2IRClient実装**

`internal/adapter/gateway/lr2ir_client.go`:

```go
package gateway

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"

	"github.com/meta-BE/bms-elsa/internal/port"
)

const (
	lr2irBaseURL    = "http://www.dream-pro.info/~lavalse/LR2IR/search.cgi"
	minRequestInterval = time.Second
)

type LR2IRClient struct {
	client   *http.Client
	baseURL  string
	mu       sync.Mutex
	lastReq  time.Time
}

func NewLR2IRClient() *LR2IRClient {
	return &LR2IRClient{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: lr2irBaseURL,
	}
}

// テスト用にbaseURLを差し替え可能にする
func NewLR2IRClientWithBaseURL(baseURL string) *LR2IRClient {
	return &LR2IRClient{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: baseURL,
	}
}
```

LookupByMD5:
1. レートリミット: 前回リクエストから1秒未満なら待つ
2. `GET {baseURL}?mode=ranking&bmsmd5={md5}`
3. レスポンスボディをShift_JIS→UTF-8変換（`japanese.ShiftJIS.NewDecoder()`）
4. `ParseLR2IRResponse()` でパース
5. 結果を返却

**Step 4: テストがパスすることを確認**

```bash
go test ./internal/adapter/gateway/ -v
```

**Step 5: コミット**

```bash
git add internal/adapter/gateway/lr2ir_client.go internal/adapter/gateway/lr2ir_client_test.go
git commit -m "feat: LR2IR HTTPクライアント実装"
```

---

## Task 7: ユースケース

**Files:**
- Create: `internal/usecase/list_songs.go`
- Create: `internal/usecase/get_song_detail.go`
- Create: `internal/usecase/update_song_meta.go`
- Create: `internal/usecase/update_chart_meta.go`
- Create: `internal/usecase/lookup_ir.go`
- Create: `internal/usecase/usecase_test.go`

ユースケース層はリポジトリ/ポートへの委譲が主。テストではモックを使用。

**Step 1: テストを作成**

`internal/usecase/usecase_test.go`:

モック:
- `mockSongRepo` — SongRepository実装
- `mockMetaRepo` — MetaRepository実装
- `mockIRClient` — IRClient実装

テストケース:
- `TestListSongs` — SongRepositoryのListSongsに委譲していることを確認
- `TestGetSongDetail` — SongRepositoryのGetSongByFolderに委譲
- `TestUpdateSongMeta` — MetaRepositoryのUpsertSongMetaに委譲
- `TestLookupIR` — IRClient.LookupByMD5 → MetaRepository.UpsertChartMeta の順に呼ばれること
- `TestLookupIR_Unregistered` — Registered=falseの場合、UpsertChartMetaは呼ばない

**Step 2: テストが失敗することを確認**

```bash
go test ./internal/usecase/ -v
```

**Step 3: ユースケース実装**

各ファイル:

`list_songs.go`:
```go
type ListSongsUseCase struct {
	songRepo model.SongRepository
}

func (u *ListSongsUseCase) Execute(ctx context.Context, opts model.ListOptions) ([]model.Song, int, error) {
	return u.songRepo.ListSongs(ctx, opts)
}
```

`get_song_detail.go`:
```go
type GetSongDetailUseCase struct {
	songRepo model.SongRepository
}

func (u *GetSongDetailUseCase) Execute(ctx context.Context, folderHash string) (*model.Song, error) {
	return u.songRepo.GetSongByFolder(ctx, folderHash)
}
```

`update_song_meta.go`:
```go
type UpdateSongMetaUseCase struct {
	metaRepo model.MetaRepository
}

func (u *UpdateSongMetaUseCase) Execute(ctx context.Context, meta model.SongMeta) error {
	return u.metaRepo.UpsertSongMeta(ctx, meta)
}
```

`update_chart_meta.go`:
```go
type UpdateChartMetaUseCase struct {
	metaRepo model.MetaRepository
}

func (u *UpdateChartMetaUseCase) Execute(ctx context.Context, meta model.ChartIRMeta) error {
	return u.metaRepo.UpsertChartMeta(ctx, meta)
}
```

`lookup_ir.go`:
```go
type LookupIRUseCase struct {
	irClient port.IRClient
	metaRepo model.MetaRepository
}

func (u *LookupIRUseCase) Execute(ctx context.Context, md5, sha256 string) (*port.IRResponse, error) {
	resp, err := u.irClient.LookupByMD5(ctx, md5)
	if err != nil {
		return nil, err
	}
	if !resp.Registered {
		return resp, nil
	}
	// chart_metaに保存
	meta := model.ChartIRMeta{
		MD5:          md5,
		SHA256:       sha256,
		Tags:         resp.Tags,
		LR2IRBodyURL: resp.BodyURL,
		LR2IRDiffURL: resp.DiffURL,
		LR2IRNotes:   resp.Notes,
		FetchedAt:    timeNowPtr(),
	}
	if err := u.metaRepo.UpsertChartMeta(ctx, meta); err != nil {
		return nil, err
	}
	return resp, nil
}
```

**Step 4: テストがパスすることを確認**

```bash
go test ./internal/usecase/ -v
```

**Step 5: コミット**

```bash
git add internal/usecase/
git commit -m "feat: ユースケース実装"
```

---

## Task 8: DTO + Wailsハンドラー

**Files:**
- Create: `internal/app/dto/dto.go`
- Create: `internal/app/song_handler.go`
- Create: `internal/app/ir_handler.go`

**Step 1: DTO作成**

`internal/app/dto/dto.go`:

設計ドキュメントのDTO定義をそのまま実装。
Song → SongRowDTO, Song → SongDetailDTO, Chart → ChartDTO の変換関数も含む。

**Step 2: SongHandler作成**

`internal/app/song_handler.go`:

```go
package app

type SongHandler struct {
	listSongs     *usecase.ListSongsUseCase
	getSongDetail *usecase.GetSongDetailUseCase
	updateMeta    *usecase.UpdateSongMetaUseCase
}
```

メソッド（Wailsにバインドされる）:
- `ListSongs(page, pageSize int, sortBy string, sortDesc bool, search string) (*dto.SongListDTO, error)`
- `GetSongDetail(folderHash string) (*dto.SongDetailDTO, error)`
- `UpdateSongMeta(folderHash string, releaseYear *int, eventName *string) error`

**Step 3: IRHandler作成**

`internal/app/ir_handler.go`:

```go
package app

type IRHandler struct {
	lookupIR    *usecase.LookupIRUseCase
	updateChart *usecase.UpdateChartMetaUseCase
}
```

メソッド:
- `LookupByMD5(md5, sha256 string) (*dto.ChartMetaDTO, error)` — LR2IR取得 + 保存
- `UpdateChartMeta(md5, sha256 string, workingBodyURL, workingDiffURL string) error` — 動作URL手動更新

**Step 4: ビルド確認**

```bash
go build ./...
```

**Step 5: コミット**

```bash
git add internal/app/
git commit -m "feat: DTO + Wailsハンドラー実装"
```

---

## Task 9: DI組み立て（main.go + app.go）

**Files:**
- Modify: `main.go`
- Modify: `app.go`

**Step 1: app.goを書き換え**

仮実装のApp構造体を、実際のハンドラーを保持する構造体に変更。

```go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
	"github.com/meta-BE/bms-elsa/internal/app"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

type App struct {
	ctx         context.Context
	db          *sql.DB
	songHandler *app.SongHandler
	irHandler   *app.IRHandler
}
```

startup内でDB接続・マイグレーション・DI組み立てを実行。
初期実装ではsongdata.dbのパスはデフォルト位置（beatorajaのデータディレクトリ）をチェックし、
見つからない場合はWailsのダイアログでファイル選択。

**Step 2: main.goを更新**

Bindに SongHandler と IRHandler を追加:

```go
Bind: []interface{}{
	app,
	app.songHandler,
	app.irHandler,
},
```

**Step 3: ビルド確認**

```bash
go build ./...
```

**Step 4: コミット**

```bash
git add main.go app.go
git commit -m "feat: DI組み立てとWailsバインディング"
```

---

## Task 10: フロントエンド — 型定義とSongTable接続

**Files:**
- Modify: `frontend/src/dummy.ts` → `frontend/src/types.ts` にリネーム（ダミーデータ削除、型定義のみ残す）
- Modify: `frontend/src/SongTable.svelte`
- Modify: `frontend/src/App.svelte`

**前提:** Task 9完了後、`wails dev` または `wails generate module` でフロントエンドバインディングが
`frontend/wailsjs/` に自動生成される。

**Step 1: Wailsバインディング生成**

```bash
cd /path/to/bms-elsa
wails generate module
```

これにより `frontend/wailsjs/go/app/SongHandler.js` 等が生成される。

**Step 2: 型定義ファイルを作成**

`frontend/src/types.ts`:

```typescript
// Goの DTO に対応するTypeScript型
// Wails生成バインディングの型を補完

export type SongRow = {
  folderHash: string
  title: string
  artist: string
  genre: string
  minBpm: number
  maxBpm: number
  eventName: string | null
  releaseYear: number | null
  hasIrMeta: boolean
  chartCount: number
}

export type SongList = {
  songs: SongRow[]
  totalCount: number
  page: number
  pageSize: number
}

export type SongDetail = {
  folderHash: string
  title: string
  artist: string
  genre: string
  eventName: string | null
  releaseYear: number | null
  charts: ChartInfo[]
}

export type ChartInfo = {
  md5: string
  sha256: string
  title: string
  mode: number
  difficulty: number
  level: number
  minBpm: number
  maxBpm: number
  hasIrMeta: boolean
  lr2irTags?: string
  lr2irBodyUrl?: string
  lr2irDiffUrl?: string
  lr2irNotes?: string
  workingBodyUrl?: string
  workingDiffUrl?: string
}
```

**Step 3: SongTable.svelteを実データ接続に更新**

変更内容:
- ダミーデータインポートを削除
- Wails生成のSongHandler.ListSongsを呼び出し
- columns定義を新しいカラム構成に変更（Title, Artist, Genre, BPM, Event, Year, IR, Charts）
- onMount で初回データ取得

**Step 4: App.svelteを更新**

- Scanボタンの代わりにsongdata.dbパス表示（将来的に設定画面へ）

**Step 5: ビルド確認**

```bash
cd frontend && npm run check
```

**Step 6: コミット**

```bash
git add frontend/src/types.ts frontend/src/SongTable.svelte frontend/src/App.svelte
git rm frontend/src/dummy.ts
git commit -m "feat: フロントエンドを実データ接続に更新"
```

---

## Task 11: フロントエンド — 詳細パネル

**Files:**
- Create: `frontend/src/SongDetail.svelte`
- Modify: `frontend/src/SongTable.svelte`（行クリックイベント追加）
- Modify: `frontend/src/App.svelte`（詳細パネル配置）

**Step 1: SongDetail.svelteを作成**

設計ドキュメントの詳細パネルワイヤーフレームに基づいて実装:
- 楽曲メタデータ（タイトル、アーティスト、ジャンル）表示
- Event / Year の編集フィールド（inline edit）
- 譜面一覧テーブル（Mode, Difficulty, Level, md5）
- LR2IR取得ボタン（各譜面ごと）
- LR2IR情報表示（選択中の譜面）
- 動作URLの編集フィールド

Wailsバインディング経由で:
- `SongHandler.GetSongDetail(folderHash)` で詳細取得
- `SongHandler.UpdateSongMeta(folderHash, year, event)` で楽曲メタ更新
- `IRHandler.LookupByMD5(md5, sha256)` でLR2IR取得
- `IRHandler.UpdateChartMeta(md5, sha256, workingBody, workingDiff)` で動作URL更新

**Step 2: SongTableに行クリックイベントを追加**

クリックされた行のfolderHashをdispatchイベントで親に通知。

**Step 3: App.svelteにレイアウト配置**

テーブルと詳細パネルを横並び（またはテーブル下部にパネル表示）。

```
┌─────────────────────────────────────┐
│ BMS ELSA                            │
├──────────────────────┬──────────────┤
│ SongTable            │ SongDetail   │
│ (flex: 1)            │ (w-96)       │
└──────────────────────┴──────────────┘
```

**Step 4: ビルド確認**

```bash
cd frontend && npm run check
```

**Step 5: コミット**

```bash
git add frontend/src/SongDetail.svelte frontend/src/SongTable.svelte frontend/src/App.svelte
git commit -m "feat: 楽曲詳細パネル実装"
```

---

## Task 12: 統合テスト + 動作確認

**Files:**
- Create: `internal/integration_test.go`（オプション）

**Step 1: 全テスト実行**

```bash
go test ./... -v
```

**Step 2: Wailsアプリ起動確認**

```bash
wails dev
```

確認項目:
- 楽曲一覧が表示される（songdata.dbから読み込み）
- ソート・検索が動作する
- 行クリックで詳細パネルが表示される
- メタデータ（Event, Year）の編集が保存される
- LR2IR取得ボタンでデータが取得・表示される

**Step 3: コミット**

```bash
git add -A
git commit -m "feat: メタデータ管理MVP統合完了"
```

---

## タスク依存関係

```
Task 1 (モデル+依存)
  ├── Task 2 (マイグレーション)
  │     └── Task 3 (MetaRepository)
  │           └── Task 4 (SongdataReader) ← Task 2, 3に依存
  │
  ├── Task 5 (LR2IRパーサー)
  │     └── Task 6 (LR2IR HTTPクライアント)
  │
  └── Task 7 (ユースケース) ← Task 3, 4, 6に依存
        └── Task 8 (DTO+ハンドラー)
              └── Task 9 (DI組み立て)
                    └── Task 10 (フロントエンド一覧)
                          └── Task 11 (フロントエンド詳細)
                                └── Task 12 (統合テスト)
```

Task 2-3 と Task 5-6 は並列実行可能。
