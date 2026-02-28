# 譜面一覧ビュー Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** songdata.dbの全譜面を個別表示する「譜面一覧」タブを追加する

**Architecture:** SongTableのカラム構成（8列）をDifficultyTableViewの構造（Tanstack + virtualizer）に適用した新コンポーネントChartListViewを作成。バックエンドにListAllChartsメソッドを追加し、songdata.songの全レコードを個別取得。既存ChartDetailを純粋な譜面情報表示に分割し、難易度表用はEntryDetail（ChartDetailを内包）として再構成。

**Tech Stack:** Go + SQLite (songdata.db ATTACH) / Svelte + Tanstack Table + Tanstack Virtual + DaisyUI

---

### Task 1: バックエンド — ChartListItemDTO の定義

**Files:**
- Modify: `internal/app/dto/dto.go`

**Step 1: ChartListItemDTO を追加**

`dto.go` の既存 ChartDTO の後に以下を追加:

```go
// ChartListItemDTO は譜面一覧用の軽量DTO
type ChartListItemDTO struct {
	MD5         string  `json:"md5"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Genre       string  `json:"genre"`
	MinBPM      float64 `json:"minBpm"`
	MaxBPM      float64 `json:"maxBpm"`
	Difficulty  int     `json:"difficulty"`
	EventName   string  `json:"eventName,omitempty"`
	ReleaseYear int     `json:"releaseYear,omitempty"`
	HasIRMeta   bool    `json:"hasIrMeta"`
}
```

**Step 2: コンパイル確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: 成功

**Step 3: コミット**

```bash
git add internal/app/dto/dto.go
git commit -m "feat: ChartListItemDTO を追加"
```

---

### Task 2: バックエンド — ListAllCharts メソッドの実装（TDD）

**Files:**
- Modify: `internal/adapter/persistence/songdata_reader.go`
- Modify: `internal/adapter/persistence/songdata_reader_test.go`

**Step 1: テストを書く**

`songdata_reader_test.go` の末尾に以下を追加:

```go
func TestListAllCharts(t *testing.T) {
	reader, _ := setupSongdataReader(t)
	ctx := context.Background()

	charts, err := reader.ListAllCharts(ctx)
	if err != nil {
		t.Fatalf("ListAllCharts failed: %v", err)
	}

	// testdata/songdata.db に譜面が存在すること
	if len(charts) == 0 {
		t.Fatal("expected charts, got 0")
	}

	// 各譜面にMD5とTitleがあること
	first := charts[0]
	if first.MD5 == "" {
		t.Error("expected MD5 to be non-empty")
	}
	if first.Title == "" {
		t.Error("expected Title to be non-empty")
	}
}
```

**Step 2: テストが失敗することを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/adapter/persistence/ -run TestListAllCharts -v`
Expected: コンパイルエラー（ListAllChartsが存在しない）

**Step 3: ListAllCharts を実装**

`songdata_reader.go` に以下を追加（GetChartByMD5メソッドの前あたり）:

```go
// ChartListItem は譜面一覧用の軽量モデル
type ChartListItem struct {
	MD5         string
	Title       string
	Artist      string
	Genre       string
	MinBPM      float64
	MaxBPM      float64
	Difficulty  int
	EventName   *string
	ReleaseYear *int
	HasIRMeta   bool
}

// ListAllCharts はsongdata.dbの全譜面を個別に取得する
func (r *SongdataReader) ListAllCharts(ctx context.Context) ([]ChartListItem, error) {
	query := `
		SELECT
			s.md5,
			s.title,
			s.artist,
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
			&c.MD5, &c.Title, &c.Artist, &c.Genre,
			&c.MinBPM, &c.MaxBPM, &c.Difficulty,
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

**Step 4: テストが通ることを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/adapter/persistence/ -run TestListAllCharts -v`
Expected: PASS

**Step 5: 全テスト回帰確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./...`
Expected: 全PASS

**Step 6: コミット**

```bash
git add internal/adapter/persistence/songdata_reader.go internal/adapter/persistence/songdata_reader_test.go
git commit -m "feat: ListAllCharts メソッドを追加（TDD）"
```

---

### Task 3: バックエンド — app.go に ListCharts バインディング追加

**Files:**
- Modify: `app.go`

**Step 1: ListCharts メソッドを追加**

`app.go` の `GetChartDetailByMD5` メソッドの前あたりに以下を追加:

```go
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
			Artist:     c.Artist,
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
```

**Step 2: コンパイル確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: 成功

**Step 3: コミット**

```bash
git add app.go
git commit -m "feat: ListCharts Wailsバインディングを追加"
```

---

### Task 4: Wails バインディング再生成

**Step 1: バインディング生成**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails generate module`
Expected: 成功（frontend/wailsjs/ 以下にTypeScript型定義が更新される）

**注意:** frontend/wailsjs/ は .gitignore で無視されているためコミット不要。`wails build` 時にも自動生成される。

---

### Task 5: フロントエンド — ChartDetail を純粋な譜面詳細に、EntryDetail を新規作成

既存の `ChartDetail.svelte` は難易度表エントリ情報（entryData）と譜面情報（chart）が混在している。
これを分離して:
- **ChartDetail**: 純粋な譜面情報のみ（md5で取得、ヘッダー + メタデータ + IR情報）
- **EntryDetail**: 難易度表エントリ固有の表示 + ChartDetail を子コンポーネントとして内包

**Files:**
- Modify: `frontend/src/ChartDetail.svelte`
- Create: `frontend/src/EntryDetail.svelte`

**Step 1: ChartDetail.svelte を譜面専用に書き換え**

entryData依存を除去し、chart情報のみで表示するように変更:

```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { GetChartDetailByMD5 } from '../wailsjs/go/main/App'
  import { LookupByMD5, UpdateChartMeta } from '../wailsjs/go/app/IRHandler'
  import type { dto } from '../wailsjs/go/models'

  const dispatch = createEventDispatcher<{ close: void }>()

  export let md5: string

  let chart: dto.ChartDTO | null = null
  let loading = false
  let editWorkingBodyUrl = ''
  let editWorkingDiffUrl = ''

  $: if (md5) loadChart(md5)

  async function loadChart(hash: string) {
    loading = true
    chart = null
    try {
      chart = await GetChartDetailByMD5(hash)
      if (chart) {
        editWorkingBodyUrl = chart.workingBodyUrl || ''
        editWorkingDiffUrl = chart.workingDiffUrl || ''
      }
    } catch (e) {
      console.error('Failed to load chart detail:', e)
      chart = null
    } finally {
      loading = false
    }
  }

  async function lookupIR() {
    if (!chart) return
    await LookupByMD5(chart.md5, chart.sha256)
    await loadChart(md5)
  }

  async function saveWorkingUrls() {
    if (!chart) return
    await UpdateChartMeta(chart.md5, chart.sha256, editWorkingBodyUrl, editWorkingDiffUrl)
    await loadChart(md5)
  }

  function modeLabel(mode: number): string {
    const labels: Record<number, string> = { 5: '5K', 7: '7K', 9: 'PMS', 10: '10K', 14: '14K', 25: '24K' }
    return labels[mode] || `${mode}K`
  }

  function diffLabel(diff: number): string {
    const labels = ['', 'BEG', 'NOR', 'HYP', 'ANO', 'INS']
    return labels[diff] || ''
  }
</script>

{#if loading}
  <div class="flex items-center justify-center h-full">
    <span class="loading loading-spinner"></span>
  </div>
{:else}
  <div class="flex flex-col gap-3">
    <!-- 譜面ヘッダー -->
    <div class="bg-base-200 rounded-lg p-3">
      <div class="flex justify-between items-start">
        <div class="flex-1 min-w-0">
          <h2 class="text-lg font-bold truncate">{chart?.title ?? ''}</h2>
          <p class="text-sm text-base-content/70">{chart?.artist ?? ''}</p>
        </div>
        <button
          class="btn btn-ghost btn-xs shrink-0 ml-2"
          on:click={() => dispatch('close')}
        >✕</button>
      </div>
    </div>

    <!-- 譜面メタデータ -->
    {#if chart}
      <div class="bg-base-200 rounded-lg p-3">
        <h3 class="text-sm font-semibold mb-2">譜面情報</h3>
        <div class="text-xs space-y-1">
          <div class="flex items-center gap-4">
            <span><span class="font-semibold">Mode:</span> {modeLabel(chart.mode)}</span>
            <span><span class="font-semibold">Difficulty:</span> {diffLabel(chart.difficulty)}</span>
            <span><span class="font-semibold">Level:</span> ☆{chart.level}</span>
          </div>
          <p>
            <span class="font-semibold">BPM:</span>
            {#if chart.minBpm === chart.maxBpm}
              {Math.round(chart.minBpm)}
            {:else}
              {Math.round(chart.minBpm)}-{Math.round(chart.maxBpm)}
            {/if}
          </p>
          {#if chart.difficultyLabels?.length}
            <div class="flex items-center gap-1 flex-wrap">
              <span class="font-semibold">難易度表:</span>
              {#each chart.difficultyLabels as label}
                <span class="badge badge-sm badge-outline" title={label.tableName}>{label.symbol}{label.level}</span>
              {/each}
            </div>
          {/if}
        </div>
      </div>

      <!-- IR情報 -->
      <div class="bg-base-200 rounded-lg p-3">
        <div class="flex items-center justify-between mb-2">
          <h3 class="text-sm font-semibold">LR2IR情報</h3>
          <button class="btn btn-ghost btn-xs" on:click={lookupIR}>IR取得</button>
        </div>
        {#if chart.hasIrMeta}
          <div class="text-xs space-y-1">
            {#if chart.lr2irTags}
              <p><span class="font-semibold">タグ:</span> {chart.lr2irTags}</p>
            {/if}
            {#if chart.lr2irBodyUrl}
              <p>
                <span class="font-semibold">本体URL:</span>
                <a href={chart.lr2irBodyUrl} target="_blank" rel="noopener noreferrer" class="link link-primary">{chart.lr2irBodyUrl}</a>
              </p>
            {/if}
            {#if chart.lr2irDiffUrl}
              <p>
                <span class="font-semibold">差分URL:</span>
                <a href={chart.lr2irDiffUrl} target="_blank" rel="noopener noreferrer" class="link link-primary">{chart.lr2irDiffUrl}</a>
              </p>
            {/if}
            {#if chart.lr2irNotes}
              <p><span class="font-semibold">備考:</span> {chart.lr2irNotes}</p>
            {/if}
            <div class="divider my-1"></div>
            <div class="flex gap-2 items-center">
              <label class="font-semibold" for="chart-working-body-url">動作URL(本体):</label>
              <input id="chart-working-body-url" class="input input-xs input-bordered flex-1" bind:value={editWorkingBodyUrl} on:blur={saveWorkingUrls} />
            </div>
            <div class="flex gap-2 items-center">
              <label class="font-semibold" for="chart-working-diff-url">動作URL(差分):</label>
              <input id="chart-working-diff-url" class="input input-xs input-bordered flex-1" bind:value={editWorkingDiffUrl} on:blur={saveWorkingUrls} />
            </div>
          </div>
        {:else}
          <p class="text-xs text-base-content/50">IR情報がありません。「IR取得」ボタンで取得してください。</p>
        {/if}
      </div>
    {/if}
  </div>
{/if}
```

**Step 2: EntryDetail.svelte を新規作成**

難易度表エントリ固有の情報を表示し、ChartDetailを内包する:

```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { GetChartDetailByMD5 } from '../wailsjs/go/main/App'
  import { LookupByMD5, UpdateChartMeta } from '../wailsjs/go/app/IRHandler'
  import type { dto, main } from '../wailsjs/go/models'

  const dispatch = createEventDispatcher<{ close: void }>()

  export let md5: string
  export let entryData: main.DifficultyTableEntryDTO

  let chart: dto.ChartDTO | null = null
  let loading = false
  let editWorkingBodyUrl = ''
  let editWorkingDiffUrl = ''

  $: if (md5) loadChart(md5)

  async function loadChart(hash: string) {
    loading = true
    chart = null
    try {
      chart = await GetChartDetailByMD5(hash)
      if (chart) {
        editWorkingBodyUrl = chart.workingBodyUrl || ''
        editWorkingDiffUrl = chart.workingDiffUrl || ''
      }
    } catch (e) {
      console.error('Failed to load chart detail:', e)
      chart = null
    } finally {
      loading = false
    }
  }

  async function lookupIR() {
    if (!chart) return
    await LookupByMD5(chart.md5, chart.sha256)
    await loadChart(md5)
  }

  async function saveWorkingUrls() {
    if (!chart) return
    await UpdateChartMeta(chart.md5, chart.sha256, editWorkingBodyUrl, editWorkingDiffUrl)
    await loadChart(md5)
  }

  function modeLabel(mode: number): string {
    const labels: Record<number, string> = { 5: '5K', 7: '7K', 9: 'PMS', 10: '10K', 14: '14K', 25: '24K' }
    return labels[mode] || `${mode}K`
  }

  function diffLabel(diff: number): string {
    const labels = ['', 'BEG', 'NOR', 'HYP', 'ANO', 'INS']
    return labels[diff] || ''
  }
</script>

{#if loading}
  <div class="flex items-center justify-center h-full">
    <span class="loading loading-spinner"></span>
  </div>
{:else}
  <div class="flex flex-col gap-3">
    <!-- エントリ基本情報 -->
    <div class="bg-base-200 rounded-lg p-3">
      <div class="flex justify-between items-start">
        <div class="flex-1 min-w-0">
          <h2 class="text-lg font-bold truncate">{entryData.title}</h2>
          <p class="text-sm text-base-content/70">{entryData.artist}</p>
          <div class="flex items-center gap-2 mt-1">
            <span class="badge badge-sm">Lv. {entryData.level}</span>
            {#if !chart}
              <span class="badge badge-sm badge-warning">未導入</span>
            {:else if entryData.status === 'duplicate'}
              <span class="badge badge-sm badge-warning">重複</span>
            {:else}
              <span class="badge badge-sm badge-success">導入済</span>
            {/if}
          </div>
        </div>
        <button
          class="btn btn-ghost btn-xs shrink-0 ml-2"
          on:click={() => dispatch('close')}
        >✕</button>
      </div>
      {#if entryData.url || entryData.urlDiff}
        <div class="divider my-1"></div>
        <div class="text-xs space-y-1">
          {#if entryData.url}
            <p>
              <span class="font-semibold">URL:</span>
              <a href={entryData.url} target="_blank" rel="noopener noreferrer" class="link link-primary">{entryData.url}</a>
            </p>
          {/if}
          {#if entryData.urlDiff}
            <p>
              <span class="font-semibold">差分URL:</span>
              <a href={entryData.urlDiff} target="_blank" rel="noopener noreferrer" class="link link-primary">{entryData.urlDiff}</a>
            </p>
          {/if}
        </div>
      {/if}
    </div>

    <!-- 譜面メタデータ（導入済の場合のみ） -->
    {#if chart}
      <div class="bg-base-200 rounded-lg p-3">
        <h3 class="text-sm font-semibold mb-2">譜面情報</h3>
        <div class="text-xs space-y-1">
          <div class="flex items-center gap-4">
            <span><span class="font-semibold">Mode:</span> {modeLabel(chart.mode)}</span>
            <span><span class="font-semibold">Difficulty:</span> {diffLabel(chart.difficulty)}</span>
            <span><span class="font-semibold">Level:</span> ☆{chart.level}</span>
          </div>
          <p>
            <span class="font-semibold">BPM:</span>
            {#if chart.minBpm === chart.maxBpm}
              {Math.round(chart.minBpm)}
            {:else}
              {Math.round(chart.minBpm)}-{Math.round(chart.maxBpm)}
            {/if}
          </p>
          {#if chart.difficultyLabels?.length}
            <div class="flex items-center gap-1 flex-wrap">
              <span class="font-semibold">難易度表:</span>
              {#each chart.difficultyLabels as label}
                <span class="badge badge-sm badge-outline" title={label.tableName}>{label.symbol}{label.level}</span>
              {/each}
            </div>
          {/if}
        </div>
      </div>

      <!-- IR情報 -->
      <div class="bg-base-200 rounded-lg p-3">
        <div class="flex items-center justify-between mb-2">
          <h3 class="text-sm font-semibold">LR2IR情報</h3>
          <button class="btn btn-ghost btn-xs" on:click={lookupIR}>IR取得</button>
        </div>
        {#if chart.hasIrMeta}
          <div class="text-xs space-y-1">
            {#if chart.lr2irTags}
              <p><span class="font-semibold">タグ:</span> {chart.lr2irTags}</p>
            {/if}
            {#if chart.lr2irBodyUrl}
              <p>
                <span class="font-semibold">本体URL:</span>
                <a href={chart.lr2irBodyUrl} target="_blank" rel="noopener noreferrer" class="link link-primary">{chart.lr2irBodyUrl}</a>
              </p>
            {/if}
            {#if chart.lr2irDiffUrl}
              <p>
                <span class="font-semibold">差分URL:</span>
                <a href={chart.lr2irDiffUrl} target="_blank" rel="noopener noreferrer" class="link link-primary">{chart.lr2irDiffUrl}</a>
              </p>
            {/if}
            {#if chart.lr2irNotes}
              <p><span class="font-semibold">備考:</span> {chart.lr2irNotes}</p>
            {/if}
            <div class="divider my-1"></div>
            <div class="flex gap-2 items-center">
              <label class="font-semibold" for="entry-working-body-url">動作URL(本体):</label>
              <input id="entry-working-body-url" class="input input-xs input-bordered flex-1" bind:value={editWorkingBodyUrl} on:blur={saveWorkingUrls} />
            </div>
            <div class="flex gap-2 items-center">
              <label class="font-semibold" for="entry-working-diff-url">動作URL(差分):</label>
              <input id="entry-working-diff-url" class="input input-xs input-bordered flex-1" bind:value={editWorkingDiffUrl} on:blur={saveWorkingUrls} />
            </div>
          </div>
        {:else}
          <p class="text-xs text-base-content/50">IR情報がありません。「IR取得」ボタンで取得してください。</p>
        {/if}
      </div>
    {/if}
  </div>
{/if}
```

**注意:** EntryDetail は ChartDetail をコンポーネントとして内包するのではなく、同じ譜面メタデータ + IR セクションのコードを持つ自己完結型コンポーネント。理由: chart の読み込み状態管理と閉じるボタンの制御がエントリヘッダーと密結合しているため、子コンポーネント化よりコード重複の方がシンプル。将来的にメタデータ・IRセクションを共通コンポーネントに抽出することは可能。

**Step 3: コミット**

```bash
git add frontend/src/ChartDetail.svelte frontend/src/EntryDetail.svelte
git commit -m "refactor: ChartDetailを譜面専用に、EntryDetailを難易度表用に分離"
```

---

### Task 6: フロントエンド — ChartListView.svelte の作成

**Files:**
- Create: `frontend/src/ChartListView.svelte`

**Step 1: DifficultyTableView.svelte をベースにChartListView.svelteを作成**

DifficultyTableViewの構造をベースに、以下を変更:
- ドロップダウン削除（全譜面表示のため不要）
- カラムをSongTableベースの8列に変更
- データ取得を `ListCharts()` に変更
- イベント発火を md5 のみに変更（entryDataなし）

```svelte
<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte'
  import {
    createSvelteTable,
    getCoreRowModel,
    getSortedRowModel,
    flexRender,
    type ColumnDef,
    type SortingState,
  } from '@tanstack/svelte-table'
  import { createVirtualizer } from '@tanstack/svelte-virtual'
  import { ListCharts } from '../wailsjs/go/main/App'
  import type { dto } from '../wailsjs/go/models'

  const dispatch = createEventDispatcher<{
    select: { md5: string }
    deselect: void
  }>()

  let charts: dto.ChartListItemDTO[] = []
  let loading = false
  let selectedMD5: string | null = null
  let scrollElement: HTMLDivElement
  let sorting: SortingState = []

  const ROW_HEIGHT = 32
  const columns: ColumnDef<dto.ChartListItemDTO>[] = [
    { accessorKey: 'title', header: 'Title', size: 300 },
    { accessorKey: 'artist', header: 'Artist', size: 200 },
    { accessorKey: 'genre', header: 'Genre', size: 140 },
    {
      id: 'bpm',
      header: 'BPM',
      size: 100,
      accessorFn: (row) => row.minBpm,
      cell: (info) => {
        const row = info.row.original
        if (row.minBpm === row.maxBpm) return String(Math.round(row.minBpm))
        return `${Math.round(row.minBpm)}-${Math.round(row.maxBpm)}`
      },
    },
    {
      accessorKey: 'difficulty',
      header: '★',
      size: 80,
    },
    {
      id: 'eventName',
      header: 'Event',
      size: 140,
      accessorFn: (row) => row.eventName || '',
    },
    {
      id: 'releaseYear',
      header: 'Year',
      size: 60,
      accessorFn: (row) => row.releaseYear || '',
    },
    {
      id: 'ir',
      header: 'IR',
      size: 40,
      accessorFn: (row) => row.hasIrMeta ? '●' : '',
    },
  ]

  $: table = createSvelteTable({
    data: charts,
    columns,
    state: { sorting },
    onSortingChange: (updater) => {
      sorting = typeof updater === 'function' ? updater(sorting) : updater
    },
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  })

  $: rows = $table.getRowModel().rows

  $: virtualizer = createVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: rows.length,
    getScrollElement: () => scrollElement,
    estimateSize: () => ROW_HEIGHT,
    overscan: 20,
  })

  $: virtualItems = $virtualizer.getVirtualItems()
  $: totalSize = $virtualizer.getTotalSize()

  onMount(async () => {
    loading = true
    try {
      charts = await ListCharts() || []
    } catch (e) {
      console.error('Failed to load charts:', e)
    } finally {
      loading = false
    }
  })

  function handleRowClick(chart: dto.ChartListItemDTO) {
    if (selectedMD5 === chart.md5) {
      selectedMD5 = null
      dispatch('deselect')
    } else {
      selectedMD5 = chart.md5
      dispatch('select', { md5: chart.md5 })
    }
  }
</script>

<div class="flex flex-col h-full">
  <!-- ヘッダー -->
  <div class="flex items-center gap-2 px-4 py-2 bg-base-100 shrink-0">
    <span class="text-sm text-base-content/70">
      {charts.length} 譜面
    </span>
  </div>

  {#if loading}
    <div class="flex items-center justify-center flex-1">
      <span class="loading loading-spinner"></span>
    </div>
  {:else}
    <!-- テーブルヘッダー -->
    <div class="shrink-0">
      <table class="table table-xs w-full">
        <thead>
          {#each $table.getHeaderGroups() as headerGroup}
            <tr>
              {#each headerGroup.headers as header}
                <th
                  style="width: {header.getSize()}px"
                  class="cursor-pointer select-none hover:bg-base-200"
                  on:click={header.column.getToggleSortingHandler()}
                >
                  <div class="flex items-center gap-1">
                    <svelte:component
                      this={flexRender(header.column.columnDef.header, header.getContext())}
                    />
                    {#if header.column.getIsSorted() === 'asc'}
                      <span class="text-xs">▲</span>
                    {:else if header.column.getIsSorted() === 'desc'}
                      <span class="text-xs">▼</span>
                    {/if}
                  </div>
                </th>
              {/each}
            </tr>
          {/each}
        </thead>
      </table>
    </div>

    <!-- 仮想スクロール本体 -->
    <div class="flex-1 overflow-auto" bind:this={scrollElement}>
      <div style="height: {totalSize}px; width: 100%; position: relative;">
        {#each virtualItems as virtualRow (virtualRow.index)}
          {@const row = rows[virtualRow.index]}
          <div
            class="absolute w-full flex items-center text-xs cursor-pointer transition-colors
              {selectedMD5 === row.original.md5 ? 'bg-primary/20' : 'hover:bg-base-200'}"
            style="height: {ROW_HEIGHT}px; transform: translateY({virtualRow.start}px);"
            on:click={() => handleRowClick(row.original)}
            role="row"
            tabindex="0"
          >
            {#each row.getVisibleCells() as cell}
              <div
                class="truncate px-2"
                style="width: {cell.column.getSize()}px"
              >
                <svelte:component
                  this={flexRender(cell.column.columnDef.cell, cell.getContext())}
                />
              </div>
            {/each}
          </div>
        {/each}
      </div>
    </div>
  {/if}
</div>
```

**Step 2: コミット**

```bash
git add frontend/src/ChartListView.svelte
git commit -m "feat: ChartListView コンポーネントを追加"
```

---

### Task 7: フロントエンド — App.svelte にタブ追加

**Files:**
- Modify: `frontend/src/App.svelte`

**Step 1: import を更新**

既存の `ChartDetail` の import を `EntryDetail` に変更し、`ChartListView` と `ChartDetail` を追加:

```typescript
import ChartDetail from './ChartDetail.svelte'
import ChartListView from './ChartListView.svelte'
import EntryDetail from './EntryDetail.svelte'
```

既存の `import ChartDetail from './ChartDetail.svelte'` は残す（譜面一覧タブで使用）。

**Step 2: activeTab の型を拡張**

```typescript
let activeTab: 'songs' | 'charts' | 'difficulty' = 'songs'
```

**Step 3: 譜面タブ用の選択状態を追加**

```typescript
let selectedChartMD5: string | null = null
```

**Step 4: イベントハンドラを追加**

```typescript
function handleChartSelect(e: CustomEvent<{ md5: string }>) {
  if (selectedChartMD5 === e.detail.md5) {
    selectedChartMD5 = null
  } else {
    selectedChartMD5 = e.detail.md5
  }
}

function handleChartDeselect() {
  selectedChartMD5 = null
}
```

**Step 5: タブバーに「譜面一覧」ボタンを追加**

「楽曲一覧」と「難易度表」の間に追加:

```svelte
<button
  class="tab"
  class:tab-active={activeTab === 'charts'}
  on:click={() => switchTab('charts')}
>譜面一覧</button>
```

**Step 6: タブコンテンツに譜面一覧セクションを追加**

楽曲一覧セクションと難易度表セクションの間に追加。レイアウト構造は難易度表タブと対称:

```svelte
<!-- 譜面一覧タブ -->
<div class="overflow-hidden" class:hidden={activeTab !== 'charts'} style="flex: {selectedChartMD5 ? splitRatio : 1}">
  <ChartListView on:select={handleChartSelect} on:deselect={handleChartDeselect} />
</div>

{#if selectedChartMD5}
  <div
    class="h-1 shrink-0 cursor-row-resize bg-base-300 hover:bg-primary/30 transition-colors my-1 rounded"
    class:hidden={activeTab !== 'charts'}
    on:mousedown={onDragStart}
    role="separator"
    tabindex="0"
  ></div>
  <div class="overflow-y-auto" class:hidden={activeTab !== 'charts'} style="flex: {1 - splitRatio}" on:click|stopPropagation>
    <ChartDetail md5={selectedChartMD5} on:close={() => { selectedChartMD5 = null }} />
  </div>
{/if}
```

**Step 7: 難易度表タブの詳細ペインを EntryDetail に変更**

難易度表タブのテンプレートで `<ChartDetail>` を `<EntryDetail>` に変更:

```svelte
<!-- 変更前 -->
<ChartDetail md5={selectedEntryMD5} entryData={selectedEntryData} on:close={handleClose} />

<!-- 変更後 -->
<EntryDetail md5={selectedEntryMD5} entryData={selectedEntryData} on:close={handleClose} />
```

**Step 8: コミット**

```bash
git add frontend/src/App.svelte
git commit -m "feat: App.svelte に譜面一覧タブを追加"
```

---

### Task 8: ビルド・動作確認

**Step 1: Wailsビルド**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: ビルド成功

**Step 2: 動作確認**

1. アプリを起動
2. 「譜面一覧」タブに切り替え
3. 全譜面が一覧表示されることを確認
4. ヘッダークリックでソートが機能することを確認
5. 行クリックでChartDetailが表示されることを確認
6. 同じ行再クリックでChartDetailが閉じることを確認
7. タブ切り替えで状態が保持されることを確認

**Step 3: 全テスト回帰確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./...`
Expected: 全PASS
