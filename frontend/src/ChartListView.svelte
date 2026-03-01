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
  <div class="px-4 py-2 bg-base-200 rounded-t-lg flex items-center justify-between gap-2" on:click|stopPropagation>
    <span class="text-sm text-base-content/70 shrink-0">
      {rows.length} 譜面
    </span>
    <div class="relative">
      <input
        type="text"
        placeholder="検索..."
        class="input input-xs input-bordered w-48 pr-6"
        bind:value={globalFilter}
      />
      {#if globalFilter}
        <button class="absolute right-1 top-1/2 -translate-y-1/2 btn btn-ghost btn-xs btn-circle h-4 w-4 min-h-0 p-0"
          on:click={() => { globalFilter = '' }}>✕</button>
      {/if}
    </div>
  </div>

  {#if loading}
    <div class="flex items-center justify-center flex-1">
      <span class="loading loading-spinner"></span>
    </div>
  {:else}
    <!-- テーブルヘッダー -->
    <div class="bg-base-200 border-b border-base-300 px-2 shrink-0">
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
          {/each}
        </div>
      {/each}
    </div>

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
