# パス検索機能 実装計画

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 楽曲一覧・譜面一覧にパス検索トグルを追加し、ON時はフォルダパスのみを検索対象にする

**Architecture:** songdata.songの`path`カラムを各DTOに追加し、フロントエンドのsearchFilter関数でトグル状態に応じて検索対象を切り替える。楽曲一覧はfolder単位のため、song_groups CTEで`MIN(s.path)`を取得して代表パスとする。

**Tech Stack:** Go / SQLite / Svelte 4 / TypeScript / TanStack Table / DaisyUI 5

---

## ファイル構成

| 変更種別 | ファイル | 変更内容 |
|---------|---------|---------|
| Modify | `internal/domain/model/song.go` | Song構造体に`Path`追加 |
| Modify | `internal/adapter/persistence/songdata_reader.go` | ListAllSongs/ListAllChartsのSQLに`path`追加、Scan追加 |
| Modify | `internal/app/dto/dto.go` | SongRowDTO/ChartListItemDTOに`Path`追加、SongToRowDTOマッピング追加 |
| Modify | `internal/app/chart_handler.go` | ChartListItemDTO生成時にPath追加 |
| Modify | `frontend/src/views/SongTable.svelte` | pathSearchトグルUI追加、searchFilter分岐 |
| Modify | `frontend/src/views/ChartListView.svelte` | pathSearchトグルUI追加、searchFilter分岐 |

---

## Task 1: バックエンド — model/DTO/SQLにPath追加

**Files:**
- Modify: `internal/domain/model/song.go:5-20`
- Modify: `internal/adapter/persistence/songdata_reader.go:200-211` (ListAllSongs CTE)
- Modify: `internal/adapter/persistence/songdata_reader.go:239-244` (ListAllSongs Scan)
- Modify: `internal/adapter/persistence/songdata_reader.go:396-410` (ChartListItem struct)
- Modify: `internal/adapter/persistence/songdata_reader.go:414-436` (ListAllCharts SQL)
- Modify: `internal/adapter/persistence/songdata_reader.go:448-451` (ListAllCharts Scan)
- Modify: `internal/app/dto/dto.go:16-27` (SongRowDTO)
- Modify: `internal/app/dto/dto.go:75-90` (ChartListItemDTO)
- Modify: `internal/app/dto/dto.go:121-134` (SongToRowDTO)
- Modify: `internal/app/chart_handler.go:35-47` (ChartListItemDTO生成)

- [ ] **Step 1: model.SongにPathフィールドを追加**

`internal/domain/model/song.go` の Song構造体に追加:
```go
type Song struct {
	FolderHash string
	Title      string
	Artist     string
	Genre      string
	Path       string  // 代表ファイルパス（MIN(path)）
	MinBPM     float64
	// ... 以下既存フィールド
}
```

- [ ] **Step 2: SongRowDTO / ChartListItemDTOにPathフィールドを追加**

`internal/app/dto/dto.go`:

SongRowDTO に追加:
```go
Path        string  `json:"path"`
```

ChartListItemDTO に追加:
```go
Path        string  `json:"path"`
```

SongToRowDTO のマッピングに追加:
```go
Path:        s.Path,
```

- [ ] **Step 3: ChartListItem構造体にPathフィールドを追加**

`internal/adapter/persistence/songdata_reader.go` のChartListItem構造体に追加:
```go
Path        string
```

- [ ] **Step 4: ListAllSongsのSQLとScanにpath追加**

song_groups CTEに `MIN(s.path) AS path` を追加:
```sql
song_groups AS (
    SELECT
        s.folder,
        COALESCE(MIN(CASE WHEN s.title != '' THEN s.title END), '') AS title,
        COALESCE(MIN(CASE WHEN s.artist != '' THEN s.artist END), '') AS artist,
        COALESCE(MIN(CASE WHEN s.genre != '' THEN s.genre END), '') AS genre,
        MIN(s.path) AS path,
        MIN(bm.min_bpm) AS min_bpm,
        MIN(bm.max_bpm) AS max_bpm,
        COUNT(*) AS chart_count
    FROM songdata.song s
    JOIN bpm_mode bm ON bm.folder = s.folder
    GROUP BY s.folder
)
```

SELECT句に `sg.path` を追加:
```sql
SELECT
    sg.folder, sg.title, sg.artist, sg.genre, sg.path,
    sg.min_bpm, sg.max_bpm, sg.chart_count,
    ...
```

Scanに `&s.Path` を追加:
```go
if err := rows.Scan(
    &s.FolderHash, &s.Title, &s.Artist, &s.Genre, &s.Path,
    &s.MinBPM, &s.MaxBPM, &s.ChartCount,
    &releaseYear, &eventName,
    &s.HasIRMeta,
); err != nil {
```

- [ ] **Step 5: ListAllChartsのSQLとScanにpath追加**

SELECT句に `s.path` を追加:
```sql
SELECT
    s.md5,
    s.title,
    COALESCE(s.subtitle, ''),
    s.artist,
    COALESCE(s.subartist, ''),
    s.genre,
    s.path,
    s.minbpm,
    ...
```

Scanに `&c.Path` を追加:
```go
if err := rows.Scan(
    &c.MD5, &c.Title, &c.Subtitle, &c.Artist, &c.SubArtist, &c.Genre,
    &c.Path,
    &c.MinBPM, &c.MaxBPM, &c.Difficulty, &c.Notes,
    &eventName, &releaseYear, &c.HasIRMeta,
); err != nil {
```

- [ ] **Step 6: chart_handler.goのChartListItemDTO生成にPath追加**

`internal/app/chart_handler.go` のListCharts()内のDTO生成に追加:
```go
result[i] = dto.ChartListItemDTO{
    MD5:        c.MD5,
    Title:      c.Title,
    // ...
    Path:       c.Path,
    // ...
}
```

- [ ] **Step 7: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: 成功

- [ ] **Step 8: テスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./...`
Expected: 全テストPASS

- [ ] **Step 9: コミット**

```bash
git add internal/domain/model/song.go internal/adapter/persistence/songdata_reader.go internal/app/dto/dto.go internal/app/chart_handler.go
git commit -m "feat: 楽曲・譜面DTOにPathフィールドを追加"
```

---

## Task 2: フロントエンド — SongTableにパス検索トグル追加

**Files:**
- Modify: `frontend/src/views/SongTable.svelte:39-48` (searchFilter)
- Modify: `frontend/src/views/SongTable.svelte:185` (SearchInput周辺のテンプレート)

- [ ] **Step 1: pathSearch状態変数を追加**

`SongTable.svelte` のscriptブロックに追加:
```typescript
let pathSearch = false
```

- [ ] **Step 2: searchFilter関数をpathSearch対応に修正**

```typescript
const searchFilter: FilterFn<dto.SongRowDTO> = (row, _columnId, filterValue) => {
  const s = (filterValue as string).toLowerCase()
  const item = row.original
  if (pathSearch) {
    return (item.path || '').toLowerCase().includes(s)
  }
  return (
    item.title.toLowerCase().includes(s) ||
    item.artist.toLowerCase().includes(s) ||
    item.genre.toLowerCase().includes(s) ||
    (item.eventName || '').toLowerCase().includes(s)
  )
}
```

注意: searchFilterはconst関数だが、TanStack Tableはフィルタ値が変わるたびにこの関数を再評価するため、pathSearch変数の変更は globalFilter の再設定（後述）でトリガーする。

- [ ] **Step 3: pathSearchトグル変更時にフィルタを再トリガー**

pathSearchが変わった時にフィルタを再評価させるリアクティブ文を追加:
```typescript
$: if (pathSearch !== undefined) {
  // トグル変更時にフィルタを再トリガー
  globalFilter = globalFilter
}
```

注意: TanStack TableのglobalFilterは値が変わらないと再評価されない。同じ値を再代入することで強制トリガーする。もし動作しない場合は、一旦空文字にしてから戻すなどのワークアラウンドを検討。

- [ ] **Step 4: SearchInput横にトグルボタンを追加**

SearchInputの隣にDaisyUIのbtnで「パス検索」トグルを追加:
```svelte
<div class="flex items-center gap-2">
  <SearchInput bind:value={globalFilter} />
  <button
    class="btn btn-sm {pathSearch ? 'btn-primary' : 'btn-ghost'}"
    on:click={() => pathSearch = !pathSearch}
    title="フォルダパスから検索"
  >パス</button>
</div>
```

既存の `<SearchInput bind:value={globalFilter} />` をこの `<div>` で置き換える。

- [ ] **Step 5: フロントエンドビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run build`
Expected: 成功

- [ ] **Step 6: コミット**

```bash
git add frontend/src/views/SongTable.svelte
git commit -m "feat: 楽曲一覧にパス検索トグルを追加"
```

---

## Task 3: フロントエンド — ChartListViewにパス検索トグル追加

**Files:**
- Modify: `frontend/src/views/ChartListView.svelte:80-90` (searchFilter)
- Modify: `frontend/src/views/ChartListView.svelte:260` (SearchInput周辺のテンプレート)

- [ ] **Step 1: pathSearch状態変数を追加**

`ChartListView.svelte` のscriptブロックに追加:
```typescript
let pathSearch = false
```

- [ ] **Step 2: searchFilter関数をpathSearch対応に修正**

```typescript
const searchFilter: FilterFn<dto.ChartListItemDTO> = (row, _columnId, filterValue) => {
  const s = (filterValue as string).toLowerCase()
  const item = row.original
  if (pathSearch) {
    return (item.path || '').toLowerCase().includes(s)
  }
  return (
    item.title.toLowerCase().includes(s) ||
    (item.subtitle || '').toLowerCase().includes(s) ||
    item.artist.toLowerCase().includes(s) ||
    (item.subArtist || '').toLowerCase().includes(s) ||
    item.genre.toLowerCase().includes(s)
  )
}
```

- [ ] **Step 3: pathSearchトグル変更時にフィルタを再トリガー**

```typescript
$: if (pathSearch !== undefined) {
  globalFilter = globalFilter
}
```

- [ ] **Step 4: SearchInput横にトグルボタンを追加**

SongTableと同様のパターン:
```svelte
<div class="flex items-center gap-2">
  <SearchInput bind:value={globalFilter} />
  <button
    class="btn btn-sm {pathSearch ? 'btn-primary' : 'btn-ghost'}"
    on:click={() => pathSearch = !pathSearch}
    title="フォルダパスから検索"
  >パス</button>
</div>
```

- [ ] **Step 5: フロントエンドビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run build`
Expected: 成功

- [ ] **Step 6: コミット**

```bash
git add frontend/src/views/ChartListView.svelte
git commit -m "feat: 譜面一覧にパス検索トグルを追加"
```

---

## Task 4: Wailsバインディング再生成・最終確認

- [ ] **Step 1: Wailsバインディング再生成**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails generate module`
Expected: frontend/wailsjs/go/models.ts が更新される（Pathフィールドが追加された型定義）

- [ ] **Step 2: フロントエンドビルド最終確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run build`
Expected: 成功

- [ ] **Step 3: Goテスト最終確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./...`
Expected: 全テストPASS

- [ ] **Step 4: ドキュメント更新**

`README.md` の機能一覧に追記:
```
- パス検索（楽曲一覧・譜面一覧でフォルダパスによる検索、トグルで切り替え）
```

`docs/manual.md` の楽曲一覧・譜面一覧セクションにパス検索の説明を追記。

`docs/TODO.md` に実装済み項目を追加。

- [ ] **Step 5: コミット**

```bash
git add README.md docs/manual.md docs/TODO.md
git commit -m "docs: パス検索機能をドキュメントに反映"
```
