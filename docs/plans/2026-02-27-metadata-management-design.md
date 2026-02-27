# メタデータ管理 MVP 設計

## 概要

bms-elsaの最初の機能として、楽曲メタデータ管理を実装する。
beatorajaのsongdata.dbを読み取り専用で直接参照し、独自DB（elsa.db）に追加メタデータのみを保存する。

### スコープ

**MVP（本設計の対象）**:
- songdata.db読み取り（ATTACH DATABASE）
- elsa.dbによる追加メタデータ管理（楽曲レベル・譜面レベル）
- LR2IRスクレイピングによるメタデータ取得
- メタデータの自動推定 + 手動修正
- 楽曲一覧表示（songdata.db + elsa.db結合）

**将来拡張（本設計の対象外）**:
- フォルダ移動（都度指定で移動先選択。移動後のbeatoraja再スキャンはユーザー手動）
- 楽曲・差分の導入（URL提示 + フォルダ取り込み機能を分離）
- URL書き換えルール（ドメイン置換）
- イベントページパース

## アーキテクチャ

### データソース構成

```
songdata.db (beatoraja管理、読み取り専用)
  └─ ATTACH DATABASE で接続
       │
       ├─ song テーブル ─── 楽曲・譜面の基本情報
       └─ folder テーブル ─ フォルダ階層情報

elsa.db (bms-elsa独自、読み書き)
  ├─ song_meta ──── 楽曲レベルの追加メタデータ
  └─ chart_meta ─── 譜面レベルの追加メタデータ + LR2IR情報
```

songdata.dbのデータはコピーしない。SQLiteのATTACH DATABASE機能でクロスDB JOINを行い、
elsa.dbには追加メタデータのみを保存する。

### 既存architecture.mdとの差異

既存のarchitecture.mdはbms-elsaが独自にBMSファイルをスキャンし自前DBに全データを格納する設計。
本MVP設計ではsongdata.dbを直接参照するため、以下が変更となる。

| 既存設計 | MVP方針 |
|----------|---------|
| songs, chartsテーブルにデータ複製 | songdata.dbをATTACHで直接参照 |
| ScanSongs（フォルダ走査+BMSパース） | MVP不要（beatorajaがスキャン済み） |
| BMSParser, Hasher ポート | MVP不要（将来拡張ポイントとして記録） |
| ir_cacheテーブル | chart_metaに統合（動作URLも保持） |

## データモデル

### elsa.db スキーマ

```sql
CREATE TABLE song_meta (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    folder_hash   TEXT NOT NULL UNIQUE,  -- songdata.song.folder と同値
    release_year  INTEGER,               -- 公開年
    event_name    TEXT,                   -- イベント名
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE chart_meta (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    md5              TEXT NOT NULL,
    sha256           TEXT NOT NULL,
    -- LR2IR生データ
    lr2ir_tags       TEXT,               -- カンマ区切り
    lr2ir_body_url   TEXT,               -- LR2IR記載の本体URL
    lr2ir_diff_url   TEXT,               -- LR2IR記載の差分URL
    lr2ir_notes      TEXT,               -- 備考
    lr2ir_fetched_at TEXT,               -- LR2IR最終取得日時
    -- 動作URL（ルール適用後 or 手動修正後）
    working_body_url TEXT,
    working_diff_url TEXT,
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at       TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(md5, sha256)
);

CREATE INDEX idx_song_meta_folder_hash ON song_meta(folder_hash);
CREATE INDEX idx_chart_meta_md5 ON chart_meta(md5);
CREATE INDEX idx_chart_meta_sha256 ON chart_meta(sha256);
```

### songdata.db参照カラム（ATTACH経由で使用）

| カラム | 用途 |
|--------|------|
| md5 | chart_metaとの結合キー、LR2IR検索キー |
| sha256 | chart_metaとの結合キー（一意性保証） |
| folder | song_metaとの結合キー（楽曲グルーピング） |
| title | 表示用 |
| artist, subartist | 表示用 |
| genre | 表示用 |
| mode | 詳細パネル表示用 |
| difficulty | 詳細パネル表示用 |
| level | 詳細パネル表示用 |
| minbpm, maxbpm | BPM表示 |
| path | 将来の移動・導入で使用 |
| adddate | 公開年の推定ソースとして使用 |

### 紐づけキー

- 楽曲レベル: `songdata.song.folder` = `elsa.song_meta.folder_hash`
- 譜面レベル: `songdata.song.md5` + `songdata.song.sha256` = `elsa.chart_meta.md5` + `elsa.chart_meta.sha256`

md5はLR2IRとの連携に使用、sha256は一意性保証に使用。

### クエリ例

```sql
ATTACH DATABASE 'songdata.db' AS songdata;

-- 楽曲一覧（メタデータ付き）
SELECT
    s.title, s.artist, s.genre, s.minbpm, s.maxbpm,
    sm.release_year, sm.event_name,
    cm.lr2ir_tags IS NOT NULL AS has_ir_meta,
    COUNT(*) OVER (PARTITION BY s.folder) AS chart_count
FROM songdata.song s
LEFT JOIN main.song_meta sm ON sm.folder_hash = s.folder
LEFT JOIN main.chart_meta cm ON cm.md5 = s.md5 AND cm.sha256 = s.sha256
ORDER BY s.title;
```

### Goドメインモデル

```go
// 楽曲（フォルダ単位のグルーピング）
type Song struct {
    FolderHash  string
    Title       string      // 代表譜面から取得
    Artist      string
    Genre       string
    Charts      []Chart
    // elsa.db メタデータ
    ReleaseYear *int
    EventName   *string
}

// 譜面（個々のBMSファイル）
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
    IRMeta     *ChartIRMeta
}

// LR2IR + 動作URLメタデータ
type ChartIRMeta struct {
    Tags           []string
    LR2IRBodyURL   string
    LR2IRDiffURL   string
    LR2IRNotes     string
    WorkingBodyURL string
    WorkingDiffURL string
    FetchedAt      *time.Time
}
```

## レイヤー構成

### ディレクトリ構造（MVP）

```
bms-elsa/
├── internal/
│   ├── domain/
│   │   └── model/
│   │       ├── song.go          # Song, Chart, ChartIRMeta
│   │       └── repository.go    # SongRepository, MetaRepository
│   │
│   ├── usecase/
│   │   ├── list_songs.go        # 楽曲一覧（songdata.db + elsa.db結合）
│   │   ├── get_song_detail.go   # 楽曲詳細（全譜面 + メタデータ）
│   │   ├── update_song_meta.go  # 楽曲メタデータ更新
│   │   ├── update_chart_meta.go # 譜面メタデータ更新
│   │   └── lookup_ir.go         # LR2IRメタデータ取得・保存
│   │
│   ├── port/
│   │   └── ir_client.go         # IRClient interface
│   │
│   ├── adapter/
│   │   ├── gateway/
│   │   │   └── lr2ir_client.go  # LR2IRスクレイピング実装
│   │   └── persistence/
│   │       ├── songdata_reader.go  # songdata.db ATTACH + 読み取り
│   │       ├── elsa_repository.go  # elsa.db CRUD
│   │       └── migrations.go       # elsa.dbスキーマ作成
│   │
│   └── app/
│       ├── song_handler.go      # Wailsバインディング
│       ├── ir_handler.go        # LR2IRバインディング
│       └── dto/
│           └── dto.go           # フロントエンド向けDTO
│
├── frontend/
│   └── src/
│       ├── SongTable.svelte     # 楽曲一覧テーブル（既存を拡張）
│       └── SongDetail.svelte    # 楽曲詳細パネル（新規）
```

### リポジトリインターフェース

```go
// songdata.dbから楽曲・譜面を読み取る（読み取り専用）
type SongRepository interface {
    ListSongs(ctx context.Context, opts ListOptions) ([]Song, int, error)
    GetSongByFolder(ctx context.Context, folderHash string) (*Song, error)
    FindDuplicates(ctx context.Context) ([]DuplicateGroup, error)
}

// elsa.dbのメタデータCRUD
type MetaRepository interface {
    GetSongMeta(ctx context.Context, folderHash string) (*SongMeta, error)
    UpsertSongMeta(ctx context.Context, meta SongMeta) error
    GetChartMeta(ctx context.Context, md5, sha256 string) (*ChartIRMeta, error)
    UpsertChartMeta(ctx context.Context, meta ChartIRMeta) error
    BulkUpsertChartMeta(ctx context.Context, metas []ChartIRMeta) error
}
```

### IRClient ポート

```go
type IRClient interface {
    LookupByMD5(ctx context.Context, md5 string) (*IRResponse, error)
}

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
```

### 主要ユースケースのフロー

**楽曲一覧取得**:
```
Frontend → SongHandler.ListSongs(page, sort, filter)
  → ListSongsUseCase.Execute()
    → SongRepository.ListSongs()   // songdata.db + elsa.db JOIN
  → DTO変換
← SongListDTO
```

**LR2IRメタデータ取得**:
```
Frontend → IRHandler.LookupByMD5(md5)
  → LookupIRUseCase.Execute(md5)
    → IRClient.LookupByMD5(md5)           // HTTPスクレイピング
    → MetaRepository.UpsertChartMeta()     // elsa.dbに保存
  → DTO変換
← ChartMetaDTO
```

**メタデータ更新**:
```
Frontend → SongHandler.UpdateMeta(folderHash, releaseYear, eventName)
  → UpdateSongMetaUseCase.Execute()
    → MetaRepository.UpsertSongMeta()
← 成功/失敗
```

## LR2IRスクレイピング

docs/lr2ir-structure.mdで検証済みの知見に基づく。

### アクセス方法

```
エンドポイント: http://www.dream-pro.info/~lavalse/LR2IR/search.cgi?mode=ranking&bmsmd5=<MD5>
プロトコル: HTTPのみ（HTTPS不可）
文字コード: Shift_JIS → UTF-8変換必要
```

### パース手順

1. `この曲は登録されていません。` の有無で未登録判定
2. `<h4>`, `<h1>`, `<h2>` からジャンル・タイトル・アーティスト
3. `<h3>情報</h3>` 直後の `<table>` から情報テーブルをパース
4. 各`<tr>`のth/tdペアからBPM, レベル, 鍵盤数, 判定ランク, タグ, 本体URL, 差分URL, 備考を抽出

### レートリミット対策

- リクエスト間に最低1秒のインターバル
- 一括取得時はバッチ処理 + キャンセル可能
- 取得済み（lr2ir_fetched_atがある）はスキップ可能

## メタデータ自動推定

### イベント名の推定ソース（優先順）

1. LR2IRのタグ（Stella, Satellite, BOF2023等）
2. LR2IRの本体URL（例: manbowのイベントページURL → BOFイベント）
3. 手動入力

### 公開年の推定ソース（優先順）

1. LR2IRのタグからの推定（イベント名に年が含まれる場合）
2. LR2IRの本体URLからの推定
3. songdata.dbのadddate（beatorajaへの追加日 ≒ 入手時期の近似）
4. 手動入力

**注意**: songdata.dbのpathやフォルダ名はユーザーのフォルダ構造に依存するため、推定ソースとして使用しない。

### 推定フロー

```
ユーザーがLR2IR取得を実行
  → LR2IRからデータ取得
  → chart_meta に保存
  → タグやURLから event_name, release_year を推定
  → 推定値をUIにプリフィルとして表示
  → ユーザーが確認・修正して確定
```

## フロントエンド設計

### 楽曲一覧テーブル

既存のSongTable.svelteをダミーデータから実データに接続。

**表示カラム**:

| カラム | ソース | ソート | フィルタ |
|--------|--------|--------|----------|
| Title | songdata.song.title | Yes | テキスト検索 |
| Artist | songdata.song.artist | Yes | テキスト検索 |
| Genre | songdata.song.genre | Yes | テキスト検索 |
| BPM | songdata.song.minbpm/maxbpm | Yes | - |
| Event | elsa.song_meta.event_name | Yes | テキスト検索 |
| Year | elsa.song_meta.release_year | Yes | 範囲 |
| IR | chart_meta有無（アイコン） | - | 有/無 |
| Charts | 譜面数 | Yes | - |

Mode, Difficulty, Levelは一覧に表示しない（実用可能な情報ではないため）。

**操作**:
- 行クリック → 詳細パネル表示
- LR2IR取得ボタン（個別 or 選択一括）
- メタデータ編集（インライン or 詳細パネル内）

### 詳細パネル

楽曲を選択したときに表示。譜面一覧ではMode/Difficulty/Levelを表示する。

```
┌─────────────────────────────────┐
│ [タイトル]          ✏ Event: BOF2023
│ [アーティスト]      ✏ Year: 2023
│ Genre: ELECTRO
├─────────────────────────────────┤
│ 譜面一覧:
│  SP NORMAL  ☆5   md5: abc...  [IR取得]
│  SP HYPER   ☆8   md5: def...  [IR取得] done
│  SP ANOTHER ☆11  md5: ghi...  [IR取得] done
├─────────────────────────────────┤
│ LR2IR情報 (SP ANOTHER):
│  タグ: Satellite
│  本体URL: https://...
│  差分URL: https://...
│  備考: ...
│  動作URL: https://... ✏
└─────────────────────────────────┘
```

### DTO

```go
type SongListDTO struct {
    Songs      []SongRowDTO `json:"songs"`
    TotalCount int          `json:"totalCount"`
    Page       int          `json:"page"`
    PageSize   int          `json:"pageSize"`
}

type SongRowDTO struct {
    FolderHash  string  `json:"folderHash"`
    Title       string  `json:"title"`
    Artist      string  `json:"artist"`
    Genre       string  `json:"genre"`
    MinBPM      float64 `json:"minBpm"`
    MaxBPM      float64 `json:"maxBpm"`
    EventName   *string `json:"eventName"`
    ReleaseYear *int    `json:"releaseYear"`
    HasIRMeta   bool    `json:"hasIrMeta"`
    ChartCount  int     `json:"chartCount"`
}

type SongDetailDTO struct {
    FolderHash  string     `json:"folderHash"`
    Title       string     `json:"title"`
    Artist      string     `json:"artist"`
    Genre       string     `json:"genre"`
    EventName   *string    `json:"eventName"`
    ReleaseYear *int       `json:"releaseYear"`
    Charts      []ChartDTO `json:"charts"`
}

type ChartDTO struct {
    MD5            string  `json:"md5"`
    SHA256         string  `json:"sha256"`
    Title          string  `json:"title"`
    Mode           int     `json:"mode"`
    Difficulty     int     `json:"difficulty"`
    Level          int     `json:"level"`
    MinBPM         float64 `json:"minBpm"`
    MaxBPM         float64 `json:"maxBpm"`
    HasIRMeta      bool    `json:"hasIrMeta"`
    LR2IRTags      string  `json:"lr2irTags,omitempty"`
    LR2IRBodyURL   string  `json:"lr2irBodyUrl,omitempty"`
    LR2IRDiffURL   string  `json:"lr2irDiffUrl,omitempty"`
    LR2IRNotes     string  `json:"lr2irNotes,omitempty"`
    WorkingBodyURL string  `json:"workingBodyUrl,omitempty"`
    WorkingDiffURL string  `json:"workingDiffUrl,omitempty"`
}
```

## 設定

```go
type Config struct {
    SongDataDBPath string // songdata.dbのファイルパス（ユーザーが指定）
    ElsaDBPath     string // elsa.dbの保存先（デフォルト: アプリ設定ディレクトリ）
}
```

- 初回起動時にsongdata.dbのパスを指定（ファイル選択ダイアログ）
- elsa.dbはアプリ設定ディレクトリに自動作成

## エラーハンドリング

| 状況 | 対応 |
|------|------|
| songdata.dbが見つからない | 設定画面でパス再指定を促す |
| songdata.dbのスキーマが想定と異なる | バージョン不一致警告 |
| LR2IRアクセス失敗 | リトライせず、エラー表示。ユーザーが再試行 |
| elsa.dbマイグレーション失敗 | 起動時エラー表示 |

## 将来拡張ポイント

| 機能 | 拡張方法 |
|------|----------|
| フォルダ移動 | usecase/move_song.go + port/filesystem.go 追加 |
| 楽曲導入（本体） | usecase/import_song.go 追加 |
| 差分導入 | usecase/import_chart.go 追加 |
| URL書き換えルール | url_rewrite_rulesテーブル追加、ドメイン置換ロジック |
| イベントページパース | port/event_page_parser.go + アダプタ追加 |
| 未導入楽曲表示 | song_metaにtitle/artist等のカラム追加、songdata.db非依存レコード対応 |
| 難易度表連携 | 外部難易度表JSONの取得・パース |
| BMSファイルパーサー | port/bms_parser.go + adapter/parser/ 追加 |
| MD5/SHA256計算 | port/hasher.go + adapter/filesystem/hasher.go 追加 |
| 重複検知 | usecase/find_duplicates.go（同一md5の複数パス検出） |
