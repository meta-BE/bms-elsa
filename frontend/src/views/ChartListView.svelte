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
  import { InferWorkingURLs } from '../../wailsjs/go/app/RewriteHandler'
  import { handleArrowNav } from '../utils/arrowNav'

  const dispatch = createEventDispatcher<{
    select: { md5: string }
    deselect: void
  }>()

  let charts: dto.ChartListItemDTO[] = []
  let loading = false

  // IR一括取得の状態
  let irFetching = false
  let irProgress = { current: 0, total: 0 }
  let irDoneMessage = ''
  let irDoneTimer: ReturnType<typeof setTimeout> | null = null

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

  function startBulkFetch() {
    console.log('[IR] startBulkFetch called')
    irFetching = true
    irProgress = { current: 0, total: 0 }
    irDoneMessage = ''
    if (irDoneTimer) { clearTimeout(irDoneTimer); irDoneTimer = null }
    StartBulkFetch().then(() => {
      console.log('[IR] StartBulkFetch resolved')
    }).catch((e: Error) => {
      console.error('[IR] StartBulkFetch failed:', e)
      irFetching = false
    })
  }

  function stopBulkFetch() {
    StopBulkFetch()
  }
  export let selected: string | null = null
  export let active = true
  let scrollElement: HTMLDivElement
  let sorting: SortingState = []
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

  const ROW_HEIGHT = 52
  const columns: ColumnDef<dto.ChartListItemDTO>[] = [
    { accessorKey: 'title', header: 'Title', size: 300 },
    { accessorKey: 'artist', header: 'Artist', size: 200 },
    { accessorKey: 'genre', header: 'Genre', size: 140 },
    {
      id: 'notes',
      header: 'Notes',
      size: 80,
      accessorFn: (row) => row.notes || 0,
    },
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
      id: 'eventName',
      header: 'Event',
      size: 140,
      accessorFn: (row) => row.eventName || '',
      enableSorting: false,
      filterFn: 'equalsString',
      meta: { filterType: 'select' },
    },
    {
      id: 'releaseYear',
      header: 'Year',
      size: 60,
      accessorFn: (row) => row.releaseYear ? String(row.releaseYear) : '',
      enableSorting: false,
      filterFn: 'equalsString',
      meta: { filterType: 'select', filterSort: 'desc' },
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

  $: rows = $table.getRowModel().rows

  $: virtualizer = createVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: rows.length,
    getScrollElement: () => scrollElement,
    estimateSize: () => ROW_HEIGHT,
    overscan: 20,
  })

  $: virtualItems = $virtualizer.getVirtualItems()
  $: totalSize = $virtualizer.getTotalSize()

  let offProgress: (() => void) | null = null
  let offDone: (() => void) | null = null

  onMount(() => {
    offProgress = EventsOn('ir:progress', (data: { current: number; total: number }) => {
      console.log('[IR] progress:', data)
      irProgress = data
    })
    offDone = EventsOn('ir:done', (data: { total: number; fetched: number; notFound: number; failed: number; cancelled: boolean }) => {
      console.log('[IR] done:', data)
      irFetching = false
      const parts: string[] = []
      if (data.total === 0) {
        irDoneMessage = '対象なし'
      } else {
        if (data.fetched > 0) parts.push(`${data.fetched}件取得`)
        if (data.notFound > 0) parts.push(`${data.notFound}件未登録`)
        if (data.failed > 0) parts.push(`${data.failed}件失敗`)
        if (data.cancelled) parts.push('中断')
        irDoneMessage = parts.join(', ') || '完了'
      }
      irDoneTimer = setTimeout(() => { irDoneMessage = '' }, 5000)
      // 譜面リスト再読み込み
      ListCharts().then(c => { charts = c || [] }).catch(console.error)
    })

    // 譜面リスト読み込み
    loading = true
    ListCharts().then(c => { charts = c || [] }).catch(e => {
      console.error('Failed to load charts:', e)
    }).finally(() => { loading = false })
  })

  onDestroy(() => {
    offProgress?.()
    offDone?.()
    if (irDoneTimer) clearTimeout(irDoneTimer)
  })

  function handleKeyNav(e: KeyboardEvent) {
    if (!active) return
    handleArrowNav(e, {
      selected,
      rows,
      getKey: (o: dto.ChartListItemDTO) => o.md5,
      onSelect: (o: dto.ChartListItemDTO) => dispatch('select', { md5: o.md5 }),
      scrollToIndex: (i: number) => $virtualizer.scrollToIndex(i, { align: 'auto' }),
    })
  }

  function handleRowClick(chart: dto.ChartListItemDTO) {
    if (selected === chart.md5) {
      dispatch('deselect')
    } else {
      dispatch('select', { md5: chart.md5 })
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
      {#if irFetching}
        <span class="text-xs text-base-content/70">
          取得中: {irProgress.current.toLocaleString()} / {irProgress.total.toLocaleString()}
        </span>
        <button class="btn btn-xs btn-error btn-outline" on:click|stopPropagation={stopBulkFetch}>停止</button>
      {:else if irDoneMessage}
        <span class="text-xs text-success">{irDoneMessage}</span>
      {:else}
        <button class="btn btn-xs btn-outline" on:click|stopPropagation={startBulkFetch}>IR取得</button>
      {/if}
      <SearchInput bind:value={globalFilter} />
    </div>
  </div>

  {#if loading}
    <div class="flex items-center justify-center flex-1">
      <span class="loading loading-spinner"></span>
    </div>
  {:else}
    <SortableHeader table={$table} />

    <!-- 仮想スクロール本体 -->
    <div class="flex-1 overflow-auto" bind:this={scrollElement}>
      <div style="height: {totalSize}px; position: relative;">
        {#each virtualItems as virtualRow (virtualRow.index)}
          {@const row = rows[virtualRow.index]}
          <div
            role="row"
            tabindex="0"
            class="flex absolute w-full items-center px-2 text-sm cursor-pointer transition-colors
              {selected === row.original.md5 ? 'bg-primary/20' : 'hover:bg-base-200'}"
            style="height: {ROW_HEIGHT}px; transform: translateY({virtualRow.start}px);"
            on:click|stopPropagation={() => handleRowClick(row.original)}
            on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') handleRowClick(row.original) }}
          >
            {#each row.getVisibleCells() as cell}
              <div
                class="px-2 truncate"
                style="width: {cell.column.getSize()}px; min-width: {cell.column.getSize()}px"
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
