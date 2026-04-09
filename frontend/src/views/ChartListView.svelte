<script lang="ts">
  import { onMount, afterUpdate, onDestroy, createEventDispatcher } from 'svelte'
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
    type ColumnSizingState,
    type ColumnSizingInfoState,
  } from '@tanstack/svelte-table'
  import { createVirtualizer } from '@tanstack/svelte-virtual'
  import { ListCharts } from '../../wailsjs/go/app/ChartHandler'
  import type { dto } from '../../wailsjs/go/models'
  import SearchInput from '../components/SearchInput.svelte'
  import SortableHeader from '../components/SortableHeader.svelte'
  import { StartBulkFetch, StopBulkFetch } from '../../wailsjs/go/app/IRHandler'
  import BulkFetchButton from '../components/BulkFetchButton.svelte'
  import { handleArrowNav } from '../utils/arrowNav'
  import Icon from '../components/Icon.svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime'
  import {
    loadColumnWidths,
    saveColumnWidths,
    recalcFromRatios,
    toRatios,
    type ViewId,
  } from '../utils/columnResize'

  const dispatch = createEventDispatcher<{
    select: { md5: string; folderHash: string }
    deselect: void
  }>()

  let charts: dto.ChartListItemDTO[] = []
  let loading = false

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
      id: 'eventName',
      header: 'Event',
      size: 140,
      accessorFn: (row) => row.eventName || '',
      enableSorting: false,
      filterFn: 'equalsString',
      meta: { flex: true, filterType: 'select' },
    },
    {
      id: 'releaseYear',
      header: 'Year',
      size: 60,
      enableResizing: false,
      accessorFn: (row) => row.releaseYear ? String(row.releaseYear) : '',
      enableSorting: false,
      filterFn: 'equalsString',
      meta: { filterType: 'select', filterSort: 'desc' },
    },
    {
      id: 'bpm',
      header: 'BPM',
      size: 100,
      enableResizing: false,
      accessorFn: (row) => row.minBpm,
      cell: (info) => {
        const row = info.row.original
        if (row.minBpm === row.maxBpm) return String(Math.round(row.minBpm))
        return `${Math.round(row.minBpm)}-${Math.round(row.maxBpm)}`
      },
    },
    {
      id: 'notes',
      header: 'Notes',
      size: 80,
      enableResizing: false,
      meta: { align: 'right' },
      accessorFn: (row) => row.notes || 0,
    },
    {
      id: 'ir',
      header: 'IR',
      size: 40,
      enableResizing: false,
      meta: { align: 'center' },
      accessorFn: (row) => row.hasIrMeta ? '●' : '',
    },
  ]

  const VIEW_ID: ViewId = 'chartList'
  const RESIZABLE_IDS = columns
    .filter(c => (c.meta as { flex?: boolean })?.flex)
    .map(c => c.id || (c as { accessorKey?: string }).accessorKey || '')
  const FIXED_WIDTH = columns
    .filter(c => !(c.meta as { flex?: boolean })?.flex)
    .reduce((sum, c) => sum + (c.size || 150), 0)
  const CONTAINER_PADDING = 16

  let columnSizing: ColumnSizingState = {}
  let columnSizingInfo: ColumnSizingInfoState = {} as ColumnSizingInfoState
  let widthsLocked = false
  let currentRatios: Record<string, number> = {}
  let offResetWidths: (() => void) | null = null

  $: table = createSvelteTable({
    data: charts,
    columns,
    enableColumnResizing: true,
    columnResizeMode: 'onChange',
    state: { sorting, globalFilter, columnSizing, columnSizingInfo },
    onSortingChange: (updater) => {
      sorting = typeof updater === 'function' ? updater(sorting) : updater
    },
    onColumnSizingChange: (updater) => {
      columnSizing = typeof updater === 'function' ? updater(columnSizing) : updater
    },
    onColumnSizingInfoChange: (updater) => {
      columnSizingInfo = typeof updater === 'function' ? updater(columnSizingInfo) : updater
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

  onMount(() => {
    // 譜面リスト読み込み
    loading = true
    ListCharts().then(c => { charts = c || [] }).catch(e => {
      console.error('Failed to load charts:', e)
    }).finally(() => { loading = false })

    offResetWidths = EventsOn('column-width-reset', (id: string) => {
      if (id !== VIEW_ID) return
      columnSizing = {}
      currentRatios = {}
      widthsLocked = false
    })
  })

  onDestroy(() => {
    offResetWidths?.()
  })

  afterUpdate(() => {
    if (charts.length > 0 && scrollElement && !widthsLocked) {
      widthsLocked = true
      requestAnimationFrame(async () => {
        const containerWidth = scrollElement.clientWidth - CONTAINER_PADDING
        const restored = await loadColumnWidths(
          { viewId: VIEW_ID, resizableColumnIds: RESIZABLE_IDS, fixedColumnsWidth: FIXED_WIDTH },
          containerWidth,
        )
        if (restored) {
          columnSizing = restored
          currentRatios = toRatios(restored, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
          return
        }
        const flexCols = columns.filter(c => (c.meta as { flex?: boolean })?.flex)
        const flexDefined = flexCols.reduce((sum, c) => sum + (c.size || 150), 0)
        const available = Math.max(0, containerWidth - FIXED_WIDTH)
        const newSizing: Record<string, number> = {}
        for (const col of flexCols) {
          const id = col.id || (col as { accessorKey?: string }).accessorKey || ''
          newSizing[id] = Math.round(((col.size || 150) / flexDefined) * available)
        }
        columnSizing = newSizing
        currentRatios = toRatios(newSizing, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
      })
    }
  })

  function handleResizeEnd() {
    if (!scrollElement) return
    const containerWidth = scrollElement.clientWidth - CONTAINER_PADDING
    currentRatios = toRatios(columnSizing, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
    saveColumnWidths(VIEW_ID, columnSizing, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
  }

  function handleWindowResize() {
    if (!scrollElement || !widthsLocked || Object.keys(currentRatios).length === 0) return
    const containerWidth = scrollElement.clientWidth - CONTAINER_PADDING
    columnSizing = recalcFromRatios(currentRatios, FIXED_WIDTH, containerWidth)
  }

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

<svelte:window on:keydown={handleKeyNav} on:resize={handleWindowResize} />

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div class="h-full flex flex-col bg-base-100 rounded-lg border border-base-300" on:click={() => dispatch('deselect')}>
  <!-- ヘッダー -->
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="px-4 py-2 bg-base-200 rounded-t-lg flex items-center justify-between gap-2">
    <span class="text-sm font-semibold shrink-0">
      {rows.length.toLocaleString()} charts
    </span>
    <div class="flex items-center gap-2">
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
    <SortableHeader table={$table} onResizeEnd={handleResizeEnd} />

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
                style={widthsLocked || !cell.column.columnDef.meta?.flex ? `flex: 0 0 ${cell.column.getSize()}px` : `flex: 1 1 ${cell.column.getSize()}px; min-width: ${cell.column.getSize()}px`}
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
