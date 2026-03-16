# subtitle/subartist 表示追加 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** songdata.dbのsubtitle/subartistフィールドを譜面一覧・詳細画面に表示する

**Architecture:** ドメインモデルにSubtitle追加 → バックエンドSQL拡張 → DTO追加 → Wailsバインディング再生成 → フロントエンド各コンポーネントで表示。SongTableは楽曲グループ単位のため対象外。

**Tech Stack:** Go / SQLite / Svelte / Tanstack Table / @tanstack/svelte-virtual / Wails v2

---

### Task 1: ドメインモデル・DTO・変換関数の更新

**Files:**
- Modify: `internal/domain/model/song.go:23-39` — Chart構造体にSubtitle追加
- Modify: `internal/app/dto/dto.go:39-70` — ChartDTO, ChartListItemDTOにフィールド追加
- Modify: `internal/app/dto/dto.go:109-138` — ChartToDTO変換にSubtitle/SubArtist追加

**Step 1: Chart構造体にSubtitleフィールドを追加**

`internal/domain/model/song.go` のChart構造体（行23-39）に`Subtitle`を追加:

```go
type Chart struct {
	MD5        string
	SHA256     string
	Title      string
	Subtitle   string  // ← 追加
	Artist     string
	SubArtist  string
	Genre      string
	Mode       int
	Difficulty int
	Level      int
	MinBPM     float64
	MaxBPM     float64
	Path       string
	IRMeta           *ChartIRMeta
	DifficultyLabels []DifficultyLabel
}
```

**Step 2: ChartDTOにSubtitle/SubArtistフィールドを追加**

`internal/app/dto/dto.go` のChartDTO（行39-56）に追加:

```go
type ChartDTO struct {
	MD5            string  `json:"md5"`
	SHA256         string  `json:"sha256"`
	Title          string  `json:"title"`
	Subtitle       string  `json:"subtitle,omitempty"`   // ← 追加
	Artist         string  `json:"artist,omitempty"`     // ← 追加（現在未定義だが表示で使用中）
	SubArtist      string  `json:"subArtist,omitempty"`  // ← 追加
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
	WorkingDiffURL   string               `json:"workingDiffUrl,omitempty"`
	DifficultyLabels []DifficultyLabelDTO `json:"difficultyLabels,omitempty"`
}
```

注: `Artist`フィールドはChartDTOに未定義だがChartDetail.svelteで`chart?.artist`として参照されている。この機会に追加する。

**Step 3: ChartListItemDTOにSubtitle/SubArtistフィールドを追加**

`internal/app/dto/dto.go` のChartListItemDTO（行58-70）に追加:

```go
type ChartListItemDTO struct {
	MD5         string  `json:"md5"`
	Title       string  `json:"title"`
	Subtitle    string  `json:"subtitle,omitempty"`   // ← 追加
	Artist      string  `json:"artist"`
	SubArtist   string  `json:"subArtist,omitempty"`  // ← 追加
	Genre       string  `json:"genre"`
	MinBPM      float64 `json:"minBpm"`
	MaxBPM      float64 `json:"maxBpm"`
	Difficulty  int     `json:"difficulty"`
	EventName   string  `json:"eventName,omitempty"`
	ReleaseYear int     `json:"releaseYear,omitempty"`
	HasIRMeta   bool    `json:"hasIrMeta"`
}
```

**Step 4: ChartToDTO変換にSubtitle/SubArtist/Artistを追加**

`internal/app/dto/dto.go` のChartToDTO関数（行109-138）を更新:

```go
func ChartToDTO(c model.Chart) ChartDTO {
	d := ChartDTO{
		MD5:        c.MD5,
		SHA256:     c.SHA256,
		Title:      c.Title,
		Subtitle:   c.Subtitle,   // ← 追加
		Artist:     c.Artist,     // ← 追加
		SubArtist:  c.SubArtist,  // ← 追加
		Mode:       c.Mode,
		Difficulty: c.Difficulty,
		Level:      c.Level,
		MinBPM:     c.MinBPM,
		MaxBPM:     c.MaxBPM,
		HasIRMeta:  c.IRMeta != nil,
	}
	// ... 残りは変更なし
```

**Step 5: コンパイル確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: PASS

**Step 6: コミット**

```bash
git add internal/domain/model/song.go internal/app/dto/dto.go
git commit -m "feat: Chart/DTOにSubtitle・SubArtistフィールドを追加"
```

---

### Task 2: バックエンドSQL拡張

**Files:**
- Modify: `internal/adapter/persistence/songdata_reader.go:172-182` — GetSongByFolder SQLにsubtitle追加
- Modify: `internal/adapter/persistence/songdata_reader.go:318-337` — ListAllCharts SQLにsubtitle/subartist追加
- Modify: `internal/adapter/persistence/songdata_reader.go:302-313` — ChartListItem構造体に追加
- Modify: `internal/adapter/persistence/songdata_reader.go:369-381` — GetChartByMD5 SQLにsubtitle追加
- Test: `internal/adapter/persistence/songdata_reader_test.go`

**Step 1: ChartListItem構造体にSubtitle/SubArtistを追加**

`songdata_reader.go` のChartListItem（行302-313）を更新:

```go
type ChartListItem struct {
	MD5         string
	Title       string
	Subtitle    string   // ← 追加
	Artist      string
	SubArtist   string   // ← 追加
	Genre       string
	MinBPM      float64
	MaxBPM      float64
	Difficulty  int
	EventName   *string
	ReleaseYear *int
	HasIRMeta   bool
}
```

**Step 2: ListAllChartsのSQLにsubtitle/subartistを追加**

`songdata_reader.go` のListAllCharts SQL（行318-337）を更新。SELECTにs.subtitle, COALESCE(s.subartist, '')を追加し、Scan部分も対応:

SQL変更:
```sql
SELECT
    s.md5,
    s.title,
    COALESCE(s.subtitle, ''),
    s.artist,
    COALESCE(s.subartist, ''),
    s.genre,
    s.minbpm,
    s.maxbpm,
    s.difficulty,
    sm.event_name,
    sm.release_year,
    EXISTS(
        SELECT 1 FROM main.chart_meta cm
        WHERE cm.md5 = s.md5 AND cm.sha256 = s.sha256
    ) AS has_ir_meta
FROM songdata.song s
LEFT JOIN main.song_meta sm ON sm.folder_hash = s.folder
WHERE s.md5 != ''
ORDER BY s.title ASC
```

Scan変更（行349-352付近）に`&item.Subtitle`と`&item.SubArtist`を追加。

**Step 3: GetSongByFolderのSQLにsubtitleを追加**

`songdata_reader.go` のGetSongByFolder SQL（行174-182）を更新:

```sql
SELECT
    s.md5, s.sha256, s.title, COALESCE(s.subtitle, ''), s.artist, COALESCE(s.subartist, ''),
    s.genre, s.mode, s.difficulty, s.level,
    s.minbpm, s.maxbpm, s.path
```

Scan部分にも`&c.Subtitle`を追加（`&c.Title`の後）。

**Step 4: GetChartByMD5のSQLにsubtitleを追加**

`songdata_reader.go` のGetChartByMD5 SQL（行369-381）を更新:

```sql
SELECT md5, sha256, title, COALESCE(subtitle, ''), artist, COALESCE(subartist, ''),
    genre, mode, difficulty, level, minbpm, maxbpm, path
```

Scan部分にも`&c.Subtitle`を追加（`&c.Title`の後）。

**Step 5: app.goのListChartsでSubtitle/SubArtistをマッピング**

`app.go` のListCharts（行254-264）のDTO変換に追加:

```go
result[i] = dto.ChartListItemDTO{
	MD5:        c.MD5,
	Title:      c.Title,
	Subtitle:   c.Subtitle,   // ← 追加
	Artist:     c.Artist,
	SubArtist:  c.SubArtist,  // ← 追加
	Genre:      c.Genre,
	MinBPM:     c.MinBPM,
	MaxBPM:     c.MaxBPM,
	Difficulty: c.Difficulty,
	HasIRMeta:  c.HasIRMeta,
}
```

**Step 6: テスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/adapter/persistence/ -run TestListAllCharts -v`
Expected: PASS

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./... -v`
Expected: 全テストPASS

**Step 7: コミット**

```bash
git add internal/adapter/persistence/songdata_reader.go app.go
git commit -m "feat: SQL・変換にsubtitle/subartist取得を追加"
```

---

### Task 3: Wailsバインディング再生成

**Step 1: バインディング生成**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails generate module`
Expected: 成功

**Step 2: 生成確認**

`frontend/wailsjs/go/models.ts` にsubtitle/subArtistフィールドが含まれることを確認。

---

### Task 4: ChartListView.svelte — subtitle/subartist表示

**Files:**
- Modify: `frontend/src/ChartListView.svelte`

**Step 1: ROW_HEIGHTを48に変更**

行26の`const ROW_HEIGHT = 32`を`const ROW_HEIGHT = 48`に変更。

**Step 2: Titleカラムのcellレンダリングを2行化**

columnsの`title`カラム（行28）を更新。accessorKey方式からカスタムcell方式に変更:

```typescript
{
  id: 'title',
  header: 'Title',
  size: 300,
  accessorFn: (row) => row.title,
  cell: (info) => {
    const row = info.row.original
    return row.subtitle ? `${row.title}\n${row.subtitle}` : row.title
  },
},
```

ただし、Tanstack Tableのcellはプレーンテキストを返すため、HTML改行は使えない。代わりにテンプレート側で2行表示にする。

カラム定義はaccessorKeyのまま維持し、テンプレート側（行171-179付近）のセル描画部分でTitle/Artistカラムを特別処理する:

Titleセルとして、行171-179のセル描画部分を以下に変更:

```svelte
{#each row.getVisibleCells() as cell}
  <div
    class="px-2 truncate"
    style="width: {cell.column.getSize()}px; min-width: {cell.column.getSize()}px"
  >
    {#if cell.column.id === 'title'}
      <div class="truncate">{cell.row.original.title}</div>
      <div class="truncate text-[10px] text-base-content/50">{cell.row.original.subtitle || ''}</div>
    {:else if cell.column.id === 'artist'}
      <div class="truncate">{cell.row.original.artist}</div>
      <div class="truncate text-[10px] text-base-content/50">{cell.row.original.subArtist || ''}</div>
    {:else}
      <svelte:component
        this={flexRender(cell.column.columnDef.cell, cell.getContext())}
      />
    {/if}
  </div>
{/each}
```

**Step 3: コミット**

```bash
git add frontend/src/ChartListView.svelte
git commit -m "feat: ChartListViewにsubtitle/subartist表示を追加"
```

---

### Task 5: ChartDetail.svelte — subtitle/subartist表示

**Files:**
- Modify: `frontend/src/ChartDetail.svelte`

**Step 1: タイトル・アーティスト行にsubtitle/subartistを追加**

`ChartDetail.svelte` の譜面ヘッダー（行64-76）のタイトル表示部分を更新:

```svelte
<div class="flex-1 min-w-0">
  <h2 class="text-lg font-bold truncate">{chart?.title ?? ''}</h2>
  {#if chart?.subtitle}
    <p class="text-sm text-base-content/50">{chart.subtitle}</p>
  {/if}
  <p class="text-sm text-base-content/70">{chart?.artist ?? ''}</p>
  {#if chart?.subArtist}
    <p class="text-xs text-base-content/50">{chart.subArtist}</p>
  {/if}
</div>
```

**Step 2: コミット**

```bash
git add frontend/src/ChartDetail.svelte
git commit -m "feat: ChartDetailにsubtitle/subartist表示を追加"
```

---

### Task 6: SongDetail.svelte — 譜面リストにsubtitle表示

**Files:**
- Modify: `frontend/src/SongDetail.svelte`

**Step 1: 譜面リスト行にsubtitleを追加**

`SongDetail.svelte` の譜面一覧ループ（行116-125付近）、☆levelの後にsubtitleを追加:

```svelte
<span class="w-8">☆{chart.level}</span>
{#if chart.subtitle}
  <span class="text-base-content/50 truncate">{chart.subtitle}</span>
{/if}
```

**Step 2: コミット**

```bash
git add frontend/src/SongDetail.svelte
git commit -m "feat: SongDetailの譜面リストにsubtitle表示を追加"
```

---

### Task 7: EntryDetail.svelte — 導入済み譜面情報にsubtitle/subartist表示

**Files:**
- Modify: `frontend/src/EntryDetail.svelte`

**Step 1: 導入済み譜面情報セクションにsubtitle/subartistを追加**

EntryDetailの導入済み時に表示される譜面情報セクション（行107-132付近）に追加。ChartDTOのsubtitle/subArtistを表示する。具体的にはtitle表示の後にsubtitle、artist表示部分があればsubArtistを追加。

**Step 2: コミット**

```bash
git add frontend/src/EntryDetail.svelte
git commit -m "feat: EntryDetailにsubtitle/subartist表示を追加"
```

---

### Task 8: ビルド・動作確認

**Step 1: 全テスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./... -v`
Expected: 全テストPASS

**Step 2: ビルド**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: ビルド成功

**Step 3: 動作確認**

アプリを起動し以下を確認:
- 譜面一覧タブ: Title/Artistの下にsubtitle/subartist表示、行高さ48px
- 譜面詳細: subtitle/subartist表示
- 楽曲詳細: 譜面リストにsubtitle表示
- 難易度表: 導入済み譜面の詳細にsubtitle/subartist表示
