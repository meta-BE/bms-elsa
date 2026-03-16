<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte'
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
  import { createVirtualizer } from '@tanstack/svelte-virtual'
  import { ListCharts } from '../../wailsjs/go/app/ChartHandler'
  import type { dto } from '../../wailsjs/go/models'
  import SearchInput from '../components/SearchInput.svelte'
  import SortableHeader from '../components/SortableHeader.svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime'
  import { StartBulkFetch, StopBulkFetch } from '../../wailsjs/go/app/IRHandler'
  import { StartMinHashScan, StopMinHashScan } from '../../wailsjs/go/app/ScanHandler'
  import BulkFetchButton from '../components/BulkFetchButton.svelte'
  import { InferWorkingURLs } from '../../wailsjs/go/app/RewriteHandler'
  import { handleArrowNav } from '../utils/arrowNav'
  import Icon from '../components/Icon.svelte'

  const dispatch = createEventDispatcher<{
    select: { md5: string; folderHash: string }
    deselect: void
  }>()

  let charts: dto.ChartListItemDTO[] = []
  let loading = false

  // MinHashスキャンの状態
  let scanRunning = false
  let scanProgress = { current: 0, total: 0 }
  let scanDoneMessage = ''
  let scanDoneTimer: ReturnType<typeof setTimeout> | null = null

  let inferringUrls = false
  let inferUrlResult = ''

  async function runInferWorkingURLs() {
    inferringUrls = true
    inferUrlResult = ''
    try {
      const result = await InferWorkingURLs()
      inferUrlResult = `${result.applied}件適用 / ${result.skipped}件スキップ / ${result.total}件中`
      setTimeout(() => inferUrlResult = '', 5000)
      ListCharts().then(c => { charts = c || [] }).catch(console.error)
    } catch (e: any) {
      inferUrlResult = e?.message || '推定に失敗しました'
    } finally {
      inferringUrls = false
    }
  }

  function startMinHashScan() {
    scanRunning = true
    scanProgress = { current: 0, total: 0 }
    scanDoneMessage = ''
    if (scanDoneTimer) { clearTimeout(scanDoneTimer); scanDoneTimer = null }
    StartMinHashScan().catch((e: Error) => {
      console.error('[Scan] StartMinHashScan failed:', e)
      scanRunning = false
    })
  }

  function stopMinHashScan() {
    StopMinHashScan()
  }

  export let selected: string | null = null
  export let active = true
  let scrollElement: HTMLDivElement
  let sorting: SortingState = []
  let globalFilter = ''
  let pathSearch = false

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

  const ROW_HEIGHT = 52
  const columns: ColumnDef<dto.ChartListItemDTO>[] = [
    { accessorKey: 'title', header: 'Title', size: 300, meta: { flex: true } },
    { accessorKey: 'artist', header: 'Artist', size: 200, meta: { flex: true } },
    { accessorKey: 'genre', header: 'Genre', size: 140, meta: { flex: true } },
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
      id: 'releaseYear',
      header: 'Year',
      size: 60,
      accessorFn: (row) => row.releaseYear ? String(row.releaseYear) : '',
      enableSorting: false,
      filterFn: 'equalsString',
      meta: { filterType: 'select', filterSort: 'desc'},
    },
    {
      id: 'eventName',
      header: 'Event',
      size: 140,
      accessorFn: (row) => row.eventName || '',
      enableSorting: false,
      filterFn: 'equalsString',
      meta: { filterType: 'select' },
    },
    {
      id: 'notes',
      header: 'Notes',
      size: 80,
      meta: { align: 'right' },
      accessorFn: (row) => row.notes || 0,
    },
    {
      id: 'ir',
      header: 'IR',
      size: 40,
      meta: { align: 'center' },
      accessorFn: (row) => row.hasIrMeta ? '●' : '',
    },
  ]

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

  function togglePathSearch() {
    pathSearch = !pathSearch
    if (globalFilter) {
      const tmp = globalFilter
      globalFilter = ''
      globalFilter = tmp
    }
  }

  $: rows = $table.getRowModel().rows

  $: virtualizer = createVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: rows.length,
    getScrollElement: () => scrollElement,
    estimateSize: () => ROW_HEIGHT,
    overscan: 20,
  })

  $: virtualItems = $virtualizer.getVirtualItems()
  $: totalSize = $virtualizer.getTotalSize()

  let offScanProgress: (() => void) | null = null
  let offScanDone: (() => void) | null = null

  onMount(() => {
    offScanProgress = EventsOn('scan:progress', (data: { current: number; total: number }) => {
      scanProgress = data
    })
    offScanDone = EventsOn('scan:done', (data: { total: number; computed: number; skipped: number; failed: number; cancelled: boolean }) => {
      scanRunning = false
      const parts: string[] = []
      if (data.total === 0) {
        scanDoneMessage = '対象なし'
      } else {
        if (data.computed > 0) parts.push(`${data.computed}件計算`)
        if (data.skipped > 0) parts.push(`${data.skipped}件スキップ`)
        if (data.failed > 0) parts.push(`${data.failed}件失敗`)
        if (data.cancelled) parts.push('中断')
        scanDoneMessage = parts.join(', ') || '完了'
      }
      scanDoneTimer = setTimeout(() => { scanDoneMessage = '' }, 5000)
    })

    // 譜面リスト読み込み
    loading = true
    ListCharts().then(c => { charts = c || [] }).catch(e => {
      console.error('Failed to load charts:', e)
    }).finally(() => { loading = false })
  })

  onDestroy(() => {
    offScanProgress?.()
    offScanDone?.()
    if (scanDoneTimer) clearTimeout(scanDoneTimer)
  })

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

  function handleRowClick(chart: dto.ChartListItemDTO) {
    const key = chart.md5 + ':' + chart.folderHash
    if (selected === key) {
      dispatch('deselect')
    } else {
      dispatch('select', { md5: chart.md5, folderHash: chart.folderHash })
    }
  }
</script>

<svelte:window on:keydown={handleKeyNav} />

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div class="h-full flex flex-col bg-base-100 rounded-lg border border-base-300" on:click={() => dispatch('deselect')}>
  <!-- ヘッダー -->
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="px-4 py-2 bg-base-200 rounded-t-lg flex items-center justify-between gap-2">
    <span class="text-sm font-semibold shrink-0">
      {rows.length.toLocaleString()} charts
    </span>
    <div class="flex items-center gap-2">
      {#if inferUrlResult}
        <span class="text-xs text-success">{inferUrlResult}</span>
      {/if}
      <button
        class="btn btn-xs btn-outline"
        on:click|stopPropagation={runInferWorkingURLs}
        disabled={inferringUrls}
      >
        {inferringUrls ? '推定中...' : '動作URL推定'}
      </button>
      {#if scanRunning}
        <span class="text-xs text-base-content/70">
          計算中: {scanProgress.current.toLocaleString()} / {scanProgress.total.toLocaleString()}
        </span>
        <button class="btn btn-xs btn-error btn-outline" on:click|stopPropagation={stopMinHashScan}>停止</button>
      {:else if scanDoneMessage}
        <span class="text-xs text-success">{scanDoneMessage}</span>
      {:else}
        <button class="btn btn-xs btn-outline" on:click|stopPropagation={startMinHashScan}>MinHash計算</button>
      {/if}
      <BulkFetchButton
        startFn={StartBulkFetch}
        stopFn={StopBulkFetch}
        on:done={() => ListCharts().then(c => { charts = c || [] }).catch(console.error)}
      />
      <div class="flex items-center gap-2">
        <SearchInput bind:value={globalFilter} />
        <button
          class="btn btn-ghost btn-xs"
          on:click={togglePathSearch}
          title="検索モード：{pathSearch ? 'フォルダパス' : '楽曲データ'}"
        >
          {#if pathSearch}
            <Icon name="folder" cls="w-4 h-4" />
          {:else}
            <Icon name="tag" cls="w-4 h-4" />
          {/if}
        </button>
      </div>
    </div>
  </div>

  {#if loading}
    <div class="flex items-center justify-center flex-1">
      <span class="loading loading-spinner"></span>
    </div>
  {:else}
    <SortableHeader table={$table} />

    <!-- 仮想スクロール本体 -->
    <div class="flex-1 overflow-y-scroll" bind:this={scrollElement}>
      <div style="height: {totalSize}px; position: relative;">
        {#each virtualItems as virtualRow (virtualRow.index)}
          {@const row = rows[virtualRow.index]}
          <div
            role="row"
            tabindex="0"
            class="flex absolute w-full items-center px-2 text-sm cursor-pointer transition-colors
              {selected === row.original.md5 + ':' + row.original.folderHash ? 'bg-primary/20' : 'hover:bg-base-200'}"
            style="height: {ROW_HEIGHT}px; transform: translateY({virtualRow.start}px);"
            on:click|stopPropagation={() => handleRowClick(row.original)}
            on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') handleRowClick(row.original) }}
          >
            {#each row.getVisibleCells() as cell}
              <div
                class="px-2 truncate {cell.column.columnDef.meta?.align === 'center' ? 'text-center' : cell.column.columnDef.meta?.align === 'right' ? 'text-right' : ''}"
                style={cell.column.columnDef.meta?.flex ? `flex: 1 1 ${cell.column.getSize()}px; min-width: ${cell.column.getSize()}px` : `flex: 0 0 ${cell.column.getSize()}px`}
              >
                {#if cell.column.id === 'title'}
                  <div class="truncate">{cell.row.original.title}</div>
                  <div class="truncate text-xs text-base-content/70">{cell.row.original.subtitle || ''}</div>
                {:else if cell.column.id === 'artist'}
                  <div class="truncate">{cell.row.original.artist}</div>
                  <div class="truncate text-xs text-base-content/70">{cell.row.original.subArtist || ''}</div>
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
  {/if}
</div>
