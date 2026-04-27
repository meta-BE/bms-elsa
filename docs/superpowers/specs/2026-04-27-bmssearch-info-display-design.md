# BMS Search 情報表示機能 設計

## 概要

BMS Search 由来の楽曲メタデータを、既存の「IR情報」表示と並列に各詳細画面で表示する機能。
イベント・公開日・DLリンク・プレビュー等を確認できるようにし、楽曲の理解を助ける。

## 背景・モチベーション

- 既存では LR2IR 情報のみが詳細画面に表示されている
- BMS Search 連携は実装済みだが、現状はイベント紐付け（`song_meta.bms_search_id` への保存）と
  楽曲詳細での「BMS Search リンク」だけで、取得済みの楽曲メタデータ自体は表示していない
- BMS Search は譜面 md5 のカバー率が約70%と不完全のため、md5 ヒットだけでは取得しきれない楽曲がある
- 「主に楽曲についてOK。イベントや年数などのメタデータを一覧したい」という要件

## スコープ

### 対象画面（IRInfoCard が表示されている全画面）

- 楽曲詳細（`SongDetail`）
- 譜面詳細（`ChartDetail`）
- 難易度表エントリ詳細（`EntryDetail`）— 未導入譜面でも表示

### 対象データ（BMS API レスポンスから取得・表示する項目）

- 作品タイトル / アーティスト / サブアーティスト / ジャンル
- イベント名・イベントID（`exhibition`）
- 公開日（`publishedAt`、フル日付）
- ダウンロードリンク（`downloads`）
- プレビュー（`previews`、YouTube/SoundCloud/NicoNico）
- 関連リンク（`relatedLinks`）
- BMS Search 作品ページへの直リンク

### 対象外

- bemuseURL、tags、Pattern 固有情報（laneType, totalNotes 等）の保存・表示
- 起動時バックグラウンドでの BMS Search 自動同期
- `song_meta.bms_search_id` の廃止・撤去
- `bmssearch_bms` の GC（参照されなくなったレコードの削除）

## アーキテクチャ

### レイヤー別責務

| レイヤー | 追加・変更 | 役割 |
|---|---|---|
| `domain/model` | `BMSSearchBMS`、`BMSSearchLink` エンティティ、`BMSSearchSource` 型、`BMSSearchRepository` インターフェース | エンティティ・リポジトリ抽象 |
| `adapter/gateway` | `BMSSearchClient` に `SearchBMSesByTitle` 追加、`BMSSearchBMS` struct 拡張 | API 呼び出し |
| `adapter/persistence` | `bmssearch_repository.go` 新規。マイグレーション追加 | DB アクセス |
| `usecase` | `BMSSearchResolver`、`LookupBMSSearchUseCase`、`UnlinkBMSSearchUseCase` 新規。`SyncBMSSearchUseCase` 改修 | ビジネスロジック |
| `app/handler` | `bmssearch_handler.go` 新規 | Wails バインディング |
| `frontend/components` | `BMSSearchInfoCard.svelte` 新規 | UI 表示 |
| `frontend/views` | 3詳細画面に `BMSSearchInfoCard` 配置 | 詳細画面組み込み |

### 重要な設計判断

- 既存 `SyncBMSSearchUseCase` の `song_meta.bms_search_id` 書き込みは継続。
  新スキーマ（`bmssearch_bms_id_md5` + `bmssearch_bms`）への書き込みも同時に行う
- `song_meta.bms_search_id` は「楽曲フォルダ単位の代表 bms_id」として明確に位置付け、継続活用
- 「Resolve+Persist」ロジックは `BMSSearchResolver` に一元化し、
  `LookupBMSSearchUseCase` と `SyncBMSSearchUseCase` から委譲する形にする
- `BMSSearchClient` は既存のレートリミット（100ms間隔）をそのまま使う

## データモデル

### テーブル定義

```sql
-- md5 ↔ bms_id リレーション（純粋なリンクのみ）
CREATE TABLE bmssearch_bms_id_md5 (
    md5         TEXT PRIMARY KEY,
    bms_id      TEXT NOT NULL,
    source      TEXT NOT NULL,        -- 'official' | 'unofficial'
    resolved_at INTEGER NOT NULL      -- UNIX秒
);
CREATE INDEX idx_bmssearch_link_bms_id ON bmssearch_bms_id_md5(bms_id);

-- bms_id 単位の楽曲メタ（BMS API レスポンスのキャッシュ）
CREATE TABLE bmssearch_bms (
    bms_id              TEXT PRIMARY KEY,
    title               TEXT NOT NULL DEFAULT '',
    artist              TEXT NOT NULL DEFAULT '',
    subartist           TEXT NOT NULL DEFAULT '',
    genre               TEXT NOT NULL DEFAULT '',
    exhibition_id       TEXT,                            -- NULL可（独立曲）
    exhibition_name     TEXT NOT NULL DEFAULT '',        -- 表示用denormalize
    published_at        TEXT NOT NULL DEFAULT '',        -- ISO 8601 or 空
    downloads_json      TEXT NOT NULL DEFAULT '[]',      -- [{url, description}]
    previews_json       TEXT NOT NULL DEFAULT '[]',      -- [{service, parameter}]
    related_links_json  TEXT NOT NULL DEFAULT '[]',      -- [{url, description}]
    fetched_at          INTEGER NOT NULL                 -- UNIX秒
);

-- song_meta への source カラム追加
ALTER TABLE song_meta ADD COLUMN bms_search_source TEXT;  -- NULL許容、'official' | 'unofficial'
```

### スキーマ設計の判断

- 文字列カラムは既存 `chart_meta` と揃え `NOT NULL DEFAULT ''`。
  `exhibition_id` のみ NULL 許容（独立曲の表現）
- 配列系は JSON 文字列で保存。Go 側で `json.Marshal/Unmarshal`。空配列は `"[]"`
- `exhibition_name` の denormalize: 表示時の JOIN を避ける。
  既存の `event_mappings.csv` 由来のイベントマスターとは独立した別系統
- `fetched_at`（メタ取得時刻）と `resolved_at`（リンク確定時刻）は分離
- `bms_search_source` は NULL 許容。`bms_search_id IS NULL` なら `bms_search_source IS NULL` で対
- CHECK 制約は付けない（SQLite ALTER TABLE では後付け困難）。整合性はアプリ層で担保

### 3層の役割明確化

| テーブル/カラム | 粒度 | 役割 |
|---|---|---|
| `song_meta.bms_search_id` | フォルダ単位 | 楽曲フォルダの代表 bms_id（既存活用） |
| `song_meta.bms_search_source` | フォルダ単位 | 代表 bms_id の source（追加） |
| `bmssearch_bms_id_md5` | md5 単位 | md5 ↔ bms_id のリレーション + source（追加） |
| `bmssearch_bms` | bms_id 単位 | BMS API レスポンスのメタキャッシュ（追加） |

### 取得フロー（画面別）

- **楽曲詳細**: `folder_hash` → `song_meta.{bms_search_id, bms_search_source}` → `bmssearch_bms`
- **譜面詳細 / 難易度表エントリ詳細**: `md5` → `bmssearch_bms_id_md5.{bms_id, source}` → `bmssearch_bms`

### 書き込みフロー（同期 / 「取得」ボタン）

1. 公式 md5 ヒット時:
   - フォルダ内全 md5 を `bmssearch_bms_id_md5` に `official` で UPSERT
   - 所持譜面の場合 `song_meta.bms_search_id` + `bms_search_source = 'official'` 更新
   - `/bmses/{bms_id}` レスポンスを `bmssearch_bms` に UPSERT
2. フォールバック検索ヒット時:
   - 同様だが source は `unofficial`
3. 「1楽曲1回ルール」により、フォルダ内全 md5 は同じ bms_id を共有
4. source の自動上書きルール: 全 source とも自動同期・手動取得で常に上書き可

### マイグレーション

順序保証:

1. `ALTER TABLE song_meta ADD COLUMN bms_search_source TEXT;`
2. `UPDATE song_meta SET bms_search_source = 'official' WHERE bms_search_id IS NOT NULL;`
   （これまで `bms_search_id` が入るのは公式 md5 ヒット起因のみ。1回限りの分類補完）
3. `CREATE TABLE bmssearch_bms_id_md5 ...`
4. `CREATE TABLE bmssearch_bms ...`
5. インデックス作成

冪等性: `CREATE TABLE IF NOT EXISTS`、`ADD COLUMN` の冪等処理で2回実行しても問題なし。

## ドメインエンティティ

```go
type BMSSearchSource string

const (
    BMSSearchSourceOfficial   BMSSearchSource = "official"
    BMSSearchSourceUnofficial BMSSearchSource = "unofficial"
)

type BMSSearchLink struct {
    MD5        string
    BMSID      string
    Source     BMSSearchSource
    ResolvedAt time.Time
}

type BMSSearchBMS struct {
    BMSID          string
    Title          string
    Artist         string
    SubArtist      string
    Genre          string
    ExhibitionID   *string  // nullable
    ExhibitionName string
    PublishedAt    string   // ISO 8601 or ""
    Downloads      []BMSSearchURLEntry
    Previews       []BMSSearchPreview
    RelatedLinks   []BMSSearchURLEntry
    FetchedAt      time.Time
}

type BMSSearchURLEntry struct {
    URL         string
    Description string
}

type BMSSearchPreview struct {
    Service   string  // "YOUTUBE" | "SOUNDCLOUD" | "NICONICO"
    Parameter string
}
```

## DTO

```go
type BMSSearchInfoDTO struct {
    HasInfo        bool                       `json:"hasInfo"`
    BMSID          string                     `json:"bmsId,omitempty"`
    Source         string                     `json:"source,omitempty"`        // "official" | "unofficial"
    Title          string                     `json:"title,omitempty"`
    Artist         string                     `json:"artist,omitempty"`
    SubArtist      string                     `json:"subArtist,omitempty"`
    Genre          string                     `json:"genre,omitempty"`
    ExhibitionID   string                     `json:"exhibitionId,omitempty"`
    ExhibitionName string                     `json:"exhibitionName,omitempty"`
    PublishedAt    string                     `json:"publishedAt,omitempty"`
    Downloads      []BMSSearchURLEntryDTO     `json:"downloads,omitempty"`
    Previews       []BMSSearchPreviewDTO      `json:"previews,omitempty"`
    RelatedLinks   []BMSSearchURLEntryDTO     `json:"relatedLinks,omitempty"`
}
```

`HasInfo: false` の場合は他フィールド未設定。フロントは「情報がありません」プレースホルダー表示。

## ゲートウェイ層

### `BMSSearchClient` 拡張

```go
// 既存 BMSSearchBMS struct を拡張
type BMSSearchBMS struct {
    ID            string                `json:"id"`
    Title         string                `json:"title"`
    Artist        string                `json:"artist"`
    SubArtist     string                `json:"subartist"`
    Genre         string                `json:"genre"`
    Exhibition    *BMSSearchExhibition  `json:"exhibition"`
    PublishedAt   string                `json:"publishedAt"`
    Downloads     []BMSSearchURLEntry   `json:"downloads"`
    Previews      []BMSSearchPreview    `json:"previews"`
    RelatedLinks  []BMSSearchURLEntry   `json:"relatedLinks"`
}

type BMSSearchURLEntry struct {
    URL         string `json:"url"`
    Description string `json:"description"`
}

type BMSSearchPreview struct {
    Service   string `json:"service"`
    Parameter string `json:"parameter"`
}

// テキスト検索フォールバック用メソッド
func (c *BMSSearchClient) SearchBMSesByTitle(
    ctx context.Context,
    title string,
    limit int,
) ([]BMSSearchBMS, error)
// GET /bmses/search?title={title}&limit={limit}&orderBy=PUBLISHED&orderDirection=DESC
```

### 既存メソッドの後方互換性

- `LookupPatternByMD5`, `LookupBMS`, `FetchAllExhibitions` のシグネチャは変更なし
- `BMSSearchBMS` のフィールド追加は JSON タグなので既存読み込み処理に影響なし
- 既存 `SyncBMSSearchUseCase` への影響は最小限

## ユースケース層

### `BMSSearchResolver`（内部、共通ロジック）

```go
type BMSSearchResolver struct {
    bmsClient     *gateway.BMSSearchClient
    bmssearchRepo model.BMSSearchRepository
    metaRepo      model.MetaRepository
    songRepo      model.SongRepository
}

// 楽曲フォルダ単位の解決（既存同期 + 楽曲詳細/譜面詳細「取得」ボタンから利用）
func (r *BMSSearchResolver) ResolveForFolder(
    ctx context.Context,
    folderHash string,
    md5s []string,
    title, artist string,
) (resolvedBMSID string, source model.BMSSearchSource, err error)

// 未所持 md5 単位の解決（難易度表エントリ「取得」ボタンから利用）
func (r *BMSSearchResolver) ResolveForOrphanMD5(
    ctx context.Context,
    md5, title, artist string,
) (resolvedBMSID string, source model.BMSSearchSource, err error)
```

#### `ResolveForFolder` の処理

1. md5s を順に `/patterns/{md5}` で試行（既存同期と同じロジック）
2. 公式ヒット時:
   - 取得した bms_id を `bmssearch_bms_id_md5` に **md5s 全件** を `official` で UPSERT
   - `song_meta.bms_search_id` + `bms_search_source = 'official'` 更新
   - `/bmses/{bms_id}` で `bmssearch_bms` UPSERT
3. 公式失敗時:
   - `/bmses/search?title={title}&limit=20&orderBy=PUBLISHED&orderDirection=DESC` を1回実行
   - 結果をスコアリング → 採用 1件あれば、md5s 全件を `unofficial` で UPSERT、`song_meta` 更新、`bmssearch_bms` UPSERT
4. フォールバックも採用なし（閾値未満 or 同点首位）→ 何も書かない、`source = ""` を返す

「1楽曲1回ルール」: フォルダ内全 md5 試行 → 失敗時にフォールバック検索1回。Resolver 内に閉じる。

#### `ResolveForOrphanMD5` の処理

- ResolveForFolder と同様だが、md5 単一で書き込む（`song_meta` は触らない、`bmssearch_bms_id_md5` のみ）

### フォールバック検索の正規化・スコアリング

2026-04-27 実施の事前調査スパイク（`2026-04-27-bmssearch-info-fallback-probe.md`）の結果に基づき確定。

**クエリ構築**

- `?title={title}&limit=20&orderBy=PUBLISHED&orderDirection=DESC`
- 試行順: **raw（原文）→ stripped（末尾剥離後）→ 採用なし** の2段階
  - `normalized`（小文字化）は今回のサンプルで raw と同等結果。BMS Search 側が原文を保持しているため単独試行の効果が低いと判断し省略。ただし NFKC 正規化はスコアリングの正規化後比較に利用する
- 末尾剥離: `[...]`, `(...)`, `-...-` をループで除去（右から1段階ずつ）

**スコア配点（最大100点）**

| 項目 | 配点 | 備考 |
|---|---|---|
| title 完全一致 | +60 | 最重要 |
| title 正規化後完全一致 | +50 | 揺れ吸収（記号・空白・大小・全半角） |
| title 部分一致 | +25 | 弱い手がかり |
| artist 完全一致 | +20 | アーティスト一致は表記揺れが多いため上限を低く |
| artist 正規化後完全一致 | +15 | 同上 |
| artist トークン共通率 × 10 | 0〜+10 | feat./BGI 等の表記揺れに弱く対応 |

**スコア計算ルール**

- title 系3項目（完全一致/正規化後完全一致/部分一致）からは**最高スコア1項目のみ採用**（排他、最大+60）
- artist 系2項目（完全一致/正規化後完全一致）からも**最高スコア1項目のみ採用**（排他、最大+20）
- artist トークン共通率は上記とは独立に加算（最大+10）
- 合計最大スコア: 60 + 20 + 10 = **90点**（理論最大）

**閾値**

- 最高スコア < **50** なら採用しない（unofficial にもしない、未紐付けのまま）
- 同点首位が複数あった場合も採用しない（曖昧）
- title 部分一致（+25）のみでは閾値50に届かない設計 → artist 一致がセーフガードとして機能する

**調査サンプルでの採用率見積もり**: 10件中7〜8件で採用確実または採用妥当（誤紐付けは0件）

### `LookupBMSSearchUseCase`（詳細画面ボタン用）

```go
type LookupBMSSearchUseCase struct {
    resolver *BMSSearchResolver
    songRepo model.SongRepository
    dtRepo   model.DifficultyTableRepository
}

func (u *LookupBMSSearchUseCase) Execute(
    ctx context.Context,
    md5 string,
) (*dto.BMSSearchInfoDTO, error)
```

#### 処理

1. `songRepo.FindFolderByMD5(md5)` で md5 の所属フォルダを引く
2. 所持譜面（フォルダあり）: `ResolveForFolder(folderHash, mdsInFolder, song.Title, song.Artist)`
3. 未所持譜面（フォルダなし）: 難易度表エントリから title/artist を取得 → `ResolveForOrphanMD5(md5, entry.Title, entry.Artist)`
4. 結果の bms_id から `bmssearch_bms` を読み出して `BMSSearchInfoDTO` で返却

### `SyncBMSSearchUseCase` の改修ポイント

- 既存の inline 処理（`syncFolder` 内の Pattern → BMS → metaRepo 更新）を `BMSSearchResolver.ResolveForFolder` に置き換え
- 並列実行・進捗表示・中断対応・bmsCache（同 BMS API 重複呼び出し回避）はそのまま維持
- フォールバック検索もこのフロー内で自動実施（既存「BMS Search同期」は手動一括実行）

#### `bmsCache` の扱い

- BMS API は `/bmses/{id}` 単位でキャッシュ可能（同期実行内）。Resolver 側にも引き渡してキャッシュヒットを利用
- 永続化されたキャッシュ（`bmssearch_bms` テーブル）は同期実行をまたいでも有効

### `UnlinkBMSSearchUseCase`（解除）

```go
// 楽曲フォルダ単位の解除（楽曲詳細・譜面詳細から）
func (u *UnlinkBMSSearchUseCase) UnlinkByFolder(ctx, folderHash) error
// 1. song_meta.bms_search_id, bms_search_source を NULL に
// 2. フォルダ内全 md5 の bmssearch_bms_id_md5 を DELETE

// 未所持 md5 単位の解除（難易度表エントリ詳細から）
func (u *UnlinkBMSSearchUseCase) UnlinkByMD5(ctx, md5) error
// 1. その md5 の bmssearch_bms_id_md5 を DELETE
```

`bmssearch_bms` のレコードは**削除しない**（共有キャッシュとして保持。次回再リンク時に即利用可、API 削減）。

## ハンドラー層

`internal/app/bmssearch_handler.go` 新規:

```go
type BMSSearchHandler struct {
    lookupUC *usecase.LookupBMSSearchUseCase
    unlinkUC *usecase.UnlinkBMSSearchUseCase
    repo     model.BMSSearchRepository
    songRepo model.SongRepository
}

// DBから読むだけ（API叩かない）。詳細画面の初期表示で使用
func (h *BMSSearchHandler) GetBMSSearchInfoByMD5(md5 string) (*dto.BMSSearchInfoDTO, error)

// 「取得」ボタン押下時。Resolver経由で取得＆保存
func (h *BMSSearchHandler) LookupBMSSearchByMD5(md5 string) (*dto.BMSSearchInfoDTO, error)

// 解除（所持譜面）
func (h *BMSSearchHandler) UnlinkBMSSearchByFolder(folderHash string) error

// 解除（未所持md5）
func (h *BMSSearchHandler) UnlinkBMSSearchByMD5(md5 string) error
```

`main.go` の Wails `Bind:` に `bmssearchHandler` を追加。フロントには
`wailsjs/go/app/BMSSearchHandler.{Get,Lookup,UnlinkByFolder,UnlinkByMD5}` として生える。

## フロントエンド

### `BMSSearchInfoCard.svelte`（新規）

`IRInfoCard.svelte` と同じスケール感（`bg-base-200 rounded-lg p-3`）。

#### Props / Events

```typescript
export let md5: string
export let folderHash: string = ''  // 空なら未所持md5扱い
export let info: BMSSearchInfoDTO | null = null

const dispatch = createEventDispatcher<{
  lookup: void
  unlink: void
}>()
```

#### 表示要素

- ヘッダー: タイトル「BMS Search情報」+ 作品ページリンク + （unofficial時のみ）虫眼鏡アイコン + ツールチップ + 「取得」ボタン + （情報あり時のみ）「解除」ボタン
- 情報なし時: プレースホルダー「BMS Search情報がありません。「取得」ボタンで取得してください。」
- 情報あり時: 作品タイトル/アーティスト/サブアーティスト/ジャンル/イベント名+リンク/公開日/DLリンク/プレビュー/関連リンク

#### source バッジ

- `official`: 何も表示しない
- `unofficial`: 虫眼鏡アイコン（Icon.svelte に `search` を追加）+ ツールチップ「テキスト検索により自動推定された紐付けです」

#### 「取得」ボタン挙動

- 通常: 「取得」テキスト
- 取得中: `loading loading-spinner loading-xs`、ボタン disabled
- DLリンク・関連リンク URL は `applyRewriteRules($rewriteRules, url)` を適用（既存 IRInfoCard と同じ）

#### 「解除」ボタン挙動

- `info?.hasInfo === true` のときのみ表示
- 押下で確認ダイアログなしに即時解除（軽い操作なので、誤クリックはすぐ「取得」で復旧可）

#### プレビュー表示

- YouTube: `https://www.youtube.com/watch?v={parameter}`
- SoundCloud: `parameter` がそのまま URL
- NicoNico: `https://www.nicovideo.jp/watch/{parameter}`
- リンク表示のみ（動画埋め込みはしない）

#### 表示形式の検討余地

要素が縦に並びすぎる懸念がある。実装段階で必要なら以下の選択肢を検討:

- 楽曲メタ部分を2列 grid に
- DLリンク/プレビュー/関連リンクは折りたたみ表示

具体形は実装後にレビュー。

### 各画面への配置

| 画面 | 既存構成 | BMSSearchInfoCard 追加位置 |
|---|---|---|
| `SongDetail` | 楽曲ヘッダー → 譜面一覧 → (選択時) ChartInfoCard → IRInfoCard | **楽曲ヘッダーと譜面一覧の間**に「楽曲レベル」として常時表示（譜面選択不要） |
| `ChartDetail` | 譜面ヘッダー → ChartInfoCard → IRInfoCard | ChartInfoCard と IRInfoCard の間 |
| `EntryDetail` | エントリ基本情報 → (導入済) ChartInfoCard / (未導入) InstallCandidateCard → IRInfoCard | IRInfoCard の下 |

#### `SongDetail` を「楽曲レベル」とする理由

- BMS Search 情報は楽曲（フォルダ）単位の情報で、譜面に依存しない
- 楽曲ヘッダーの Event/Year（song_meta 由来）の隣に楽曲レベルの BMS Search 情報があるのは自然
- 譜面未選択でも表示され、「取得」ボタンが押せる

#### `SongDetail` で渡す md5

- フォルダの代表として `detail.charts[0].md5` を渡す
- 内部的には Resolver がフォルダ単位で動くので、どの md5 を起点にしても同じ bmssearch_bms に行き着く

### 取得・解除のハンドラー呼び分け

```typescript
// 取得
async function lookupBMSSearch(md5: string) {
  await LookupBMSSearchByMD5(md5)
  // データ再読み込み
}

// 解除（SongDetail/ChartDetail = 所持譜面、EntryDetail で導入済の場合）
async function unlinkBMSSearch() {
  await UnlinkBMSSearchByFolder(folderHash)
  // データ再読み込み
}

// 解除（EntryDetail で未導入時 = 未所持md5）
async function unlinkBMSSearchOrphan() {
  await UnlinkBMSSearchByMD5(md5)
  // データ再読み込み
}
```

### データ取得タイミング

- 詳細画面オープン時、既存の `loadDetail` / `loadChart` / `loadEntry` の中で `GetBMSSearchInfoByMD5(md5)` も並列に呼ぶ（DB読みなので軽い）
- 結果を `bmssearchInfo` ステートに格納して `BMSSearchInfoCard` に渡す

## 事前調査スパイク（実装計画の最初のタスク）

### 目的

フォールバック検索のスコア配点・閾値・正規化ルールを実データで検証し、確定する。

### 手段

- `cmd/probe-bmssearch/main.go` を一時作成
- 実行後は削除 or `cmd/` に残置（再現性目的、最終判断は実装時）

### サンプル

`testdata/songdata.db` から **公式 md5 ヒット5曲 + ミス5曲（合計10曲）** を抽出。

### 検証パターン

各楽曲について以下のクエリを試行:

1. 原文 title で `/bmses/search`
2. 正規化後 title（記号除去・全半角統一）で `/bmses/search`
3. 末尾の `[...]` `(...)` `-...-` 剥離後 title で `/bmses/search`

### 保存物

- レスポンス JSON を `docs/superpowers/specs/data/bmssearch-probe/{date}/` に保存（リプレイ可能）
- 調査レポート: `docs/superpowers/specs/{調査実施日YYYY-MM-DD}-bmssearch-info-fallback-probe.md`（実施日に確定）
  - 末尾付帯文字列パターン一覧
  - 正規化ルール最終案
  - スコア配点最終案（質問10の初期案からの調整有無）
  - 閾値最終値（初期値50からの調整有無）
  - top1 採用率と誤紐付け率の見積もり
  - 採用結果が出なかった楽曲の取り扱い（再試行ロジックの有無）

### 反映先

調査結果を踏まえて本設計ドキュメントの「フォールバック検索の正規化・スコアリング」セクションを更新してから実装に入る。

## テスト戦略

### 単体テスト

| 対象 | テストファイル | 主な確認項目 |
|---|---|---|
| `BMSSearchClient` 拡張 | `internal/adapter/gateway/bmssearch_client_test.go` 追記 | `SearchBMSesByTitle` 正常系・404・空配列・パラメータ組立 |
| `BMSSearchRepository` | `internal/adapter/persistence/bmssearch_repository_test.go` 新規 | `bmssearch_bms_id_md5` UPSERT/DELETE、`bmssearch_bms` UPSERT、source 整合、フォルダ単位削除 |
| 正規化・スコアリング（pure functions） | `internal/usecase/bmssearch_scoring_test.go` 新規 | 各種正規化変換、スコア計算、閾値判定、同点首位の保留 |
| `BMSSearchResolver` | `internal/usecase/bmssearch_resolver_test.go` 新規 | 公式ヒットパス、フォールバックパス、閾値未満で未紐付け、ResolveForFolder/Orphan の差異 |
| `LookupBMSSearchUseCase` | `internal/usecase/lookup_bmssearch_test.go` 新規 | 所持/未所持の分岐、エラーハンドリング |
| `UnlinkBMSSearchUseCase` | `internal/usecase/unlink_bmssearch_test.go` 新規 | フォルダ単位削除（song_meta 更新含む）、md5 単位削除 |
| `SyncBMSSearchUseCase` 改修 | 既存 `sync_bmssearch_test.go` 拡充 | フォールバック含む既存挙動の回帰、新スキーマへの書き込み |

### モック方針

- `BMSSearchClient` は interface を切ってモック化（既存パターン踏襲）
- リポジトリは既存の sqlite テストヘルパー（テスト用 elsa.db）を使用

### マイグレーションテスト

`internal/adapter/persistence/migration_test.go` 既存に追加:

- 旧スキーマからマイグレーション後に:
  - `bms_search_source` カラムが追加されている
  - `bms_search_id IS NOT NULL` のレコードはすべて `bms_search_source = 'official'` になっている
  - `bmssearch_bms_id_md5`、`bmssearch_bms` テーブルが作成されている
- 冪等性: 2回連続実行でエラーなし

### 手動 QA チェックリスト

1. 詳細画面オープンで既存表示が壊れていない（既存 IRInfoCard・楽曲ヘッダー）
2. 各画面で「取得」ボタン → 公式ヒット楽曲 → official 表示・bmssearch_bms 内容反映
3. 公式ミス楽曲 → フォールバック発動 → unofficial 虫眼鏡表示
4. 解除ボタン → カードが「情報なし」表示に戻る
5. 既存「BMS Search同期」（一括手動）が新スキーマにも書く
6. 未所持md5（難易度表エントリ）で「取得」ボタン動作
7. UI 上の URL書き換えルールが DLリンク・関連リンクに適用されている
8. 同フォルダの別譜面を選んでも同じ BMS Search 情報が表示される（共有キャッシュ動作）

## マニュアル更新

`docs/manual.md` の以下のセクションに追記:

- 楽曲詳細・譜面詳細・難易度表エントリ詳細の節に BMS Search 情報カードの説明
- 「取得」ボタン・「解除」ボタンの操作説明
- official / unofficial の意味（虫眼鏡アイコン）
- 既存「BMS Search同期」フロー説明にフォールバック検索の動作追記

## 実装順（参考）

1. 事前調査スパイク → 設計ドキュメント更新（フォールバック検索仕様）
2. マイグレーション + リポジトリ実装 + マイグレーションテスト
3. ゲートウェイ層拡張 + テスト
4. `BMSSearchResolver` + 正規化・スコアリング pure functions + テスト
5. `LookupBMSSearchUseCase` + `UnlinkBMSSearchUseCase` + テスト
6. `SyncBMSSearchUseCase` 改修 + 既存テスト拡充
7. ハンドラー追加 + Wails バインディング
8. フロントエンド `BMSSearchInfoCard.svelte` 実装
9. 各詳細画面への配置
10. マニュアル更新
11. 手動 QA
