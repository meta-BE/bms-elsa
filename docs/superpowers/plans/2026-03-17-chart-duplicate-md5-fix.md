# 同一MD5譜面の重複表示バグ修正 実装計画

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 同一MD5が複数フォルダに存在する場合、譜面一覧の各行クリック時にそれぞれ正しいフォルダの詳細を表示する

**Architecture:** folderHashを選択キーに追加し、バックエンドのGetChartByMD5にfolderHash引数を追加。フロントエンドはmd5+folderHashの結合文字列で行を識別し、詳細取得時に両方を渡す。

**Tech Stack:** Go 1.24 / Wails v2 / Svelte 4 / TypeScript / SQLite

**Spec:** `docs/superpowers/specs/2026-03-17-chart-duplicate-md5-design.md`

---

## ファイルマップ

| ファイル | 操作 | 内容 |
|---------|------|------|
| `internal/adapter/persistence/songdata_reader.go` | Modify | ChartListItemにFolderHash追加、ListAllChartsのSELECT/Scan更新、GetChartByMD5にfolderHash引数追加 |
| `internal/app/dto/dto.go` | Modify | ChartListItemDTOにFolderHash追加 |
| `internal/app/chart_handler.go` | Modify | ListChartsのマッピング追加、GetChartDetailByMD5にfolderHash引数追加 |
| `frontend/src/views/ChartListView.svelte` | Modify | dispatch型変更、getKey/selected比較をmd5:folderHashに変更 |
| `frontend/src/views/ChartDetail.svelte` | Modify | folderHash prop追加、リアクティブ文修正 |
| `frontend/src/views/EntryDetail.svelte` | Modify | GetChartDetailByMD5に空文字第2引数追加 |
| `frontend/src/App.svelte` | Modify | selectedChart状態管理変更、prop渡し更新 |

---

### Task 1: バックエンド — songdata_reader.go

**Files:**
- Modify: `internal/adapter/persistence/songdata_reader.go:396-541`

- [ ] **Step 1: ChartListItemにFolderHashフィールドを追加**

`songdata_reader.go:396-411` の `ChartListItem` 構造体に追加:

```go
// ChartListItem は譜面一覧用の軽量モデル
type ChartListItem struct {
	MD5         string
	FolderHash  string
	Title       string
	Subtitle    string
	Artist      string
	SubArtist   string
	Genre       string
	Path        string
	MinBPM      float64
	MaxBPM      float64
	Difficulty  int
	Notes       int
	EventName   *string
	ReleaseYear *int
	HasIRMeta   bool
}
```

- [ ] **Step 2: ListAllChartsのSELECTとScanを更新**

`songdata_reader.go:415-466` の `ListAllCharts` メソッドを更新。SELECTに `s.folder` を追加し、Scanに `&c.FolderHash` を追加:

```go
func (r *SongdataReader) ListAllCharts(ctx context.Context) ([]ChartListItem, error) {
	query := `
		SELECT
			s.md5,
			s.folder,
			s.title,
			COALESCE(s.subtitle, ''),
			s.artist,
			COALESCE(s.subartist, ''),
			s.genre,
			s.path,
			s.minbpm,
			s.maxbpm,
			s.difficulty,
			s.notes,
			sm.event_name,
			sm.release_year,
			EXISTS(
				SELECT 1 FROM main.chart_meta cm
				WHERE cm.md5 = s.md5
			) AS has_ir_meta
		FROM songdata.song s
		LEFT JOIN main.song_meta sm ON sm.folder_hash = s.folder
		WHERE s.md5 != ''
		ORDER BY s.title ASC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("ListAllCharts: %w", err)
	}
	defer rows.Close()

	var charts []ChartListItem
	for rows.Next() {
		var c ChartListItem
		var eventName sql.NullString
		var releaseYear sql.NullInt64
		if err := rows.Scan(
			&c.MD5, &c.FolderHash, &c.Title, &c.Subtitle, &c.Artist, &c.SubArtist, &c.Genre, &c.Path,
			&c.MinBPM, &c.MaxBPM, &c.Difficulty, &c.Notes,
			&eventName, &releaseYear, &c.HasIRMeta,
		); err != nil {
			return nil, fmt.Errorf("ListAllCharts scan: %w", err)
		}
		if eventName.Valid {
			c.EventName = &eventName.String
		}
		if releaseYear.Valid {
			v := int(releaseYear.Int64)
			c.ReleaseYear = &v
		}
		charts = append(charts, c)
	}
	return charts, rows.Err()
}
```

- [ ] **Step 3: GetChartByMD5にfolderHash引数を追加**

`songdata_reader.go:505-541` の `GetChartByMD5` メソッドを更新。folderHashが空でない場合は `WHERE md5 = ? AND folder = ?` で検索、空の場合は既存の `WHERE md5 = ? LIMIT 1` を維持:

```go
// GetChartByMD5 はmd5（+任意のfolderHash）で譜面を1件取得し、IRメタ・難易度ラベルを付与して返す
func (r *SongdataReader) GetChartByMD5(ctx context.Context, md5, folderHash string) (*model.Chart, error) {
	var c model.Chart
	var query string
	var args []any
	if folderHash != "" {
		query = `
			SELECT md5, sha256, title, COALESCE(subtitle, ''), artist, COALESCE(subartist, ''),
				genre, mode, difficulty, level, minbpm, maxbpm, path, notes
			FROM songdata.song
			WHERE md5 = ? AND folder = ?`
		args = []any{md5, folderHash}
	} else {
		query = `
			SELECT md5, sha256, title, COALESCE(subtitle, ''), artist, COALESCE(subartist, ''),
				genre, mode, difficulty, level, minbpm, maxbpm, path, notes
			FROM songdata.song
			WHERE md5 = ?
			LIMIT 1`
		args = []any{md5}
	}
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&c.MD5, &c.SHA256, &c.Title, &c.Subtitle, &c.Artist, &c.SubArtist,
		&c.Genre, &c.Mode, &c.Difficulty, &c.Level,
		&c.MinBPM, &c.MaxBPM, &c.Path, &c.Notes,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("GetChartByMD5 query: %w", err)
	}

	// IRメタ付与
	irMeta, err := r.metaRepo.GetChartMeta(ctx, c.MD5)
	if err != nil {
		return nil, fmt.Errorf("GetChartByMD5 GetChartMeta: %w", err)
	}
	c.IRMeta = irMeta

	// 難易度ラベル付与
	labels, err := r.dtRepo.GetLabelsByMD5(ctx, c.MD5)
	if err != nil {
		return nil, fmt.Errorf("GetChartByMD5 GetLabelsByMD5: %w", err)
	}
	c.DifficultyLabels = labels

	return &c, nil
}
```

- [ ] **Step 4: GetChartByMD5の既存呼び出し元を更新**

`GetChartByMD5` のシグネチャが変わるため、songdata_reader.go内の他の呼び出し元がないか確認する（chart_handler.goの更新はTask 3で行う）。

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && grep -rn 'GetChartByMD5' --include='*.go' | grep -v '_test.go'`

該当箇所はすべて後続Taskで更新する。

- [ ] **Step 5: go buildで構文確認（エラー想定）**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`

Expected: コンパイルエラー（chart_handler.goの呼び出し元がまだ未更新のため）。エラー内容が `GetChartByMD5` の引数不足のみであることを確認。Task 2で修正する。

---

### Task 2: バックエンド — dto.go + chart_handler.go

**Files:**
- Modify: `internal/app/dto/dto.go:77-92`
- Modify: `internal/app/chart_handler.go:28-69`

- [ ] **Step 1: ChartListItemDTOにFolderHashフィールドを追加**

`dto.go:77-92` の `ChartListItemDTO` を更新:

```go
// ChartListItemDTO は譜面一覧用の軽量DTO
type ChartListItemDTO struct {
	MD5         string  `json:"md5"`
	FolderHash  string  `json:"folderHash"`
	Title       string  `json:"title"`
	Subtitle    string  `json:"subtitle,omitempty"`
	Artist      string  `json:"artist"`
	SubArtist   string  `json:"subArtist,omitempty"`
	Genre       string  `json:"genre"`
	Path        string  `json:"path"`
	MinBPM      float64 `json:"minBpm"`
	MaxBPM      float64 `json:"maxBpm"`
	Difficulty  int     `json:"difficulty"`
	Notes       int     `json:"notes"`
	EventName   string  `json:"eventName,omitempty"`
	ReleaseYear int     `json:"releaseYear,omitempty"`
	HasIRMeta   bool    `json:"hasIrMeta"`
}
```

- [ ] **Step 2: ChartHandler.ListChartsのマッピングを更新**

`chart_handler.go:34-47` の `ListCharts` メソッドで `FolderHash` のマッピングを追加:

```go
func (h *ChartHandler) ListCharts() ([]dto.ChartListItemDTO, error) {
	charts, err := h.songReader.ListAllCharts(h.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.ChartListItemDTO, len(charts))
	for i, c := range charts {
		result[i] = dto.ChartListItemDTO{
			MD5:        c.MD5,
			FolderHash: c.FolderHash,
			Title:      c.Title,
			Subtitle:   c.Subtitle,
			Artist:     c.Artist,
			SubArtist:  c.SubArtist,
			Genre:      c.Genre,
			Path:       c.Path,
			MinBPM:     c.MinBPM,
			MaxBPM:     c.MaxBPM,
			Difficulty: c.Difficulty,
			Notes:      c.Notes,
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
```

- [ ] **Step 3: ChartHandler.GetChartDetailByMD5にfolderHash引数を追加**

`chart_handler.go:59-69` を更新:

```go
func (h *ChartHandler) GetChartDetailByMD5(md5, folderHash string) (*dto.ChartDTO, error) {
	chart, err := h.songReader.GetChartByMD5(h.ctx, md5, folderHash)
	if err != nil {
		return nil, err
	}
	if chart == nil {
		return nil, nil
	}
	result := dto.ChartToDTO(*chart)
	return &result, nil
}
```

- [ ] **Step 4: go buildで確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`

Expected: PASS（バックエンドのコンパイルが通る）

- [ ] **Step 5: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add internal/adapter/persistence/songdata_reader.go internal/app/dto/dto.go internal/app/chart_handler.go
git commit -m "fix: GetChartByMD5にfolderHash引数を追加し、同一MD5の異なるフォルダを区別可能にする"
```

- [ ] **Step 6: Wailsバインディング再生成**

Goのシグネチャ変更に伴い、フロントエンド用のTypeScriptバインディングを再生成する。これによりTask 3以降のフロントエンド作業でTypeScriptの型が正しくなる。

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails generate module`

Expected: `frontend/wailsjs/go/app/ChartHandler.js`, `ChartHandler.d.ts`, `models.ts` が更新される。`GetChartDetailByMD5` の引数が `(arg1:string, arg2:string)` に変わっていることを確認。

---

### Task 3: フロントエンド — ChartListView.svelte

**Files:**
- Modify: `frontend/src/views/ChartListView.svelte:28-31, 75, 218-235, 307`

- [ ] **Step 1: dispatch型を更新**

`ChartListView.svelte:28-31` を変更:

```typescript
const dispatch = createEventDispatcher<{
  select: { md5: string; folderHash: string }
  deselect: void
}>()
```

- [ ] **Step 2: handleRowClickを更新**

`ChartListView.svelte:229-235` を変更:

```typescript
function handleRowClick(chart: dto.ChartListItemDTO) {
  const key = chart.md5 + ':' + chart.folderHash
  if (selected === key) {
    dispatch('deselect')
  } else {
    dispatch('select', { md5: chart.md5, folderHash: chart.folderHash })
  }
}
```

- [ ] **Step 3: handleKeyNavを更新**

`ChartListView.svelte:218-227` を変更:

```typescript
function handleKeyNav(e: KeyboardEvent) {
  if (!active) return
  handleArrowNav(e, {
    selected,
    items: rows.map(r => r.original),
    getKey: (o: dto.ChartListItemDTO) => o.md5 + ':' + o.folderHash,
    onSelect: (o: dto.ChartListItemDTO) => dispatch('select', { md5: o.md5, folderHash: o.folderHash }),
    scrollToIndex: (i: number) => $virtualizer.scrollToIndex(i, { align: 'auto' }),
  })
}
```

- [ ] **Step 4: 行ハイライト比較を更新**

`ChartListView.svelte:307` の行ハイライト条件を変更:

```svelte
{selected === row.original.md5 + ':' + row.original.folderHash ? 'bg-primary/20' : 'hover:bg-base-200'}
```

（変更前: `selected === row.original.md5`）

---

### Task 4: フロントエンド — ChartDetail.svelte + EntryDetail.svelte

**Files:**
- Modify: `frontend/src/views/ChartDetail.svelte:13-31`
- Modify: `frontend/src/views/EntryDetail.svelte:35`

- [ ] **Step 1: ChartDetail.svelteにfolderHash propを追加しリアクティブ文を修正**

`ChartDetail.svelte:13-31` を変更:

```typescript
export let md5: string
export let folderHash: string = ''

let chart: dto.ChartDTO | null = null
let loading = false

$: chartKey = md5 + ':' + folderHash
$: if (chartKey) loadChart(md5, folderHash)

async function loadChart(hash: string, folder: string) {
  loading = true
  chart = null
  try {
    chart = await GetChartDetailByMD5(hash, folder)
  } catch (e) {
    console.error('Failed to load chart detail:', e)
    chart = null
  } finally {
    loading = false
  }
}
```

`saveWorkingUrls` と `lookupIR` 内の `loadChart(md5)` も `loadChart(md5, folderHash)` に変更:

```typescript
async function lookupIR() {
  if (!chart) return
  await LookupByMD5(chart.md5, chart.sha256)
  await loadChart(md5, folderHash)
}

async function saveWorkingUrls(e: CustomEvent<{ bodyUrl: string; diffUrl: string }>) {
  if (!chart) return
  await UpdateChartMeta(chart.md5, e.detail.bodyUrl, e.detail.diffUrl)
  await loadChart(md5, folderHash)
}
```

- [ ] **Step 2: EntryDetail.svelteのGetChartDetailByMD5呼び出しを更新**

`EntryDetail.svelte:35` を変更:

```typescript
chart = await GetChartDetailByMD5(hash, '')
```

（変更前: `chart = await GetChartDetailByMD5(hash)`）

---

### Task 5: フロントエンド — App.svelte

**Files:**
- Modify: `frontend/src/App.svelte:33, 59-60, 70-71, 78-89, 216-224`

- [ ] **Step 1: 選択状態の型を変更**

`App.svelte:33` を変更:

```typescript
// 譜面タブの選択状態
let selectedChart: { md5: string; folderHash: string } | null = null
```

（変更前: `let selectedChartMD5: string | null = null`）

- [ ] **Step 2: handleDeselectとhandleCloseを更新**

`App.svelte:59-60` と `70-71` の `selectedChartMD5 = null` を `selectedChart = null` に変更。

- [ ] **Step 3: handleChartSelectを更新**

`App.svelte:78-85` を変更:

```typescript
function handleChartSelect(e: CustomEvent<{ md5: string; folderHash: string }>) {
  if (selectedChart?.md5 === e.detail.md5 && selectedChart?.folderHash === e.detail.folderHash) {
    selectedChart = null
  } else {
    selectedChart = { md5: e.detail.md5, folderHash: e.detail.folderHash }
  }
}
```

- [ ] **Step 4: handleChartDeselectを更新**

`App.svelte:87-89` を変更:

```typescript
function handleChartDeselect() {
  selectedChart = null
}
```

- [ ] **Step 5: テンプレートの譜面一覧タブセクションを更新**

`App.svelte:215-224` を変更:

```svelte
<!-- 譜面一覧タブ -->
<div class="h-full" class:hidden={activeTab !== 'charts'}>
  <SplitPane showDetail={!!selectedChart} bind:splitRatio>
    <ChartListView slot="list" selected={selectedChart ? selectedChart.md5 + ':' + selectedChart.folderHash : null} active={activeTab === 'charts'} on:select={handleChartSelect} on:deselect={handleChartDeselect} />
    <svelte:fragment slot="detail">
      {#if selectedChart}
        <ChartDetail md5={selectedChart.md5} folderHash={selectedChart.folderHash} on:close={() => { selectedChart = null }} />
      {/if}
    </svelte:fragment>
  </SplitPane>
</div>
```

---

### Task 6: ビルド確認とコミット

- [ ] **Step 1: Wailsバインディング再生成を含むフロントエンドビルド**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run build`

Expected: PASS

- [ ] **Step 2: go buildで全体確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build .`

Expected: PASS

- [ ] **Step 3: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add frontend/src/views/ChartListView.svelte frontend/src/views/ChartDetail.svelte frontend/src/views/EntryDetail.svelte frontend/src/App.svelte
git commit -m "fix: 同一MD5の異なるフォルダの譜面をそれぞれ正しく詳細表示する"
```

- [ ] **Step 4: 手動テスト（wails devで動作確認）**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails dev`

確認項目:
1. 譜面一覧で「#B2FFFF」を検索 → 2行表示される（EVENT/YEARが異なる）
2. 各行をクリック → それぞれ異なるパスの詳細が表示される
3. 難易度表タブでエントリをクリック → 既存通り詳細が表示される
4. 通常の譜面（重複なし）の選択・詳細表示が正常に動作する
5. 同一MD5の2行それぞれの選択・選択解除（トグル）が独立して動作する
