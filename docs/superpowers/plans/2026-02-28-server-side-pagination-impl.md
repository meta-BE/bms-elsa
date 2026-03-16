# サーバーサイドページネーション実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** SongTable の5,000件一括取得をサーバーサイドページネーションに置き換え、初回描画を高速化し20,000曲規模に対応する。

**Architecture:** TanStack Table を廃止し、カラム定義を簡素な配列に変更。TanStack Virtual のスクロール位置からページ番号を算出し、バックエンドの `ListSongs` API をページ単位で呼び出す。ソートはバックエンドの `ORDER BY` に一本化。

**Tech Stack:** Svelte 4, TanStack Virtual, Wails v2 バインディング（Go バックエンドは変更なし）

---

### Task 1: SongTable.svelte サーバーサイドページネーション化

**Files:**
- Modify: `frontend/src/SongTable.svelte` (全面書き換え)

**背景:**
- 現在の SongTable は TanStack Table (`@tanstack/svelte-table`) で行モデルを管理し、`getSortedRowModel()` でフロント側ソートを実行
- サーバーサイドページネーションでは、データがメモリ上の単一配列ではなくページキャッシュ（`Map<number, SongRowDTO[]>`）になる
- TanStack Table はインメモリ配列前提の設計であり、ページキャッシュとの組み合わせは複雑になるため廃止する
- カラム定義・ヘッダー・セルの描画はシンプルなので手動レンダリングに移行
- TanStack Virtual はスクロール位置管理のみ担当するので引き続き使用

**Step 1: SongTable.svelte を書き換え**

`frontend/src/SongTable.svelte` を以下の内容で置き換える:

```svelte
<script lang="ts">
  import { createVirtualizer } from '@tanstack/svelte-virtual'
  import { onMount, createEventDispatcher } from 'svelte'
  import { ListSongs } from '../wailsjs/go/app/SongHandler'
  import type { dto } from '../wailsjs/go/models'

  const dispatch = createEventDispatcher<{ select: string; deselect: void }>()

  const ROW_HEIGHT = 32
  const PAGE_SIZE = 100

  // カラム定義（TanStack Table の ColumnDef を簡素な型に置換）
  type Column = {
    key: string
    header: string
    size: number
    accessor: (row: dto.SongRowDTO) => string
  }

  const columns: Column[] = [
    { key: 'title', header: 'Title', size: 300, accessor: (r) => r.title },
    { key: 'artist', header: 'Artist', size: 200, accessor: (r) => r.artist },
    { key: 'genre', header: 'Genre', size: 140, accessor: (r) => r.genre },
    {
      key: 'bpm',
      header: 'BPM',
      size: 100,
      accessor: (r) => {
        if (r.minBpm === r.maxBpm) return String(Math.round(r.minBpm))
        return `${Math.round(r.minBpm)}-${Math.round(r.maxBpm)}`
      },
    },
    { key: 'eventName', header: 'Event', size: 140, accessor: (r) => r.eventName ?? '' },
    {
      key: 'releaseYear',
      header: 'Year',
      size: 60,
      accessor: (r) => (r.releaseYear != null ? String(r.releaseYear) : ''),
    },
    { key: 'ir', header: 'IR', size: 40, accessor: (r) => (r.hasIrMeta ? '●' : '') },
    { key: 'chartCount', header: 'Charts', size: 60, accessor: (r) => String(r.chartCount) },
  ]

  // 状態
  let totalCount = 0
  let loading = true
  let pageCache = new Map<number, dto.SongRowDTO[]>()
  let loadingPages = new Set<number>()
  let generation = 0 // ソート切替時にインクリメントし、古いレスポンスを破棄する
  let sortBy = 'title'
  let sortDesc = false
  let scrollElement: HTMLDivElement

  // ページ取得（1-based page number）
  async function fetchPage(page: number) {
    const gen = generation
    if (pageCache.has(page) || loadingPages.has(page)) return
    loadingPages.add(page)
    loadingPages = loadingPages
    try {
      const result = await ListSongs(page, PAGE_SIZE, sortBy, sortDesc, '')
      if (gen !== generation) return // ソート変更後の古いレスポンスは破棄
      totalCount = result.totalCount
      pageCache.set(page, result.songs || [])
      pageCache = pageCache // Svelte リアクティビティのトリガー
    } catch (e) {
      console.error('Failed to load page:', page, e)
    } finally {
      loadingPages.delete(page)
      loadingPages = loadingPages
    }
  }

  // 行データアクセス（0-based index → 1-based page + offset）
  function getRow(index: number): dto.SongRowDTO | null {
    const page = Math.floor(index / PAGE_SIZE) + 1
    const offset = index % PAGE_SIZE
    return pageCache.get(page)?.[offset] ?? null
  }

  // ソート切替
  async function toggleSort(key: string) {
    if (sortBy === key) {
      sortDesc = !sortDesc
    } else {
      sortBy = key
      sortDesc = false
    }
    generation++
    pageCache.clear()
    pageCache = pageCache
    loadingPages.clear()
    loadingPages = loadingPages
    if (scrollElement) scrollElement.scrollTop = 0
    await fetchPage(1)
  }

  // TanStack Virtual
  $: virtualizer = createVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: totalCount,
    getScrollElement: () => scrollElement,
    estimateSize: () => ROW_HEIGHT,
    overscan: 20,
  })

  $: virtualItems = $virtualizer.getVirtualItems()
  $: totalSize = $virtualizer.getTotalSize()

  // スクロール連動ページ取得: 表示中の virtualItems からページ番号を算出
  $: {
    const pages = new Set<number>()
    for (const item of virtualItems) {
      pages.add(Math.floor(item.index / PAGE_SIZE) + 1)
    }
    for (const page of pages) {
      fetchPage(page)
    }
  }

  onMount(async () => {
    await fetchPage(1)
    loading = false
  })
</script>

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div
  class="h-full flex flex-col bg-base-100 rounded-lg border border-base-300"
  on:click={() => dispatch('deselect')}
>
  <div class="px-4 py-2 bg-base-200 rounded-t-lg flex items-center justify-between">
    {#if loading}
      <span class="text-sm font-semibold">Loading...</span>
    {:else}
      <span class="text-sm font-semibold">{totalCount.toLocaleString()} songs</span>
    {/if}
  </div>

  <!-- ヘッダー -->
  <div class="bg-base-200 border-b border-base-300 px-2">
    <div class="flex">
      {#each columns as col}
        <div
          role="columnheader"
          tabindex="0"
          class="px-2 py-1.5 text-xs font-bold uppercase cursor-pointer select-none hover:bg-base-300 transition-colors truncate"
          style="width: {col.size}px; min-width: {col.size}px"
          on:click|stopPropagation={() => toggleSort(col.key)}
          on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') toggleSort(col.key) }}
        >
          <span class="flex items-center gap-1">
            {col.header}
            {#if sortBy === col.key}
              <span>{sortDesc ? '▼' : '▲'}</span>
            {/if}
          </span>
        </div>
      {/each}
    </div>
  </div>

  <!-- 仮想スクロール領域 -->
  <div
    bind:this={scrollElement}
    class="flex-1 overflow-auto"
    role="grid"
    tabindex="-1"
    on:keydown={(e) => { if (e.key === 'Escape') dispatch('deselect') }}
  >
    {#if loading}
      <div class="flex items-center justify-center h-32">
        <span class="loading loading-spinner loading-md"></span>
      </div>
    {:else}
      <div style="height: {totalSize}px; position: relative;">
        {#each virtualItems as virtualRow (virtualRow.index)}
          {@const row = getRow(virtualRow.index)}
          {#if row}
            <div
              role="row"
              tabindex="0"
              class="flex absolute w-full hover:bg-base-200 border-b border-base-300/50 items-center px-2 cursor-pointer"
              style="height: {virtualRow.size}px; transform: translateY({virtualRow.start}px);"
              on:click|stopPropagation={() => dispatch('select', row.folderHash)}
              on:keydown|stopPropagation={(e) => { if (e.key === 'Enter' || e.key === ' ') dispatch('select', row.folderHash) }}
            >
              {#each columns as col}
                <div
                  class="px-2 text-sm truncate"
                  style="width: {col.size}px; min-width: {col.size}px"
                >
                  {col.accessor(row)}
                </div>
              {/each}
            </div>
          {:else}
            <div
              class="flex absolute w-full border-b border-base-300/50 items-center px-2"
              style="height: {virtualRow.size}px; transform: translateY({virtualRow.start}px);"
            >
              <div class="h-3 bg-base-300/50 rounded animate-pulse" style="width: 40%"></div>
            </div>
          {/if}
        {/each}
      </div>
    {/if}
  </div>
</div>
```

**変更ポイント:**
- `@tanstack/svelte-table` の import を全削除（`createSvelteTable`, `flexRender`, `getCoreRowModel`, `getSortedRowModel` 等）
- `writable` の import を削除（TanStack Table の options store 用だった）
- `ColumnDef<dto.SongRowDTO>[]` → 独自 `Column` 型（key/header/size/accessor）
- `data: dto.SongRowDTO[]` → `pageCache: Map<number, dto.SongRowDTO[]>`
- `PAGE_SIZE: 5000` → `100`
- `getSortedRowModel()` 削除 → ソートはバックエンドのみ
- Virtual の `count: rows.length` → `count: totalCount`
- ヘッダー: `$table.getHeaderGroups()` → `{#each columns as col}` で直接レンダリング
- 行: `row.getVisibleCells()` + `flexRender` → `{#each columns as col}{col.accessor(row)}`
- 未取得行: `animate-pulse` プレースホルダー表示

**Step 2: 型チェックの実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run check`
Expected: 0 errors, 0 warnings（A11y 関連の warning を除く）

**Step 3: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run build`
Expected: ビルド成功

**Step 4: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add frontend/src/SongTable.svelte
git commit -m "feat: サーバーサイドページネーションに移行

TanStack Table を廃止し、スクロール位置に応じてバックエンドから
ページ単位（100件）でデータを取得する方式に変更。
ソートはバックエンドの ORDER BY に一本化。"
```

---

### Task 2: 動作検証

**Step 1: wails dev で起動**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails dev`

**Step 2: 手動テストチェックリスト**

| # | テスト項目 | 期待結果 |
|---|-----------|---------|
| 1 | 起動後のテーブル表示 | 「2,666 songs」と表示され、最初の行がすぐ見える |
| 2 | 下方向スクロール | スムーズにスクロールでき、新しい行が表示される |
| 3 | 高速スクロール（末尾へジャンプ） | 一瞬プレースホルダー（灰色パルス）が見え、すぐデータに置換される |
| 4 | Title ヘッダークリック（2回） | 1回目: ▲→▼（降順に切替）、2回目: ▼→▲（昇順に戻る） |
| 5 | Artist ヘッダークリック | ソート列が Artist に変更、▲表示、テーブル先頭にスクロール |
| 6 | 行クリック | 詳細パネルが表示される |
| 7 | テーブル外クリック | 詳細パネルが閉じる |
| 8 | 詳細パネルの×ボタン | 詳細パネルが閉じる |

**Step 3: 問題があれば修正してコミット**
