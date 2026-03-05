# カラムフィルタ実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 楽曲一覧・譜面一覧のEVENT/YEARカラム、難易度表のSTATUSカラムをソートからドロップダウンフィルタに変更する。

**Architecture:** TanStack Tableのカラムフィルタ機能（`column.setFilterValue()`）を使用。SortableHeaderを拡張してフィルタ対象カラムにドロップダウンを表示する。選択肢はファセット値または固定リストから取得。

**Tech Stack:** Svelte + TypeScript, @tanstack/svelte-table

---

### Task 1: SortableHeader にフィルタドロップダウンのサポートを追加

**Files:**
- Modify: `frontend/src/SortableHeader.svelte`

**Step 1: SortableHeader を修正**

`meta.filterType === 'select'` のカラムにはドロップダウンを表示する。選択肢は `meta.filterOptions`（固定）または `column.getFacetedUniqueValues()`（動的）から取得。

```svelte
<script lang="ts">
  import { flexRender, type Table, type Column } from '@tanstack/svelte-table'

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  export let table: Table<any>

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  function getFilterOptions(column: Column<any, unknown>): string[] {
    const meta = column.columnDef.meta as { filterOptions?: string[] } | undefined
    if (meta?.filterOptions) return meta.filterOptions
    try {
      const values = column.getFacetedUniqueValues()
      return Array.from(values.keys())
        .filter((v) => v != null && v !== '')
        .map(String)
        .sort()
    } catch {
      return []
    }
  }
</script>

<div class="bg-base-200 border-b border-base-300 px-2 shrink-0">
  {#each table.getHeaderGroups() as headerGroup}
    <div class="flex">
      {#each headerGroup.headers as header}
        {#if (header.column.columnDef.meta as any)?.filterType === 'select'}
          <div
            class="px-1 py-1 text-xs truncate"
            style="width: {header.getSize()}px; min-width: {header.getSize()}px"
          >
            <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
            <select
              class="select select-xs w-full min-h-0 h-6 font-bold uppercase"
              value={String(header.column.getFilterValue() ?? '')}
              on:change={(e) => {
                const val = e.currentTarget.value
                header.column.setFilterValue(val || undefined)
              }}
              on:click|stopPropagation
            >
              <option value="">{header.column.columnDef.header}</option>
              {#each getFilterOptions(header.column) as opt}
                <option value={opt}>{opt}</option>
              {/each}
            </select>
          </div>
        {:else}
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
                <svelte:component
                  this={flexRender(header.column.columnDef.header, header.getContext())}
                />
              {/if}
              {#if header.column.getIsSorted() === 'asc'}
                <span>▲</span>
              {:else if header.column.getIsSorted() === 'desc'}
                <span>▼</span>
              {/if}
            </span>
          </div>
        {/if}
      {/each}
    </div>
  {/each}
</div>
```

**Step 2: コミット**

```bash
git add frontend/src/SortableHeader.svelte
git commit -m "feat: SortableHeaderにフィルタドロップダウンのサポートを追加"
```

---

### Task 2: SongTable の EVENT/YEAR カラムをフィルタに変更

**Files:**
- Modify: `frontend/src/SongTable.svelte`

**Step 1: import に `getFacetedRowModel`, `getFacetedUniqueValues` を追加**

```typescript
import {
  createSvelteTable,
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  getFacetedRowModel,
  getFacetedUniqueValues,
  type ColumnDef,
  type SortingState,
  type FilterFn,
} from '@tanstack/svelte-table'
```

**Step 2: EVENT/YEAR カラム定義を変更**

EVENT カラム（現在の `{ accessorKey: 'eventName', header: 'Event', size: 140 }`）を以下に変更:

```typescript
{
  id: 'eventName',
  header: 'Event',
  size: 140,
  accessorFn: (row) => row.eventName || '',
  enableSorting: false,
  filterFn: 'equalsString',
  meta: { filterType: 'select' },
},
```

YEAR カラム（現在の `{ accessorKey: 'releaseYear', header: 'Year', size: 60 }`）を以下に変更:

```typescript
{
  id: 'releaseYear',
  header: 'Year',
  size: 60,
  accessorFn: (row) => row.releaseYear ? String(row.releaseYear) : '',
  enableSorting: false,
  filterFn: 'equalsString',
  meta: { filterType: 'select' },
},
```

**Step 3: テーブル設定に faceted model を追加**

`createSvelteTable` に以下を追加:

```typescript
$: table = createSvelteTable({
  data: songs,
  columns,
  state: { sorting, globalFilter },
  onSortingChange: (updater) => {
    sorting = typeof updater === 'function' ? updater(sorting) : updater
  },
  globalFilterFn: searchFilter,
  getCoreRowModel: getCoreRowModel(),
  getFilteredRowModel: getFilteredRowModel(),
  getSortedRowModel: getSortedRowModel(),
  getFacetedRowModel: getFacetedRowModel(),
  getFacetedUniqueValues: getFacetedUniqueValues(),
})
```

**Step 4: ビルド確認**

```bash
cd frontend && npm run build
```

**Step 5: コミット**

```bash
git add frontend/src/SongTable.svelte
git commit -m "feat: SongTableのEVENT/YEARカラムをフィルタに変更"
```

---

### Task 3: ChartListView の EVENT/YEAR カラムをフィルタに変更

**Files:**
- Modify: `frontend/src/ChartListView.svelte`

**Step 1: import に `getFacetedRowModel`, `getFacetedUniqueValues` を追加**

```typescript
import {
  createSvelteTable,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  getFacetedRowModel,
  getFacetedUniqueValues,
  flexRender,
  type ColumnDef,
  type SortingState,
  type FilterFn,
} from '@tanstack/svelte-table'
```

**Step 2: EVENT カラム定義を変更**

現在の EVENT カラムに `enableSorting`, `filterFn`, `meta` を追加:

```typescript
{
  id: 'eventName',
  header: 'Event',
  size: 140,
  accessorFn: (row) => row.eventName || '',
  enableSorting: false,
  filterFn: 'equalsString',
  meta: { filterType: 'select' },
},
```

**Step 3: YEAR カラム定義を変更**

`accessorFn` を文字列化し、フィルタ設定を追加:

```typescript
{
  id: 'releaseYear',
  header: 'Year',
  size: 60,
  accessorFn: (row) => row.releaseYear ? String(row.releaseYear) : '',
  enableSorting: false,
  filterFn: 'equalsString',
  meta: { filterType: 'select' },
},
```

**Step 4: テーブル設定に faceted model を追加**

`createSvelteTable` に `getFacetedRowModel()` と `getFacetedUniqueValues()` を追加:

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
  getFacetedRowModel: getFacetedRowModel(),
  getFacetedUniqueValues: getFacetedUniqueValues(),
})
```

**Step 5: コミット**

```bash
git add frontend/src/ChartListView.svelte
git commit -m "feat: ChartListViewのEVENT/YEARカラムをフィルタに変更"
```

---

### Task 4: DifficultyTableView の STATUS カラムをフィルタに変更

**Files:**
- Modify: `frontend/src/DifficultyTableView.svelte`

**Step 1: import に `getFilteredRowModel` を追加**

```typescript
import {
  createSvelteTable,
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  type ColumnDef,
  type SortingState,
  type TableOptions,
} from '@tanstack/svelte-table'
```

**Step 2: STATUS カラム定義を変更**

`enableSorting: false`, `filterFn`, `meta` を追加:

```typescript
{
  id: 'statusLabel',
  header: 'Status',
  size: 100,
  accessorFn: (row) => {
    if (row.status === 'installed') return '導入済'
    if (row.status === 'not_installed') return '未導入'
    if (row.status === 'duplicate') return '重複'
    return row.status
  },
  enableSorting: false,
  filterFn: 'equalsString',
  meta: { filterType: 'select', filterOptions: ['導入済', '未導入', '重複'] },
},
```

**Step 3: テーブル設定に `getFilteredRowModel` を追加**

`options` の初期値に追加:

```typescript
const options = writable<TableOptions<main.DifficultyTableEntryDTO>>({
  data: entries,
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
  getFilteredRowModel: getFilteredRowModel(),
})
```

**Step 4: ビルド確認**

```bash
cd frontend && npm run build
```

**Step 5: コミット**

```bash
git add frontend/src/DifficultyTableView.svelte
git commit -m "feat: DifficultyTableViewのSTATUSカラムをフィルタに変更"
```

---

### Task 5: 全体ビルド検証

**Step 1: フロントエンドビルド**

```bash
cd frontend && npm run build
```

**Step 2: Wailsビルド**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build
```

Expected: ビルド成功
