<script lang="ts">
  import {
    createSvelteTable,
    flexRender,
    getCoreRowModel,
    getSortedRowModel,
    type ColumnDef,
    type SortingState,
    type TableOptions,
  } from '@tanstack/svelte-table'
  import { createVirtualizer } from '@tanstack/svelte-virtual'
  import { writable } from 'svelte/store'
  import { generateDummySongs, type Song } from './dummy'

  const ROW_HEIGHT = 32
  const SONG_COUNT = 5000

  const data = generateDummySongs(SONG_COUNT)

  const columns: ColumnDef<Song>[] = [
    { accessorKey: 'title', header: 'Title', size: 280 },
    { accessorKey: 'artist', header: 'Artist', size: 180 },
    { accessorKey: 'genre', header: 'Genre', size: 140 },
    { accessorKey: 'bpm', header: 'BPM', size: 70 },
    { accessorKey: 'playLevel', header: 'Lv', size: 50 },
    {
      accessorKey: 'difficulty',
      header: 'Diff',
      size: 80,
      cell: (info) => {
        const labels = ['', 'BEGINNER', 'NORMAL', 'HYPER', 'ANOTHER', 'INSANE']
        return labels[info.getValue() as number] ?? ''
      },
    },
    { accessorKey: 'chartCount', header: 'Charts', size: 70 },
  ]

  let sorting: SortingState = []

  const options = writable<TableOptions<Song>>({
    data,
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
  })

  const table = createSvelteTable(options)

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

<div class="h-full flex flex-col bg-base-100 rounded-lg border border-base-300">
  <div class="px-4 py-2 bg-base-200 rounded-t-lg flex items-center justify-between">
    <span class="text-sm font-semibold">{rows.length.toLocaleString()} songs</span>
  </div>

  <!-- ヘッダー（スクロールしない） -->
  <div class="bg-base-200 border-b border-base-300 px-2">
    {#each $table.getHeaderGroups() as headerGroup}
      <div class="flex">
        {#each headerGroup.headers as header}
          <div
            class="px-2 py-1.5 text-xs font-bold uppercase cursor-pointer select-none hover:bg-base-300 transition-colors truncate"
            style="width: {header.getSize()}px; min-width: {header.getSize()}px"
            on:click={header.column.getToggleSortingHandler()}
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

  <!-- 仮想スクロール領域 -->
  <div
    bind:this={scrollElement}
    class="flex-1 overflow-auto"
  >
    <div style="height: {totalSize}px; position: relative;">
      {#each virtualItems as virtualRow (virtualRow.index)}
        {@const row = rows[virtualRow.index]}
        <div
          class="flex absolute w-full hover:bg-base-200 border-b border-base-300/50 items-center px-2"
          style="height: {virtualRow.size}px; transform: translateY({virtualRow.start}px);"
        >
          {#each row.getVisibleCells() as cell}
            <div
              class="px-2 text-sm truncate"
              style="width: {cell.column.getSize()}px; min-width: {cell.column.getSize()}px"
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
</div>
