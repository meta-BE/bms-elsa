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
  import { onMount, createEventDispatcher } from 'svelte'
  import { ListSongs } from '../wailsjs/go/app/SongHandler'
  import type { dto } from '../wailsjs/go/models'

  const dispatch = createEventDispatcher<{ select: string; deselect: void }>()

  export let selected: string | null = null

  const ROW_HEIGHT = 32
  const PAGE_SIZE = 5000

  let data: dto.SongRowDTO[] = []
  let totalCount = 0
  let loading = true
  let searchText = ''
  let debounceTimer: ReturnType<typeof setTimeout>

  const columns: ColumnDef<dto.SongRowDTO>[] = [
    { accessorKey: 'title', header: 'Title', size: 300 },
    { accessorKey: 'artist', header: 'Artist', size: 200 },
    { accessorKey: 'genre', header: 'Genre', size: 140 },
    {
      id: 'bpm',
      header: 'BPM',
      size: 100,
      accessorFn: (row) => {
        if (row.minBpm === row.maxBpm) return String(Math.round(row.minBpm))
        return `${Math.round(row.minBpm)}-${Math.round(row.maxBpm)}`
      },
    },
    { accessorKey: 'eventName', header: 'Event', size: 140 },
    { accessorKey: 'releaseYear', header: 'Year', size: 60 },
    {
      id: 'ir',
      header: 'IR',
      size: 40,
      accessorFn: (row) => row.hasIrMeta ? '●' : '',
    },
    { accessorKey: 'chartCount', header: 'Charts', size: 60 },
  ]

  let sorting: SortingState = []

  const options = writable<TableOptions<dto.SongRowDTO>>({
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

  onMount(async () => {
    try {
      const result = await ListSongs(1, PAGE_SIZE, 'title', false, '')
      data = result.songs || []
      totalCount = result.totalCount
    } catch (e) {
      console.error('Failed to load songs:', e)
      data = []
    } finally {
      loading = false
    }
    options.update((o) => ({ ...o, data }))
  })

  async function doSearch() {
    loading = true
    try {
      const result = await ListSongs(1, PAGE_SIZE, 'title', false, searchText)
      data = result.songs || []
      totalCount = result.totalCount
    } catch (e) {
      console.error('Failed to search songs:', e)
      data = []
    } finally {
      loading = false
    }
    options.update((o) => ({ ...o, data }))
  }

  function handleSearchInput() {
    clearTimeout(debounceTimer)
    debounceTimer = setTimeout(doSearch, 300)
  }
</script>

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div
  class="h-full flex flex-col bg-base-100 rounded-lg border border-base-300"
  on:click={() => dispatch('deselect')}
>
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="px-4 py-2 bg-base-200 rounded-t-lg flex items-center justify-between gap-2" on:click|stopPropagation>
    <span class="text-sm font-semibold shrink-0">
      {#if loading}Loading...{:else}{totalCount.toLocaleString()} songs{/if}
    </span>
    <div class="relative">
      <input
        type="text"
        placeholder="検索..."
        class="input input-xs input-bordered w-48 pr-6"
        bind:value={searchText}
        on:input={handleSearchInput}
      />
      {#if searchText}
        <button class="absolute right-1 top-1/2 -translate-y-1/2 btn btn-ghost btn-xs btn-circle h-4 w-4 min-h-0 p-0"
          on:click={() => { searchText = ''; doSearch() }}>✕</button>
      {/if}
    </div>
  </div>

  <!-- ヘッダー（スクロールしない） -->
  <div class="bg-base-200 border-b border-base-300 px-2">
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

  <!-- 仮想スクロール領域 -->
  <div
    bind:this={scrollElement}
    class="flex-1 overflow-auto"
    role="grid"
    tabindex="-1"
    on:keydown={(e) => { if (e.key === 'Escape') dispatch('deselect') }}
  >
    {#if loading}
      <div class="flex items-center justify-center h-32">
        <span class="loading loading-spinner loading-md"></span>
      </div>
    {:else}
      <div style="height: {totalSize}px; position: relative;">
        {#each virtualItems as virtualRow (virtualRow.index)}
          {@const row = rows[virtualRow.index]}
          <div
            role="row"
            tabindex="0"
            class="flex absolute w-full border-b border-base-300/50 items-center px-2 cursor-pointer
              {selected === row.original.folderHash ? 'bg-primary/20' : 'hover:bg-base-200'}"
            style="height: {virtualRow.size}px; transform: translateY({virtualRow.start}px);"
            on:click|stopPropagation={() => dispatch('select', row.original.folderHash)}
            on:keydown|stopPropagation={(e) => { if (e.key === 'Enter' || e.key === ' ') dispatch('select', row.original.folderHash) }}
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
    {/if}
  </div>
</div>
