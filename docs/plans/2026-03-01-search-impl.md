# 各画面への検索機能追加 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 3つのタブ（songs/charts/difficulty）にテキスト検索バーを追加する

**Architecture:** 各テーブルコンポーネントのヘッダー行に検索フィールドを追加。SongTableはバックエンドAPI（既存`search`パラメータ）+debounce、ChartListViewはTanstack Tableの`getFilteredRowModel()`、DifficultyTableViewは配列`.filter()`でフィルタリング。検索バーの行には`on:click|stopPropagation`を付与し、詳細ビューが閉じる問題を防ぐ。

**Tech Stack:** Svelte 3, TypeScript, Tanstack Table, Tanstack Virtual, DaisyUI/Tailwind CSS

**設計ドキュメント:** `docs/plans/2026-03-01-search-design.md`

---

### Task 1: SongTable — バックエンド検索の追加

**Files:**
- Modify: `frontend/src/SongTable.svelte`

**概要:** 既存の`ListSongs`APIの`search`パラメータを活用。ヘッダー行に検索フィールドを追加し、300msのdebounceでバックエンド検索を実行する。

**Step 1: script部に検索ロジックを追加**

`SongTable.svelte`のscriptセクションに以下を追加:

1. 変数宣言（`let loading = true` の後に追加）:
```typescript
let searchText = ''
let debounceTimer: ReturnType<typeof setTimeout>
```

2. 検索関数を追加（`onMount`の後に追加）:
```typescript
async function doSearch() {
  loading = true
  try {
    const result = await ListSongs(1, PAGE_SIZE, 'title', false, searchText)
    data = result.songs || []
    totalCount = result.totalCount
  } catch (e) {
    console.error('Failed to search songs:', e)
    data = []
  } finally {
    loading = false
  }
  options.update((o) => ({ ...o, data }))
}

function handleSearchInput() {
  clearTimeout(debounceTimer)
  debounceTimer = setTimeout(doSearch, 300)
}
```

**Step 2: ヘッダー行のHTMLを変更**

現在のヘッダー行（104-110行目）:
```svelte
<div class="px-4 py-2 bg-base-200 rounded-t-lg flex items-center justify-between">
  {#if loading}
    <span class="text-sm font-semibold">Loading...</span>
  {:else}
    <span class="text-sm font-semibold">{totalCount.toLocaleString()} songs</span>
  {/if}
</div>
```

以下に変更（`on:click|stopPropagation`をヘッダー行全体に付与）:
```svelte
<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div class="px-4 py-2 bg-base-200 rounded-t-lg flex items-center justify-between gap-2" on:click|stopPropagation>
  <span class="text-sm font-semibold shrink-0">
    {#if loading}Loading...{:else}{totalCount.toLocaleString()} songs{/if}
  </span>
  <input
    type="text"
    placeholder="検索..."
    class="input input-xs input-bordered w-48"
    bind:value={searchText}
    on:input={handleSearchInput}
  />
</div>
```

**Step 3: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: BUILD SUCCESS

**Step 4: コミット**

```bash
git add frontend/src/SongTable.svelte
git commit -m "feat: SongTableに検索フィールドを追加（バックエンドdebounce検索）"
```

---

### Task 2: ChartListView — Tanstack Tableフロントエンドフィルタの追加

**Files:**
- Modify: `frontend/src/ChartListView.svelte`

**概要:** Tanstack Tableの`getFilteredRowModel()`を使い、title/subtitle/artist/subArtist/genreをフロントエンドでフィルタリング。全件がメモリ上にあるため即座に反映される。

**Step 1: importに`getFilteredRowModel`を追加**

`ChartListView.svelte`の2-10行目のimport部:
```typescript
import {
  createSvelteTable,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,  // ← 追加
  flexRender,
  type ColumnDef,
  type SortingState,
  type FilterFn,         // ← 追加
} from '@tanstack/svelte-table'
```

**Step 2: フィルタ関数と状態変数を追加**

`let sorting: SortingState = []`（24行目）の後に追加:
```typescript
let globalFilter = ''

const searchFilter: FilterFn<dto.ChartListItemDTO> = (row, _columnId, filterValue) => {
  const s = (filterValue as string).toLowerCase()
  const item = row.original
  return (
    item.title.toLowerCase().includes(s) ||
    (item.subtitle || '').toLowerCase().includes(s) ||
    item.artist.toLowerCase().includes(s) ||
    (item.subArtist || '').toLowerCase().includes(s) ||
    item.genre.toLowerCase().includes(s)
  )
}
```

**Step 3: createSvelteTableにフィルタ設定を追加**

62-71行目のcreateSvelteTable呼び出しを変更:
```typescript
$: table = createSvelteTable({
  data: charts,
  columns,
  state: { sorting, globalFilter },
  onSortingChange: (updater) => {
    sorting = typeof updater === 'function' ? updater(sorting) : updater
  },
  globalFilterFn: searchFilter,
  getCoreRowModel: getCoreRowModel(),
  getFilteredRowModel: getFilteredRowModel(),
  getSortedRowModel: getSortedRowModel(),
})
```

**Step 4: ヘッダー行のHTMLを変更**

現在のヘッダー行（110-114行目）:
```svelte
<div class="flex items-center gap-2 px-4 py-2 bg-base-100 shrink-0">
  <span class="text-sm text-base-content/70">
    {charts.length} 譜面
  </span>
</div>
```

以下に変更:
```svelte
<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div class="flex items-center justify-between gap-2 px-4 py-2 bg-base-100 shrink-0" on:click|stopPropagation>
  <span class="text-sm text-base-content/70 shrink-0">
    {rows.length} 譜面
  </span>
  <input
    type="text"
    placeholder="検索..."
    class="input input-xs input-bordered w-48"
    bind:value={globalFilter}
  />
</div>
```

**Step 5: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: BUILD SUCCESS

**Step 6: コミット**

```bash
git add frontend/src/ChartListView.svelte
git commit -m "feat: ChartListViewに検索フィールドを追加（フロントエンドフィルタ）"
```

---

### Task 3: DifficultyTableView — フロントエンドフィルタの追加

**Files:**
- Modify: `frontend/src/DifficultyTableView.svelte`

**概要:** `entries`配列を`.filter()`でフィルタリングしてテーブルに渡す。テーブル切り替え時に検索文字列をクリアする。

**Step 1: 検索変数とリアクティブフィルタを追加**

`let selectedMD5: string | null = null`（29行目）の後に追加:
```typescript
let searchText = ''
```

`loadEntries`関数（103-115行目）の末尾にある`options.update((o) => ({ ...o, data: entries }))`を削除し、代わりにリアクティブブロックを追加（`loadEntries`関数の後に配置）:

```typescript
$: {
  const filtered = searchText
    ? entries.filter(e => {
        const s = searchText.toLowerCase()
        return e.title.toLowerCase().includes(s) || e.artist.toLowerCase().includes(s)
      })
    : entries
  options.update((o) => ({ ...o, data: filtered }))
}
```

**Step 2: handleTableChangeで検索をクリア**

`handleTableChange`関数（117-123行目）に`searchText = ''`を追加:
```typescript
async function handleTableChange(e: Event) {
  const target = e.target as HTMLSelectElement
  const id = Number(target.value)
  selectedTableId = id
  searchText = ''
  dispatch('deselect')
  await loadEntries(id)
}
```

**Step 3: ヘッダー行のHTMLを変更**

現在のヘッダー行（147-164行目）:
```svelte
<div class="px-4 py-2 bg-base-200 rounded-t-lg flex items-center justify-between gap-2">
  {#if loading}
    <span class="text-sm font-semibold">Loading...</span>
  {:else if tables.length === 0}
    <span class="text-sm text-base-content/50">Settings画面から難易度表を追加してください</span>
  {:else}
    <select ...>...</select>
    <span class="text-sm font-semibold">{entries.length} entries</span>
  {/if}
</div>
```

以下に変更（`on:click|stopPropagation`をヘッダー行全体に付与）:
```svelte
<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div class="px-4 py-2 bg-base-200 rounded-t-lg flex items-center justify-between gap-2" on:click|stopPropagation>
  {#if loading}
    <span class="text-sm font-semibold">Loading...</span>
  {:else if tables.length === 0}
    <span class="text-sm text-base-content/50">Settings画面から難易度表を追加してください</span>
  {:else}
    <select
      class="select select-bordered select-sm"
      value={selectedTableId}
      on:change={handleTableChange}
    >
      {#each tables as t}
        <option value={t.id}>{t.symbol} {t.name} ({t.entryCount})</option>
      {/each}
    </select>
    <span class="text-sm font-semibold shrink-0">{rows.length} entries</span>
    <input
      type="text"
      placeholder="検索..."
      class="input input-xs input-bordered w-48"
      bind:value={searchText}
    />
  {/if}
</div>
```

**Step 4: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: BUILD SUCCESS

**Step 5: コミット**

```bash
git add frontend/src/DifficultyTableView.svelte
git commit -m "feat: DifficultyTableViewに検索フィールドを追加（フロントエンドフィルタ）"
```

---

### Task 4: 全体ビルド・動作確認

**Step 1: 全テスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./...`
Expected: 全テスト PASS

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: BUILD SUCCESS
