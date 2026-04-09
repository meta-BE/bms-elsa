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

  let candidates: app.DiffImportCandidateDTO[] = []
  let estimating = false
  let estimateProgress = { current: 0, total: 0 }
  let importResult: app.DiffImportResultDTO | null = null
  let importResultTimer: ReturnType<typeof setTimeout> | null = null
  let importing = false
  let dragging = false

  let offProgress: (() => void) | null = null
  let offDone: (() => void) | null = null

  export function handleFileDrop(paths: string[]) {
    dragging = false
    if (estimating) return
    handleDrop(paths)
  }

  onMount(() => {
    offProgress = EventsOn('diff-import:progress', (data: { current: number; total: number }) => {
      estimateProgress = data
    })

    offDone = EventsOn('diff-import:done', () => {
      estimating = false
    })
  })

  onDestroy(() => {
    offProgress?.()
    offDone?.()
    if (importResultTimer) clearTimeout(importResultTimer)
  })

  async function handleDrop(filePaths: string[]) {
    estimating = true
    estimateProgress = { current: 0, total: 0 }
    importResult = null
    try {
      const result = await ParseAndEstimate(filePaths)
      const existingPaths = new Set(candidates.map(c => c.filePath))
      candidates = [...candidates, ...(result || []).filter(c => !existingPaths.has(c.filePath))]
    } catch (e: any) {
      console.error('ParseAndEstimate failed:', e)
    } finally {
      estimating = false
    }
  }

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

  function clearAll() {
    candidates = []
    importResult = null
  }

  async function handleImport() {
    const toImport = candidates.filter(c => c.destFolder)
    if (toImport.length === 0) return

    importing = true
    importResult = null
    try {
      const result = await ExecuteImport(toImport)
      importResult = result
      // 導入成功分をリストから除去
      const importedPaths = new Set(toImport.map(c => c.filePath))
      candidates = candidates.filter(c => !importedPaths.has(c.filePath))
    } catch (e: any) {
      console.error('ExecuteImport failed:', e)
    } finally {
      importing = false
    }
  }

  function handleStopEstimate() {
    StopEstimate()
  }

  const matchMethodLabels: Record<string, string> = {
    minhash: 'WAV定義',
    ir: '本体URL',
    title: 'タイトル',
  }

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

  $: importableCount = candidates.filter(c => c.destFolder).length

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
</script>

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div
  class="h-full flex flex-col bg-base-100 rounded-lg border border-base-300 {dragging ? 'border-primary border-2' : ''}"
  on:dragover|preventDefault={() => { dragging = true }}
  on:dragleave|preventDefault={() => { dragging = false }}
  on:drop|preventDefault={() => { dragging = false }}
>
  <!-- ヘッダー -->
  <div class="px-4 py-2 bg-base-200 rounded-t-lg flex items-center justify-between gap-2">
    <span class="text-sm font-semibold shrink-0">
      {candidates.length} ファイル
      {#if importableCount > 0}
        ({importableCount} 件導入可能)
      {/if}
    </span>
    <div class="flex items-center gap-2" style:width={estimating ? '50%' : null}>
      {#if estimating}
        <ProgressBar current={estimateProgress.current} total={estimateProgress.total} cancelable on:cancel={handleStopEstimate} />
      {/if}
      {#if importResult}
        <span class="text-xs text-success">
          導入完了: {importResult.success}件成功
          {#if importResult.failed > 0}
            , {importResult.failed}件失敗
          {/if}
        </span>
      {/if}
      {#if candidates.length > 0}
        <button class="btn btn-xs btn-ghost" on:click|stopPropagation={clearAll}>全クリア</button>
      {/if}
    </div>
  </div>

  {#if candidates.length === 0 && !estimating}
    <!-- 空状態 -->
    <div class="flex-1 flex items-center justify-center text-base-content/40">
      <div class="text-center">
        <Icon name="cloudUpload" cls="h-12 w-12 mx-auto mb-2 opacity-30" />
        <p class="text-sm">BMS/BME/BMLファイルをドラッグ＆ドロップして差分を追加</p>
      </div>
    </div>
  {:else}
    <!-- テーブル -->
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

    <!-- フッター -->
    <div class="px-4 py-2 border-t border-base-300 flex items-center justify-between">
      <div class="text-xs text-base-content/60">
        {#if importResult && importResult.errors && importResult.errors.length > 0}
          <details>
            <summary class="cursor-pointer text-error">エラー詳細 ({importResult.errors.length}件)</summary>
            <ul class="mt-1 ml-4 list-disc">
              {#each importResult.errors as err}
                <li>{err}</li>
              {/each}
            </ul>
          </details>
        {/if}
      </div>
      <button
        class="btn btn-sm btn-primary"
        disabled={importableCount === 0 || importing || estimating}
        on:click|stopPropagation={handleImport}
      >
        {#if importing}
          導入中...
        {:else}
          推定先に導入 ({importableCount})
        {/if}
      </button>
    </div>
  {/if}
</div>
