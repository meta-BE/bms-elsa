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

  const ROW_HEIGHT = 36
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
      size: 60,
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

  $: virtualizer = createVirtualizer<HTMLDivElement, HTMLTableRowElement>({
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

  <div
    bind:this={scrollElement}
    class="flex-1 overflow-auto"
  >
    <table class="table table-xs table-pin-rows w-full">
      <thead>
        {#each $table.getHeaderGroups() as headerGroup}
          <tr>
            {#each headerGroup.headers as header}
              <th
                class="cursor-pointer select-none hover:bg-base-300 transition-colors"
                style="width: {header.getSize()}px"
                on:click={header.column.getToggleSortingHandler()}
              >
                <div class="flex items-center gap-1">
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
                </div>
              </th>
            {/each}
          </tr>
        {/each}
      </thead>
      <tbody style="height: {totalSize}px; position: relative;">
        {#each virtualItems as virtualRow (virtualRow.index)}
          {@const row = rows[virtualRow.index]}
          <tr
            class="hover absolute w-full"
            style="height: {virtualRow.size}px; transform: translateY({virtualRow.start}px);"
          >
            {#each row.getVisibleCells() as cell}
              <td
                class="truncate"
                style="width: {cell.column.getSize()}px; max-width: {cell.column.getSize()}px"
              >
                <svelte:component
                  this={flexRender(cell.column.columnDef.cell, cell.getContext())}
                />
              </td>
            {/each}
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>
