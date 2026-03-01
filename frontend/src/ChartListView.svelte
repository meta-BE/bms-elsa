<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte'
  import {
    createSvelteTable,
    getCoreRowModel,
    getSortedRowModel,
    getFilteredRowModel,
    flexRender,
    type ColumnDef,
    type SortingState,
    type FilterFn,
  } from '@tanstack/svelte-table'
  import { createVirtualizer } from '@tanstack/svelte-virtual'
  import { ListCharts } from '../wailsjs/go/main/App'
  import type { dto } from '../wailsjs/go/models'
  import SearchInput from './SearchInput.svelte'
  import SortableHeader from './SortableHeader.svelte'

  const dispatch = createEventDispatcher<{
    select: { md5: string }
    deselect: void
  }>()

  let charts: dto.ChartListItemDTO[] = []
  let loading = false
  export let selected: string | null = null
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

  const ROW_HEIGHT = 48
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
    state: { sorting, globalFilter },
    onSortingChange: (updater) => {
      sorting = typeof updater === 'function' ? updater(sorting) : updater
    },
    globalFilterFn: searchFilter,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
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
    if (selected === chart.md5) {
      dispatch('deselect')
    } else {
      dispatch('select', { md5: chart.md5 })
    }
  }
</script>

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div class="h-full flex flex-col bg-base-100 rounded-lg border border-base-300" on:click={() => dispatch('deselect')}>
  <!-- ヘッダー -->
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="px-4 py-2 bg-base-200 rounded-t-lg flex items-center justify-between gap-2">
    <span class="text-sm font-semibold shrink-0">
      {rows.length.toLocaleString()} charts
    </span>
    <SearchInput bind:value={globalFilter} />
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
            class="flex absolute w-full items-center px-2 text-xs cursor-pointer transition-colors
              {selected === row.original.md5 ? 'bg-primary/20' : 'hover:bg-base-200'}"
            style="height: {ROW_HEIGHT}px; transform: translateY({virtualRow.start}px);"
            on:click|stopPropagation={() => handleRowClick(row.original)}
            on:keydown|stopPropagation={(e) => { if (e.key === 'Enter' || e.key === ' ') handleRowClick(row.original) }}
          >
            {#each row.getVisibleCells() as cell}
              <div
                class="px-2 truncate"
                style="width: {cell.column.getSize()}px; min-width: {cell.column.getSize()}px"
              >
                {#if cell.column.id === 'title'}
                  <div class="truncate">{cell.row.original.title}</div>
                  <div class="truncate text-[10px] text-base-content/70">{cell.row.original.subtitle || ''}</div>
                {:else if cell.column.id === 'artist'}
                  <div class="truncate">{cell.row.original.artist}</div>
                  <div class="truncate text-[10px] text-base-content/70">{cell.row.original.subArtist || ''}</div>
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
