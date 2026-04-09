# 差分導入画面 カラムリサイズ対応 実装プラン

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** DiffImportViewのHTMLテーブルを@tanstack/svelte-table + 仮想スクロールに移行し、ヘッダードラッグによるカラム幅リサイズ機能を追加する

**Architecture:** SortableHeaderにリサイズハンドルを追加（全テーブル共通）、DiffImportViewをSongTable/ChartListViewと同じtanstackパターンに書き換え。enableColumnResizing有効化でリサイズ機能を利用。

**Tech Stack:** Svelte 4, @tanstack/svelte-table 8.21.3, @tanstack/svelte-virtual 3.13.19, Tailwind CSS 4.2.1, daisyUI 5.5.19

---

### Task 1: SortableHeaderにリサイズハンドルを追加

**Files:**
- Modify: `frontend/src/components/SortableHeader.svelte`

- [ ] **Step 1: ソートヘッダーにリサイズハンドルを追加**

`frontend/src/components/SortableHeader.svelte` の102-123行（ソートヘッダー部分）を修正。各ヘッダーセルにリサイズハンドルのdivを追加する。

ソートヘッダーの `<div role="columnheader" ...>` を以下に変更:

```svelte
        {:else}
          <!-- ソートヘッダー -->
          <div
            class="relative"
            style={header.column.columnDef.meta?.flex ? `flex: 1 1 ${header.getSize()}px; min-width: ${header.getSize()}px` : `flex: 0 0 ${header.getSize()}px`}
          >
            <div
              role="columnheader"
              tabindex="0"
              class="px-2 py-1.5 text-xs font-bold uppercase cursor-pointer select-none hover:bg-base-300 transition-colors truncate"
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
            {#if header.column.getCanResize()}
              <!-- svelte-ignore a11y-no-static-element-interactions -->
              <div
                class="absolute top-0 right-0 w-1 h-full cursor-col-resize select-none touch-none
                  {header.column.getIsResizing() ? 'bg-primary' : 'hover:bg-primary/50'}"
                on:mousedown|stopPropagation={header.getResizeHandler()}
                on:touchstart|stopPropagation={header.getResizeHandler()}
              />
            {/if}
          </div>
        {/if}
```

- [ ] **Step 2: フィルタヘッダーにもリサイズハンドルを追加**

62-100行（フィルタヘッダー部分）の `<div class="relative" ...>` 内、閉じタグ `</div>` の直前（フィルタメニューのif/endifの後）にリサイズハンドルを追加:

```svelte
            {#if header.column.getCanResize()}
              <!-- svelte-ignore a11y-no-static-element-interactions -->
              <div
                class="absolute top-0 right-0 w-1 h-full cursor-col-resize select-none touch-none
                  {header.column.getIsResizing() ? 'bg-primary' : 'hover:bg-primary/50'}"
                on:mousedown|stopPropagation={header.getResizeHandler()}
                on:touchstart|stopPropagation={header.getResizeHandler()}
              />
            {/if}
```

フィルタヘッダーは既に `<div class="relative" ...>` でラップされているため、構造変更は不要。

- [ ] **Step 3: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: ビルド成功

- [ ] **Step 4: コミット**

```bash
git add frontend/src/components/SortableHeader.svelte
git commit -m "feat: SortableHeaderにカラムリサイズハンドルを追加"
```

---

### Task 2: DiffImportViewをtanstack tableに移行

**Files:**
- Modify: `frontend/src/views/DiffImportView.svelte`

- [ ] **Step 1: import文とカラム定義を追加**

`frontend/src/views/DiffImportView.svelte` の1-9行のimport部分を以下に変更:

```svelte
<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import {
    createSvelteTable,
    flexRender,
    getCoreRowModel,
    type ColumnDef,
    type ColumnSizingState,
    type ColumnSizingInfoState,
  } from '@tanstack/svelte-table'
  import { createVirtualizer } from '@tanstack/svelte-virtual'
  import { EventsOn } from '../../wailsjs/runtime/runtime'
  import { ParseAndEstimate, ExecuteImport, StopEstimate } from '../../wailsjs/go/app/DiffImportHandler'
  import type { app } from '../../wailsjs/go/models'
  import OpenFolderButton from '../components/OpenFolderButton.svelte'
  import Icon from '../components/Icon.svelte'
  import ProgressBar from '../components/ProgressBar.svelte'
  import SortableHeader from '../components/SortableHeader.svelte'
```

- [ ] **Step 2: ROW_HEIGHT定数とカラム定義を追加**

`matchMethodLabels` の定義（98-102行）の直後に以下を追加:

```typescript
  const ROW_HEIGHT = 32

  const columns: ColumnDef<app.DiffImportCandidateDTO>[] = [
    {
      accessorKey: 'fileName',
      header: 'ファイル名',
      size: 200,
      meta: { flex: true },
    },
    {
      id: 'title',
      header: 'TITLE',
      size: 200,
      meta: { flex: true },
      accessorFn: (row) => {
        const parts = [row.title, row.subtitle].filter(Boolean)
        return parts.join(' ')
      },
    },
    {
      id: 'artist',
      header: 'ARTIST',
      size: 200,
      meta: { flex: true },
      accessorFn: (row) => {
        const parts = [row.artist, row.subartist].filter(Boolean)
        return parts.join(' ')
      },
    },
    {
      accessorKey: 'destFolder',
      header: '推定先',
      size: 250,
      meta: { flex: true },
    },
    {
      id: 'score',
      header: 'スコア',
      size: 64,
      accessorFn: (row) => row.score > 0 ? Math.round(row.score * 10) : null,
    },
    {
      id: 'matchMethod',
      header: '推定方法',
      size: 80,
      accessorFn: (row) => matchMethodLabels[row.matchMethod] || row.matchMethod || '-',
    },
    {
      id: 'actions',
      header: '',
      size: 64,
      enableResizing: false,
    },
  ]
```

- [ ] **Step 3: tanstack tableインスタンスと仮想スクロールのリアクティブ宣言を追加**

`$: importableCount = ...` の行（104行付近）の直後に以下を追加:

```typescript
  let columnSizing: ColumnSizingState = {}
  let columnSizingInfo: ColumnSizingInfoState = {} as ColumnSizingInfoState

  $: table = createSvelteTable({
    data: candidates,
    columns,
    enableSorting: false,
    enableColumnResizing: true,
    columnResizeMode: 'onChange',
    state: { columnSizing, columnSizingInfo },
    onColumnSizingChange: (updater) => {
      columnSizing = typeof updater === 'function' ? updater(columnSizing) : updater
    },
    onColumnSizingInfoChange: (updater) => {
      columnSizingInfo = typeof updater === 'function' ? updater(columnSizingInfo) : updater
    },
    getCoreRowModel: getCoreRowModel(),
  })

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
```

- [ ] **Step 4: テンプレートのテーブル部分を書き換え**

150-218行（`<div class="flex-1 overflow-auto">` から `</table></div>` まで）を以下に置換:

```svelte
    <div class="flex-1 overflow-hidden flex flex-col">
      <SortableHeader table={$table} />

      <div bind:this={scrollElement} class="flex-1 overflow-y-scroll">
        <div style="height: {totalSize}px; position: relative;">
          {#each virtualItems as virtualRow (virtualRow.index)}
            {@const row = rows[virtualRow.index]}
            {@const original = row.original}
            <div
              role="row"
              class="flex absolute w-full border-b border-base-300/50 items-center px-2 hover:bg-base-200"
              style="height: {ROW_HEIGHT}px; transform: translateY({virtualRow.start}px);"
            >
              {#each row.getVisibleCells() as cell}
                <div
                  class="px-2 text-sm truncate"
                  style={cell.column.columnDef.meta?.flex ? `flex: 1 1 ${cell.column.getSize()}px; min-width: ${cell.column.getSize()}px` : `flex: 0 0 ${cell.column.getSize()}px`}
                >
                  {#if cell.column.id === 'fileName'}
                    <span class="flex items-center gap-1 font-mono">
                      <OpenFolderButton path={original.filePath} size="xs" title="ファイルのフォルダを開く" />
                      <span class="truncate" title={original.filePath}>{original.fileName}</span>
                    </span>
                  {:else if cell.column.id === 'destFolder'}
                    {#if original.destFolder}
                      <span class="flex items-center gap-1">
                        <OpenFolderButton path={original.destFolder} size="xs" title="推定先フォルダを開く" />
                        <span class="truncate text-success" title={original.destFolder}>{original.destFolder}</span>
                      </span>
                    {:else}
                      <span class="text-base-content/30">-</span>
                    {/if}
                  {:else if cell.column.id === 'score'}
                    <span class="font-mono">
                      {#if original.score > 0}
                        {Math.round(original.score * 10)}
                      {:else}
                        -
                      {/if}
                    </span>
                  {:else if cell.column.id === 'actions'}
                    <div class="flex gap-1">
                      <button
                        class="btn btn-xs btn-ghost"
                        title="推定先をクリア"
                        disabled={!original.destFolder}
                        on:click|stopPropagation={() => clearDestFolder(original.filePath)}
                      >
                        <Icon name="close" cls="h-3 w-3" />
                      </button>
                      <button
                        class="btn btn-xs btn-ghost text-error"
                        title="削除"
                        on:click|stopPropagation={() => clearCandidate(original.filePath)}
                      >
                        <Icon name="trash" cls="h-3 w-3" />
                      </button>
                    </div>
                  {:else}
                    <svelte:component
                      this={flexRender(cell.column.columnDef.cell, cell.getContext())}
                    />
                  {/if}
                </div>
              {/each}
            </div>
          {/each}
        </div>
      </div>
    </div>
```

- [ ] **Step 5: clearCandidate/clearDestFolderをfilePath基準に変更**

57-68行のclearCandidate/clearDestFolder関数を以下に変更:

```typescript
  function clearCandidate(filePath: string) {
    candidates = candidates.filter(c => c.filePath !== filePath)
  }

  function clearDestFolder(filePath: string) {
    candidates = candidates.map(c => {
      if (c.filePath === filePath) {
        return { ...c, destFolder: '' }
      }
      return c
    })
  }
```

- [ ] **Step 6: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: ビルド成功

- [ ] **Step 7: コミット**

```bash
git add frontend/src/views/DiffImportView.svelte
git commit -m "feat: DiffImportViewをtanstack table + 仮想スクロール + カラムリサイズに移行"
```

---

### Task 3: 動作確認

- [ ] **Step 1: アプリを起動して動作確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails dev`

確認項目:
1. 差分導入画面にファイルをドラッグ＆ドロップして候補が表示される
2. 各カラムのヘッダー右端にマウスを合わせるとリサイズハンドル（青線）が表示される
3. ドラッグでカラム幅を変更できる
4. 操作カラム（最右端）はリサイズ不可
5. 推定先クリア・削除ボタンが動作する
6. 導入ボタンが動作する
7. 空状態のドロップゾーンが正常に表示される
8. 推定中のプログレスバーが正常に動作する
9. 既存テーブル（楽曲一覧、譜面一覧）のヘッダーにリサイズハンドルが表示されないこと（enableColumnResizing未設定のため）
