# 楽曲メタデータ推測機能 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** LR2IR本体URLのパターンマッチで楽曲のReleaseYear/EventNameを半自動設定する機能を追加する

**Architecture:** elsa.dbに`event_mapping`テーブルを追加し、未設定曲のIR URLとマッチング。マッチした曲は自動保存、マッチしない曲はUIで手動確認。新規の`InferenceHandler`をWailsにバインドしてフロントエンドから呼び出す。

**Tech Stack:** Go 1.24 / SQLite (modernc.org/sqlite) / Svelte 4 / Wails v2 / TanStack Table / DaisyUI

---

### Task 1: event_mappingテーブルのマイグレーション追加

**Files:**
- Modify: `internal/adapter/persistence/migrations.go`

マイグレーションのstatementsスライスに追加:

```sql
CREATE TABLE IF NOT EXISTS event_mapping (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    url_pattern  TEXT NOT NULL UNIQUE,
    event_name   TEXT NOT NULL,
    release_year INTEGER NOT NULL
)
```

**確認:** `go build ./...` が通ること

---

### Task 2: ドメインモデルとリポジトリインターフェース

**Files:**
- Modify: `internal/domain/model/song.go` — `EventMapping`構造体を追加
- Modify: `internal/domain/model/repository.go` — `MetaRepository`にevent_mapping CRUD + 推測用クエリを追加

**EventMapping構造体:**

```go
type EventMapping struct {
    ID          int
    URLPattern  string
    EventName   string
    ReleaseYear int
}
```

**MetaRepositoryに追加するメソッド:**

```go
// event_mapping CRUD
ListEventMappings(ctx context.Context) ([]EventMapping, error)
UpsertEventMapping(ctx context.Context, m EventMapping) error
DeleteEventMapping(ctx context.Context, id int) error

// 推測用: 未設定曲のfolderHash + 紐づくIR本体URLを取得
// song_metaにレコードがない or (release_year IS NULL AND event_name IS NULL)の曲が対象
// chart_metaのlr2ir_body_urlをJOINで取得
ListUnsetSongsWithIRURLs(ctx context.Context) ([]SongIRURLs, error)
```

**SongIRURLs構造体:**

```go
type SongIRURLs struct {
    FolderHash string
    Title      string
    Artist     string
    Genre      string
    BodyURLs   []string // この曲の全譜面のlr2ir_body_url（空文字列除く）
    ChartCount int      // 総譜面数
    IRCount    int      // IR取得済み譜面数
}
```

**確認:** `go build ./...` が通ること

---

### Task 3: リポジトリ実装（event_mapping CRUD）

**Files:**
- Modify: `internal/adapter/persistence/elsa_repository.go`

**ListEventMappings:**

```go
func (r *ElsaRepository) ListEventMappings(ctx context.Context) ([]model.EventMapping, error) {
    rows, err := r.db.QueryContext(ctx,
        `SELECT id, url_pattern, event_name, release_year FROM event_mapping ORDER BY event_name`)
    // ...scan loop...
}
```

**UpsertEventMapping:**

```go
func (r *ElsaRepository) UpsertEventMapping(ctx context.Context, m model.EventMapping) error {
    _, err := r.db.ExecContext(ctx,
        `INSERT INTO event_mapping (url_pattern, event_name, release_year)
         VALUES (?, ?, ?)
         ON CONFLICT(url_pattern) DO UPDATE SET
           event_name = excluded.event_name,
           release_year = excluded.release_year`,
        m.URLPattern, m.EventName, m.ReleaseYear)
    return err
}
```

**DeleteEventMapping:**

```go
func (r *ElsaRepository) DeleteEventMapping(ctx context.Context, id int) error {
    _, err := r.db.ExecContext(ctx, `DELETE FROM event_mapping WHERE id = ?`, id)
    return err
}
```

**確認:** `go build ./...` が通ること

---

### Task 4: リポジトリ実装（ListUnsetSongsWithIRURLs）

**Files:**
- Modify: `internal/adapter/persistence/elsa_repository.go`

songdata.dbとelsa.dbのクロスDB JOIN。songdata.dbは`sd`スキーマでATTACH済み。

```go
func (r *ElsaRepository) ListUnsetSongsWithIRURLs(ctx context.Context) ([]model.SongIRURLs, error) {
    // 1. 未設定のfolderHashを特定（song_metaにレコードがない or 両方NULL）
    // 2. songdata.dbからタイトル等を取得
    // 3. chart_metaからbody_urlを取得
    //
    // sd.song → sd.folder でfolderHashを取得
    // LEFT JOIN song_meta で未設定判定
    // LEFT JOIN chart_meta で body_url取得
}
```

クエリの構造（CTEで段階的に構築）:

```sql
WITH song_groups AS (
    SELECT
        f.path AS folder_hash,
        MIN(s.title) AS title,
        MIN(s.artist) AS artist,
        MIN(s.genre) AS genre,
        COUNT(*) AS chart_count
    FROM sd.song s
    JOIN sd.folder f ON s.path = f.path
    GROUP BY f.path
),
ir_urls AS (
    SELECT
        f.path AS folder_hash,
        cm.lr2ir_body_url,
        CASE WHEN cm.lr2ir_fetched_at IS NOT NULL THEN 1 ELSE 0 END AS has_ir
    FROM sd.song s
    JOIN sd.folder f ON s.path = f.path
    LEFT JOIN chart_meta cm ON s.md5 = cm.md5
)
SELECT
    sg.folder_hash, sg.title, sg.artist, sg.genre, sg.chart_count,
    GROUP_CONCAT(DISTINCT iu.lr2ir_body_url) AS body_urls,
    SUM(iu.has_ir) AS ir_count
FROM song_groups sg
LEFT JOIN song_meta sm ON sg.folder_hash = sm.folder_hash
LEFT JOIN ir_urls iu ON sg.folder_hash = iu.folder_hash
WHERE sm.folder_hash IS NULL
   OR (sm.release_year IS NULL AND sm.event_name IS NULL)
GROUP BY sg.folder_hash
ORDER BY sg.title
```

`body_urls`は`GROUP_CONCAT`の結果をGoで`strings.Split`。空文字列やNULLを除外。

**確認:** `go build ./...` + `go test ./internal/usecase/...` が通ること

---

### Task 5: テスト（mockにメソッド追加 + 推測ロジックテスト）

**Files:**
- Modify: `internal/usecase/usecase_test.go` — mockMetaRepoに新メソッド追加
- 新規: `internal/usecase/infer_meta_test.go` — 推測ロジックのテスト

**mockMetaRepoに追加:**

```go
listEventMappingsFunc        func(ctx context.Context) ([]model.EventMapping, error)
upsertEventMappingFunc       func(ctx context.Context, m model.EventMapping) error
deleteEventMappingFunc       func(ctx context.Context, id int) error
listUnsetSongsWithIRURLsFunc func(ctx context.Context) ([]model.SongIRURLs, error)
```

**テストケース（infer_meta_test.go）:**

1. `TestRunAutoInference_AllMatch` — 全曲のURLがマッピングにマッチ → 全件自動設定
2. `TestRunAutoInference_PartialMatch` — 一部マッチ、一部未マッチ → マッチ分のみ保存、未マッチリスト返却
3. `TestRunAutoInference_NoIRURLs` — IR URL未取得の曲 → 未マッチとして返却（IR未取得数もカウント）

**確認:** `go test ./internal/usecase/... -v`

---

### Task 6: UseCase（InferSongMetaUseCase）

**Files:**
- 新規: `internal/usecase/infer_meta.go`

```go
type InferSongMetaUseCase struct {
    metaRepo model.MetaRepository
}

type InferenceResult struct {
    AutoSetCount   int
    UnmatchedSongs []model.SongIRURLs
    NoIRCount      int  // IR未取得の曲数（UnmatchedSongsの内数）
}

func (u *InferSongMetaUseCase) RunAutoInference(ctx context.Context) (*InferenceResult, error) {
    // 1. マッピングテーブル取得
    mappings, _ := u.metaRepo.ListEventMappings(ctx)

    // 2. 未設定曲＋IR URL取得
    songs, _ := u.metaRepo.ListUnsetSongsWithIRURLs(ctx)

    // 3. マッチング
    var autoSet int
    var unmatched []model.SongIRURLs
    for _, song := range songs {
        if matched := matchURL(song.BodyURLs, mappings); matched != nil {
            // 一括保存
            year := matched.ReleaseYear
            name := matched.EventName
            u.metaRepo.UpsertSongMeta(ctx, model.SongMeta{
                FolderHash: song.FolderHash, ReleaseYear: &year, EventName: &name,
            })
            autoSet++
        } else {
            unmatched = append(unmatched, song)
        }
    }

    // NoIRCount集計
    noIR := 0
    for _, s := range unmatched {
        if s.IRCount == 0 { noIR++ }
    }

    return &InferenceResult{AutoSetCount: autoSet, UnmatchedSongs: unmatched, NoIRCount: noIR}, nil
}

// matchURL: URLリストの中にマッピングのpatternを含むものがあるか
func matchURL(urls []string, mappings []model.EventMapping) *model.EventMapping {
    for _, url := range urls {
        for i, m := range mappings {
            if strings.Contains(url, m.URLPattern) {
                return &mappings[i]
            }
        }
    }
    return nil
}
```

**確認:** `go test ./internal/usecase/... -v` — Task 5のテストがパスすること

---

### Task 7: Handler + DI配線

**Files:**
- 新規: `internal/app/inference_handler.go`
- Modify: `app.go` — InferenceHandler作成 + Bind追加
- Modify: `main.go` — Bind配列にInferenceHandler追加

**InferenceHandler:**

```go
type InferenceHandler struct {
    ctx      context.Context
    inferMeta *usecase.InferSongMetaUseCase
    metaRepo  model.MetaRepository  // マッピングCRUD用に直接参照
}

// Wailsバインディング用メソッド
func (h *InferenceHandler) RunAutoInference() (*InferenceResultDTO, error)
func (h *InferenceHandler) ListEventMappings() ([]EventMappingDTO, error)
func (h *InferenceHandler) UpsertEventMapping(urlPattern, eventName string, releaseYear int) error
func (h *InferenceHandler) DeleteEventMapping(id int) error
```

**DTO:**（`internal/app/dto/dto.go`に追加）

```go
type InferenceResultDTO struct {
    AutoSetCount   int           `json:"autoSetCount"`
    UnmatchedSongs []SongIRURLsDTO `json:"unmatchedSongs"`
    NoIRCount      int           `json:"noIRCount"`
}

type SongIRURLsDTO struct {
    FolderHash string   `json:"folderHash"`
    Title      string   `json:"title"`
    Artist     string   `json:"artist"`
    Genre      string   `json:"genre"`
    BodyURLs   []string `json:"bodyUrls"`
    ChartCount int      `json:"chartCount"`
    IRCount    int      `json:"irCount"`
}

type EventMappingDTO struct {
    ID          int    `json:"id"`
    URLPattern  string `json:"urlPattern"`
    EventName   string `json:"eventName"`
    ReleaseYear int    `json:"releaseYear"`
}
```

**app.goのDI:**

```go
inferMeta := usecase.NewInferSongMetaUseCase(elsaRepo)
a.InferenceHandler = internalapp.NewInferenceHandler(inferMeta, elsaRepo)
```

**main.goのBind:**

```go
Bind: []interface{}{
    app,
    app.SongHandler,
    app.IRHandler,
    app.InferenceHandler,
},
```

**確認:** `go build ./...` が通ること + `wails build`でバインディング生成確認

---

### Task 8: フロントエンド — マッピング管理UI

**Files:**
- 新規: `frontend/src/EventMappingManager.svelte`
- Modify: `frontend/src/App.svelte` — マッピング管理の導線追加

マッピング管理は設定画面（Settings）の一部またはモーダルとして実装。

**EventMappingManager.svelte:**
- `ListEventMappings()` で一覧取得・表示（テーブル形式）
- 各行に編集・削除ボタン
- 追加フォーム: url_pattern / event_name / release_year の3フィールド
- `UpsertEventMapping()` / `DeleteEventMapping()` で保存・削除

**確認:** `wails dev` で画面表示・CRUD操作を確認

---

### Task 9: フロントエンド — 推測フローUI

**Files:**
- 新規: `frontend/src/InferenceModal.svelte`
- Modify: `frontend/src/SongTable.svelte` — ヘッダーに「メタ推測」ボタン追加

**InferenceModal.svelte:**

フェーズ1（自動）:
- 「メタ推測」ボタン押下 → `RunAutoInference()` 呼び出し
- ローディング表示
- 結果サマリー:「X曲を自動設定 / Y曲が未マッチ（うちIR未取得Z曲）」
- 「手動確認を開始」ボタン（未マッチ曲がある場合）

フェーズ2（手動）:
- 未マッチ曲を1曲ずつ表示（タイトル、アーティスト、ジャンル、IR URL一覧、IR取得状況）
- event_name / release_year の入力欄
- 「保存して次へ」→ `UpdateSongMeta()` 呼び出し → 次の曲へ
- 「スキップ」→ 保存せず次の曲へ
- 「終了」→ モーダルを閉じる
- 進捗表示:「3 / 45」

**SongTable.svelteの変更:**
- ヘッダーバーの検索窓の隣に「メタ推測」ボタン追加
- クリックでInferenceModalを表示

**確認:** `wails dev` でフルフロー動作確認

---

### Task 10: 初期マッピングデータの投入

**方針:** 主要なBMSイベントのURL→イベント名・年のマッピングを調査してevent_mappingテーブルの初期データとして用意する。

- LLMで主要イベント（BOF, BOFU, BMS OF FIGHTERS等）のmanbow.nothing.sh URLパターンを収集
- Go initスクリプトまたはマイグレーションで初期データINSERT
- `INSERT OR IGNORE` で冪等に

**確認:** アプリ起動後にマッピング管理画面でデータが表示されること
