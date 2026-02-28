# 難易度表譜面一覧ビュー 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 楽曲一覧とタブで切り替え可能な難易度表の譜面一覧画面を追加する

**Architecture:** 独立コンポーネント方式。バックエンドに2つの新規API（エントリ一覧+譜面詳細）を追加し、フロントエンドにDifficultyTableView（上ペイン）+ ChartDetail（下ペイン）を新規作成。App.svelteにタブ切り替えUIを追加。

**Tech Stack:** Go (SQLite), Svelte, @tanstack/svelte-table, @tanstack/svelte-virtual, DaisyUI, Wails v2

**設計ドキュメント:** `docs/plans/2026-03-01-difficulty-table-view-design.md`

---

## Task 1: DifficultyTableRepository.ListEntries メソッド追加

**Files:**
- Modify: `internal/adapter/persistence/difficulty_table_repository.go`
- Test: `internal/adapter/persistence/difficulty_table_repository_test.go` (新規作成)

**Step 1: テストファイル作成**

`difficulty_table_repository_test.go` を新規作成。インメモリDB + RunMigrationsパターン（elsa_repository_test.go準拠）で、ListEntriesのテストを書く。

```go
package persistence

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupDTRepo(t *testing.T) *DifficultyTableRepository {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	return NewDifficultyTableRepository(db)
}

func TestListEntries(t *testing.T) {
	repo := setupDTRepo(t)
	ctx := context.Background()

	// テーブル登録
	tableID, err := repo.InsertTable(ctx, DifficultyTable{
		URL: "http://example.com", HeaderURL: "http://example.com/header.json",
		DataURL: "http://example.com/body.json", Name: "Test Table", Symbol: "T",
	})
	if err != nil {
		t.Fatal(err)
	}

	// エントリ登録
	entries := []DifficultyTableEntry{
		{TableID: tableID, MD5: "aaa", Level: "1", Title: "Song A", Artist: "Artist A", URL: "http://dl.example.com/a", URLDiff: ""},
		{TableID: tableID, MD5: "bbb", Level: "2", Title: "Song B", Artist: "Artist B", URL: "", URLDiff: "http://dl.example.com/b"},
	}
	if err := repo.ReplaceEntries(ctx, tableID, entries); err != nil {
		t.Fatal(err)
	}

	// ListEntries
	result, err := repo.ListEntries(ctx, tableID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result[0].MD5 != "aaa" || result[0].Level != "1" {
		t.Errorf("unexpected first entry: %+v", result[0])
	}
}

func TestListEntries_EmptyTable(t *testing.T) {
	repo := setupDTRepo(t)
	ctx := context.Background()

	tableID, _ := repo.InsertTable(ctx, DifficultyTable{
		URL: "http://example.com", HeaderURL: "http://example.com/h",
		DataURL: "http://example.com/b", Name: "Empty", Symbol: "E",
	})

	result, err := repo.ListEntries(ctx, tableID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(result))
	}
}
```

**Step 2: テスト失敗を確認**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/adapter/persistence/ -run TestListEntries -v
```

期待: コンパイルエラー（ListEntriesメソッドが未定義）

**Step 3: ListEntries 実装**

`difficulty_table_repository.go` の `CountEntries` メソッドの直後（行127付近）に追加:

```go
func (r *DifficultyTableRepository) ListEntries(ctx context.Context, tableID int) ([]DifficultyTableEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT table_id, md5, level, COALESCE(title, ''), COALESCE(artist, ''), COALESCE(url, ''), COALESCE(url_diff, '')
		FROM difficulty_table_entry
		WHERE table_id = ?
		ORDER BY level, title
	`, tableID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []DifficultyTableEntry
	for rows.Next() {
		var e DifficultyTableEntry
		if err := rows.Scan(&e.TableID, &e.MD5, &e.Level, &e.Title, &e.Artist, &e.URL, &e.URLDiff); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
```

**Step 4: テスト通過を確認**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/adapter/persistence/ -run TestListEntries -v
```

期待: PASS

**Step 5: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && git add internal/adapter/persistence/difficulty_table_repository.go internal/adapter/persistence/difficulty_table_repository_test.go && git commit -m "feat: DifficultyTableRepository.ListEntriesメソッドを追加"
```

---

## Task 2: SongdataReader に md5 照合メソッド追加

**Files:**
- Modify: `internal/adapter/persistence/songdata_reader.go`

**Step 1: CountChartsByMD5s 実装**

`songdata_reader.go` の末尾（`GetSongByFolder`の後）に追加。songdata.song テーブルをmd5でGROUP BYし、各md5の件数を返す。

```go
// CountChartsByMD5s は指定md5群がsongdata.db内に何件存在するかを返す
func (r *SongdataReader) CountChartsByMD5s(ctx context.Context, md5s []string) (map[string]int, error) {
	if len(md5s) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(md5s))
	args := make([]interface{}, len(md5s))
	for i, m := range md5s {
		placeholders[i] = "?"
		args[i] = m
	}

	query := `
		SELECT md5, COUNT(*) FROM songdata.song
		WHERE md5 IN (` + joinStrings(placeholders, ",") + `)
		GROUP BY md5
	`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("CountChartsByMD5s: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var md5 string
		var count int
		if err := rows.Scan(&md5, &count); err != nil {
			return nil, err
		}
		result[md5] = count
	}
	return result, rows.Err()
}
```

注: `joinStrings` は `difficulty_table_repository.go` で定義済み（同一パッケージ）。

**Step 2: GetChartByMD5 実装**

`songdata_reader.go` に追加。md5でsongdata.songから1件取得し、IRMeta・難易度ラベルを付与して返す。

```go
// GetChartByMD5 はmd5で譜面を1件取得し、IRメタ・難易度ラベルを付与して返す
func (r *SongdataReader) GetChartByMD5(ctx context.Context, md5 string) (*model.Chart, error) {
	var c model.Chart
	err := r.db.QueryRowContext(ctx, `
		SELECT md5, sha256, title, artist, COALESCE(subartist, ''),
			genre, mode, difficulty, level, minbpm, maxbpm, path
		FROM songdata.song
		WHERE md5 = ?
		LIMIT 1
	`, md5).Scan(
		&c.MD5, &c.SHA256, &c.Title, &c.Artist, &c.SubArtist,
		&c.Genre, &c.Mode, &c.Difficulty, &c.Level,
		&c.MinBPM, &c.MaxBPM, &c.Path,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("GetChartByMD5 query: %w", err)
	}

	// IRメタ付与
	irMeta, err := r.metaRepo.GetChartMeta(ctx, c.MD5, c.SHA256)
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

**Step 3: ビルド確認**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...
```

期待: ビルド成功

**Step 4: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && git add internal/adapter/persistence/songdata_reader.go && git commit -m "feat: SongdataReaderにCountChartsByMD5s/GetChartByMD5を追加"
```

---

## Task 3: App層 — DTO定義 + 新規APIメソッド

**Files:**
- Modify: `app.go`

**Step 1: App構造体に songReader フィールド追加**

`app.go` のApp構造体にフィールドを追加:

```go
type App struct {
	ctx         context.Context
	db          *sql.DB
	SongHandler *internalapp.SongHandler
	IRHandler   *internalapp.IRHandler
	dtRepo      *persistence.DifficultyTableRepository
	dtFetcher   *gateway.DifficultyTableFetcher
	songReader  *persistence.SongdataReader  // 追加
}
```

`Init()` メソッド内で `songdataReader` をフィールドに保存:

```go
songdataReader := persistence.NewSongdataReader(db, elsaRepo, a.dtRepo)
a.songReader = songdataReader  // 追加
```

**Step 2: DTO定義**

`app.go` の `RefreshResult` 定義の後（行195付近）に追加:

```go
type DifficultyTableEntryDTO struct {
	MD5            string `json:"md5"`
	Level          string `json:"level"`
	Title          string `json:"title"`
	Artist         string `json:"artist"`
	URL            string `json:"url"`
	URLDiff        string `json:"urlDiff"`
	Status         string `json:"status"`         // "installed", "not_installed", "duplicate"
	InstalledCount int    `json:"installedCount"`
}
```

**Step 3: ListDifficultyTableEntries 実装**

```go
func (a *App) ListDifficultyTableEntries(tableID int) ([]DifficultyTableEntryDTO, error) {
	entries, err := a.dtRepo.ListEntries(a.ctx, tableID)
	if err != nil {
		return nil, err
	}

	// md5一覧を収集
	md5s := make([]string, len(entries))
	for i, e := range entries {
		md5s[i] = e.MD5
	}

	// songdata.dbとmd5で照合
	counts, err := a.songReader.CountChartsByMD5s(a.ctx, md5s)
	if err != nil {
		return nil, err
	}

	result := make([]DifficultyTableEntryDTO, len(entries))
	for i, e := range entries {
		count := 0
		if counts != nil {
			count = counts[e.MD5]
		}
		status := "not_installed"
		if count == 1 {
			status = "installed"
		} else if count > 1 {
			status = "duplicate"
		}
		result[i] = DifficultyTableEntryDTO{
			MD5: e.MD5, Level: e.Level, Title: e.Title, Artist: e.Artist,
			URL: e.URL, URLDiff: e.URLDiff,
			Status: status, InstalledCount: count,
		}
	}
	return result, nil
}
```

**Step 4: GetChartDetailByMD5 実装**

既存の `dto.ChartDTO` を返す。未導入の場合はnilを返し、フロントエンドがエントリ情報でフォールバック。

```go
func (a *App) GetChartDetailByMD5(md5 string) (*dto.ChartDTO, error) {
	chart, err := a.songReader.GetChartByMD5(a.ctx, md5)
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

注: `app.go` のimportに `"github.com/meta-BE/bms-elsa/internal/app/dto"` を追加する必要がある。

**Step 5: ビルド確認**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...
```

期待: ビルド成功

**Step 6: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && git add app.go && git commit -m "feat: 難易度表エントリ一覧・譜面詳細APIを追加"
```

---

## Task 4: Wailsバインディング再生成

**Files:**
- Auto-generated: `frontend/wailsjs/go/main/App.{d.ts,js}`, `frontend/wailsjs/go/models.ts`

**Step 1: バインディング再生成**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails generate module
```

**Step 2: 生成結果確認**

`frontend/wailsjs/go/main/App.d.ts` に `ListDifficultyTableEntries` と `GetChartDetailByMD5` が追加されていること、`models.ts` に `DifficultyTableEntryDTO` が追加されていることを確認。

**Step 3: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && git add frontend/wailsjs/ && git commit -m "chore: Wailsバインディング再生成"
```

---

## Task 5: App.svelte タブ切り替えUI

**Files:**
- Modify: `frontend/src/App.svelte`

**Step 1: タブ状態とimport追加**

`App.svelte` の `<script>` 冒頭に追加:

```typescript
import DifficultyTableView from './DifficultyTableView.svelte'
import ChartDetail from './ChartDetail.svelte'

// ...既存のlet宣言の後に追加
type TabId = 'songs' | 'difficulty'
let activeTab: TabId = 'songs'

// 難易度表タブ用の状態
let selectedEntryMD5: string | null = null
let selectedEntryData: any = null  // 行クリック時のエントリ情報

function switchTab(tab: TabId) {
  activeTab = tab
  // タブ切り替え時に選択状態リセット
  if (tab === 'songs') {
    selectedEntryMD5 = null
    selectedEntryData = null
  } else {
    selectedFolderHash = null
  }
}

function handleEntrySelect(e: CustomEvent<{ md5: string; entry: any }>) {
  selectedEntryMD5 = e.detail.md5
  selectedEntryData = e.detail.entry
}
```

**Step 2: テンプレート変更**

navbarの下、メインコンテナの前にタブバーを追加。上ペインと下ペインをタブで条件分岐。

```svelte
<!-- navbar直後 -->
<div class="tabs tabs-bordered px-4 bg-base-100 shrink-0">
  <button
    class="tab"
    class:tab-active={activeTab === 'songs'}
    on:click={() => switchTab('songs')}
  >楽曲一覧</button>
  <button
    class="tab"
    class:tab-active={activeTab === 'difficulty'}
    on:click={() => switchTab('difficulty')}
  >難易度表</button>
</div>

<!-- メインコンテナ -->
<div bind:this={containerEl} class="flex-1 overflow-hidden p-4 flex flex-col" on:click={handleDeselect}>
  {#if activeTab === 'songs'}
    <!-- 楽曲一覧タブ（既存） -->
    <div class="overflow-hidden" style="flex: {selectedFolderHash ? splitRatio : 1}">
      <SongTable on:select={handleSelect} on:deselect={handleDeselect} />
    </div>
    {#if selectedFolderHash}
      <div class="h-1 shrink-0 cursor-row-resize bg-base-300 hover:bg-primary/30 transition-colors my-1 rounded"
        on:mousedown={onDragStart} role="separator" tabindex="0"></div>
      <div class="overflow-y-auto" style="flex: {1 - splitRatio}" on:click|stopPropagation>
        <SongDetail folderHash={selectedFolderHash} on:close={handleClose} />
      </div>
    {/if}
  {:else}
    <!-- 難易度表タブ -->
    <div class="overflow-hidden" style="flex: {selectedEntryMD5 ? splitRatio : 1}">
      <DifficultyTableView on:select={handleEntrySelect} />
    </div>
    {#if selectedEntryMD5 && selectedEntryData}
      <div class="h-1 shrink-0 cursor-row-resize bg-base-300 hover:bg-primary/30 transition-colors my-1 rounded"
        on:mousedown={onDragStart} role="separator" tabindex="0"></div>
      <div class="overflow-y-auto" style="flex: {1 - splitRatio}" on:click|stopPropagation>
        <ChartDetail md5={selectedEntryMD5} entryData={selectedEntryData} />
      </div>
    {/if}
  {/if}
</div>
```

**Step 3: ビルド確認**

この時点ではDifficultyTableView/ChartDetailが未作成のため、空のスタブファイルを先に作成してビルドが通ることを確認。

`frontend/src/DifficultyTableView.svelte`:
```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  const dispatch = createEventDispatcher()
</script>
<div>DifficultyTableView placeholder</div>
```

`frontend/src/ChartDetail.svelte`:
```svelte
<script lang="ts">
  export let md5: string
  export let entryData: any
</script>
<div>ChartDetail placeholder: {md5}</div>
```

**Step 4: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && git add frontend/src/App.svelte frontend/src/DifficultyTableView.svelte frontend/src/ChartDetail.svelte && git commit -m "feat: App.svelteにタブ切り替えUIを追加"
```

---

## Task 6: DifficultyTableView.svelte 実装

**Files:**
- Modify: `frontend/src/DifficultyTableView.svelte`

**Step 1: 完全な実装**

SongTable.svelteのパターンに準拠。ドロップダウン + tanstack-table + 仮想スクロール + 背景色制御。

```svelte
<script lang="ts">
  import {
    createSvelteTable,
    flexRender,
    getCoreRowModel,
    getSortedRowModel,
    type ColumnDef,
    type SortingState,
    type TableOptions,
  } from '@tanstack/svelte-table'
  import { createVirtualizer } from '@tanstack/svelte-virtual'
  import { writable } from 'svelte/store'
  import { onMount, createEventDispatcher } from 'svelte'
  import { ListDifficultyTables, ListDifficultyTableEntries } from '../wailsjs/go/main/App'
  import type { main } from '../wailsjs/go/models'

  const dispatch = createEventDispatcher<{
    select: { md5: string; entry: main.DifficultyTableEntryDTO }
  }>()

  const ROW_HEIGHT = 32

  let tables: main.DifficultyTableDTO[] = []
  let selectedTableId: number | null = null
  let data: main.DifficultyTableEntryDTO[] = []
  let loading = true
  let entriesLoading = false
  let selectedMD5: string | null = null

  const columns: ColumnDef<main.DifficultyTableEntryDTO>[] = [
    { accessorKey: 'level', header: 'Level', size: 80 },
    { accessorKey: 'title', header: 'Title', size: 300 },
    { accessorKey: 'artist', header: 'Artist', size: 200 },
    {
      id: 'hasUrl',
      header: 'URL',
      size: 50,
      accessorFn: (row) => row.url ? '●' : '',
    },
    {
      id: 'statusLabel',
      header: 'Status',
      size: 80,
      accessorFn: (row) => {
        if (row.status === 'installed') return '導入済'
        if (row.status === 'duplicate') return '重複'
        return '未導入'
      },
    },
  ]

  let sorting: SortingState = []

  const options = writable<TableOptions<main.DifficultyTableEntryDTO>>({
    data,
    columns,
    state: { sorting },
    onSortingChange: (updater) => {
      if (typeof updater === 'function') {
        sorting = updater(sorting)
      } else {
        sorting = updater
      }
      options.update((o) => ({ ...o, state: { ...o.state, sorting } }))
    },
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  })

  const table = createSvelteTable(options)

  let scrollElement: HTMLDivElement

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
    try {
      tables = (await ListDifficultyTables()) || []
      if (tables.length > 0) {
        selectedTableId = tables[0].id
        await loadEntries(selectedTableId)
      }
    } catch (e) {
      console.error('Failed to load tables:', e)
    } finally {
      loading = false
    }
  })

  async function loadEntries(tableId: number) {
    entriesLoading = true
    selectedMD5 = null
    try {
      data = (await ListDifficultyTableEntries(tableId)) || []
    } catch (e) {
      console.error('Failed to load entries:', e)
      data = []
    } finally {
      entriesLoading = false
    }
    options.update((o) => ({ ...o, data }))
  }

  async function onTableChange(e: Event) {
    const id = parseInt((e.target as HTMLSelectElement).value)
    selectedTableId = id
    await loadEntries(id)
  }

  function rowBgClass(status: string): string {
    if (status === 'not_installed') return 'bg-base-300/50'
    if (status === 'duplicate') return 'bg-warning/20'
    return ''
  }

  function handleRowClick(entry: main.DifficultyTableEntryDTO) {
    selectedMD5 = entry.md5
    dispatch('select', { md5: entry.md5, entry })
  }
</script>

<div class="h-full flex flex-col bg-base-100 rounded-lg border border-base-300">
  <!-- ヘッダー -->
  <div class="px-4 py-2 bg-base-200 rounded-t-lg flex items-center justify-between gap-2">
    {#if loading}
      <span class="text-sm font-semibold">Loading...</span>
    {:else if tables.length === 0}
      <span class="text-sm text-base-content/50">Settings画面から難易度表を追加してください</span>
    {:else}
      <select
        class="select select-sm select-bordered"
        value={selectedTableId}
        on:change={onTableChange}
      >
        {#each tables as t}
          <option value={t.id}>{t.name} ({t.entryCount})</option>
        {/each}
      </select>
      <span class="text-sm text-base-content/70">{data.length} entries</span>
    {/if}
  </div>

  <!-- テーブルヘッダー -->
  <div class="bg-base-200 border-b border-base-300 px-2">
    {#each $table.getHeaderGroups() as headerGroup}
      <div class="flex">
        {#each headerGroup.headers as header}
          <div
            role="columnheader"
            tabindex="0"
            class="px-2 py-1.5 text-xs font-bold uppercase cursor-pointer select-none hover:bg-base-300 transition-colors truncate"
            style="width: {header.getSize()}px; min-width: {header.getSize()}px"
            on:click|stopPropagation={header.column.getToggleSortingHandler()}
            on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') header.column.getToggleSortingHandler()?.(e) }}
          >
            <span class="flex items-center gap-1">
              {#if !header.isPlaceholder}
                <svelte:component this={flexRender(header.column.columnDef.header, header.getContext())} />
              {/if}
              {#if header.column.getIsSorted() === 'asc'}
                <span>▲</span>
              {:else if header.column.getIsSorted() === 'desc'}
                <span>▼</span>
              {/if}
            </span>
          </div>
        {/each}
      </div>
    {/each}
  </div>

  <!-- 仮想スクロール領域 -->
  <div
    bind:this={scrollElement}
    class="flex-1 overflow-auto"
    role="grid"
    tabindex="-1"
  >
    {#if entriesLoading}
      <div class="flex items-center justify-center h-32">
        <span class="loading loading-spinner loading-md"></span>
      </div>
    {:else if data.length === 0 && !loading}
      <div class="flex items-center justify-center h-32 text-base-content/50">
        エントリがありません。更新してください
      </div>
    {:else}
      <div style="height: {totalSize}px; position: relative;">
        {#each virtualItems as virtualRow (virtualRow.index)}
          {@const row = rows[virtualRow.index]}
          <div
            role="row"
            tabindex="0"
            class="flex absolute w-full hover:bg-base-200 border-b border-base-300/50 items-center px-2 cursor-pointer {rowBgClass(row.original.status)}"
            class:!bg-primary/20={selectedMD5 === row.original.md5}
            style="height: {virtualRow.size}px; transform: translateY({virtualRow.start}px);"
            on:click|stopPropagation={() => handleRowClick(row.original)}
            on:keydown|stopPropagation={(e) => { if (e.key === 'Enter' || e.key === ' ') handleRowClick(row.original) }}
          >
            {#each row.getVisibleCells() as cell}
              <div
                class="px-2 text-sm truncate"
                style="width: {cell.column.getSize()}px; min-width: {cell.column.getSize()}px"
              >
                <svelte:component this={flexRender(cell.column.columnDef.cell, cell.getContext())} />
              </div>
            {/each}
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>
```

**Step 2: ビルド確認**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && cd frontend && npm run build
```

注: `main.DifficultyTableEntryDTO` の型がWailsバインディングに存在することが前提（Task 4で生成済み）。型エラーが出た場合は `models.ts` の型名を確認して調整。

**Step 3: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && git add frontend/src/DifficultyTableView.svelte && git commit -m "feat: DifficultyTableView.svelteを実装"
```

---

## Task 7: ChartDetail.svelte 実装

**Files:**
- Modify: `frontend/src/ChartDetail.svelte`

**Step 1: 完全な実装**

SongDetail.svelteのパターンに準拠。導入済みならGetChartDetailByMD5で詳細取得、未導入ならエントリ情報のみ表示。

```svelte
<script lang="ts">
  import { GetChartDetailByMD5 } from '../wailsjs/go/main/App'
  import { LookupByMD5, UpdateChartMeta } from '../wailsjs/go/app/IRHandler'
  import type { dto, main } from '../wailsjs/go/models'

  export let md5: string
  export let entryData: main.DifficultyTableEntryDTO

  let chart: dto.ChartDTO | null = null
  let loading = false

  let editWorkingBodyUrl = ''
  let editWorkingDiffUrl = ''

  $: if (md5) loadChart(md5)

  async function loadChart(targetMD5: string) {
    loading = true
    chart = null
    try {
      chart = await GetChartDetailByMD5(targetMD5)
      if (chart) {
        editWorkingBodyUrl = chart.workingBodyUrl || ''
        editWorkingDiffUrl = chart.workingDiffUrl || ''
      }
    } catch (e) {
      console.error('Failed to load chart detail:', e)
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
    {#if entryData.status === 'not_installed'}
      <!-- 未導入 -->
      <div class="bg-warning/10 rounded-lg p-3">
        <div class="flex items-center gap-2 mb-2">
          <span class="badge badge-warning badge-sm">未導入</span>
          <span class="text-sm font-semibold">この譜面はsongdata.dbに存在しません</span>
        </div>
      </div>
    {/if}

    <!-- エントリ基本情報（常に表示） -->
    <div class="bg-base-200 rounded-lg p-3">
      <h2 class="text-lg font-bold truncate">{entryData.title || '(no title)'}</h2>
      <p class="text-sm text-base-content/70">{entryData.artist || '(no artist)'}</p>
      <div class="flex gap-2 mt-1 text-xs">
        <span class="badge badge-sm badge-outline">Lv.{entryData.level}</span>
        <span class="text-base-content/50">MD5: {entryData.md5.slice(0, 16)}...</span>
      </div>
      {#if entryData.url}
        <div class="mt-2 text-xs">
          <span class="font-semibold">URL:</span>
          <a href={entryData.url} target="_blank" rel="noopener noreferrer" class="link link-primary ml-1">{entryData.url}</a>
        </div>
      {/if}
      {#if entryData.urlDiff}
        <div class="text-xs">
          <span class="font-semibold">差分URL:</span>
          <a href={entryData.urlDiff} target="_blank" rel="noopener noreferrer" class="link link-primary ml-1">{entryData.urlDiff}</a>
        </div>
      {/if}
    </div>

    <!-- 導入済みの場合: 譜面詳細 -->
    {#if chart}
      <div class="bg-base-200 rounded-lg p-3">
        <h3 class="text-sm font-semibold mb-2">譜面情報</h3>
        <div class="text-xs space-y-1">
          <p><span class="font-semibold">モード:</span> {modeLabel(chart.mode)} / {diffLabel(chart.difficulty)} / ☆{chart.level}</p>
          <p><span class="font-semibold">BPM:</span> {chart.minBpm === chart.maxBpm ? chart.minBpm : `${chart.minBpm}-${chart.maxBpm}`}</p>
          {#if chart.difficultyLabels?.length}
            <div class="flex gap-1 flex-wrap">
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
          <p class="text-xs text-base-content/50">IR情報なし</p>
        {/if}
      </div>
    {/if}
  </div>
{/if}
```

**Step 2: ビルド確認**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && cd frontend && npm run build
```

**Step 3: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && git add frontend/src/ChartDetail.svelte && git commit -m "feat: ChartDetail.svelteを実装"
```

---

## Task 8: 統合動作確認

**Step 1: wails dev で起動**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails dev
```

**Step 2: 動作確認チェックリスト**

- [ ] タブ切り替え（楽曲一覧 ↔ 難易度表）が動作する
- [ ] 難易度表タブでドロップダウンに登録済みテーブルが表示される
- [ ] テーブル選択でエントリ一覧が表示される
- [ ] 行の背景色が導入ステータスで色分けされる（デフォルト/グレー/黄色）
- [ ] 行クリックで下ペインにChartDetailが表示される
- [ ] 導入済み譜面: 譜面情報 + IR情報 + 難易度ラベル + url/url_diff
- [ ] 未導入譜面: 「未導入です」メッセージ + エントリ基本情報
- [ ] 楽曲一覧タブに戻しても既存機能が正常動作する

**Step 3: 型エラーや表示崩れがあれば修正してコミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && git add -A && git commit -m "fix: 統合動作確認での修正"
```
