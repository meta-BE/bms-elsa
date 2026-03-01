<script lang="ts">
  import { onMount, createEventDispatcher } from 'svelte'
  import {
    createSvelteTable,
    getCoreRowModel,
    getSortedRowModel,
    flexRender,
    type ColumnDef,
    type SortingState,
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
  let selectedMD5: string | null = null
  let scrollElement: HTMLDivElement
  let sorting: SortingState = []

  const ROW_HEIGHT = 32
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
      accessorKey: 'difficulty',
      header: '★',
      size: 80,
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
    state: { sorting },
    onSortingChange: (updater) => {
      sorting = typeof updater === 'function' ? updater(sorting) : updater
    },
    getCoreRowModel: getCoreRowModel(),
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
    if (selectedMD5 === chart.md5) {
      selectedMD5 = null
      dispatch('deselect')
    } else {
      selectedMD5 = chart.md5
      dispatch('select', { md5: chart.md5 })
    }
  }
</script>

<div class="flex flex-col h-full">
  <!-- ヘッダー -->
  <div class="flex items-center gap-2 px-4 py-2 bg-base-100 shrink-0">
    <span class="text-sm text-base-content/70">
      {charts.length} 譜面
    </span>
  </div>

  {#if loading}
    <div class="flex items-center justify-center flex-1">
      <span class="loading loading-spinner"></span>
    </div>
  {:else}
    <!-- テーブルヘッダー -->
    <div class="shrink-0">
      <table class="table table-xs w-full">
        <thead>
          {#each $table.getHeaderGroups() as headerGroup}
            <tr>
              {#each headerGroup.headers as header}
                <th
                  style="width: {header.getSize()}px"
                  class="cursor-pointer select-none hover:bg-base-200"
                  on:click={header.column.getToggleSortingHandler()}
                >
                  <div class="flex items-center gap-1">
                    <svelte:component
                      this={flexRender(header.column.columnDef.header, header.getContext())}
                    />
                    {#if header.column.getIsSorted() === 'asc'}
                      <span class="text-xs">▲</span>
                    {:else if header.column.getIsSorted() === 'desc'}
                      <span class="text-xs">▼</span>
                    {/if}
                  </div>
                </th>
              {/each}
            </tr>
          {/each}
        </thead>
      </table>
    </div>

    <!-- 仮想スクロール本体 -->
    <div class="flex-1 overflow-auto" bind:this={scrollElement}>
      <div style="height: {totalSize}px; width: 100%; position: relative;">
        {#each virtualItems as virtualRow (virtualRow.index)}
          {@const row = rows[virtualRow.index]}
          <div
            class="absolute w-full flex items-center text-xs cursor-pointer transition-colors
              {selectedMD5 === row.original.md5 ? 'bg-primary/20' : 'hover:bg-base-200'}"
            style="height: {ROW_HEIGHT}px; transform: translateY({virtualRow.start}px);"
            on:click={() => handleRowClick(row.original)}
            role="row"
            tabindex="0"
          >
            {#each row.getVisibleCells() as cell}
              <div
                class="truncate px-2"
                style="width: {cell.column.getSize()}px"
              >
                <svelte:component
                  this={flexRender(cell.column.columnDef.cell, cell.getContext())}
                />
              </div>
            {/each}
          </div>
        {/each}
      </div>
    </div>
  {/if}
</div>
