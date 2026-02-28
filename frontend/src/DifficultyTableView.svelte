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
  import { ListDifficultyTables, ListDifficultyTableEntries } from '../wailsjs/go/main/App'
  import type { main } from '../wailsjs/go/models'

  const dispatch = createEventDispatcher<{
    select: { md5: string; entry: main.DifficultyTableEntryDTO }
    deselect: void
  }>()

  const ROW_HEIGHT = 32

  let tables: main.DifficultyTableDTO[] = []
  let selectedTableId: number | null = null
  let entries: main.DifficultyTableEntryDTO[] = []
  let loading = false
  let loadingEntries = false
  let selectedMD5: string | null = null

  const columns: ColumnDef<main.DifficultyTableEntryDTO>[] = [
    { accessorKey: 'level', header: 'Level', size: 80 },
    { accessorKey: 'title', header: 'Title', size: 300 },
    { accessorKey: 'artist', header: 'Artist', size: 200 },
    {
      id: 'hasUrl',
      header: 'URL',
      size: 60,
      accessorFn: (row) => row.url ? '○' : '',
    },
    {
      id: 'statusLabel',
      header: 'Status',
      size: 100,
      accessorFn: (row) => {
        if (row.status === 'installed') return '導入済'
        if (row.status === 'not_installed') return '未導入'
        if (row.status === 'duplicate') return '重複'
        return row.status
      },
    },
  ]

  let sorting: SortingState = []

  const options = writable<TableOptions<main.DifficultyTableEntryDTO>>({
    data: entries,
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
    loading = true
    try {
      tables = (await ListDifficultyTables()) || []
      if (tables.length > 0) {
        selectedTableId = tables[0].id
        await loadEntries(tables[0].id)
      }
    } catch (e) {
      console.error('Failed to load difficulty tables:', e)
    } finally {
      loading = false
    }
  })

  async function loadEntries(tableId: number) {
    loadingEntries = true
    selectedMD5 = null
    try {
      entries = (await ListDifficultyTableEntries(tableId)) || []
    } catch (e) {
      console.error('Failed to load entries:', e)
      entries = []
    } finally {
      loadingEntries = false
    }
    options.update((o) => ({ ...o, data: entries }))
  }

  async function handleTableChange(e: Event) {
    const target = e.target as HTMLSelectElement
    const id = Number(target.value)
    selectedTableId = id
    dispatch('deselect')
    await loadEntries(id)
  }

  function rowBgClass(status: string, md5: string): string {
    if (md5 === selectedMD5) return 'bg-primary/20'
    if (status === 'not_installed') return 'bg-base-300/50'
    if (status === 'duplicate') return 'bg-warning/20'
    return ''
  }

  function handleRowClick(entry: main.DifficultyTableEntryDTO) {
    selectedMD5 = entry.md5
    dispatch('select', { md5: entry.md5, entry })
  }
</script>

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div
  class="h-full flex flex-col bg-base-100 rounded-lg border border-base-300"
  on:click={() => dispatch('deselect')}
>
  <div class="px-4 py-2 bg-base-200 rounded-t-lg flex items-center justify-between gap-2">
    {#if loading}
      <span class="text-sm font-semibold">Loading...</span>
    {:else if tables.length === 0}
      <span class="text-sm text-base-content/50">Settings画面から難易度表を追加してください</span>
    {:else}
      <select
        class="select select-bordered select-sm"
        value={selectedTableId}
        on:change={handleTableChange}
      >
        {#each tables as t}
          <option value={t.id}>{t.symbol} {t.name} ({t.entryCount})</option>
        {/each}
      </select>
      <span class="text-sm font-semibold">{entries.length} entries</span>
    {/if}
  </div>

  {#if tables.length > 0}
    <!-- ヘッダー -->
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
      {#if loadingEntries}
        <div class="flex items-center justify-center h-32">
          <span class="loading loading-spinner loading-md"></span>
        </div>
      {:else if entries.length === 0}
        <div class="flex items-center justify-center h-32">
          <span class="text-sm text-base-content/50">エントリがありません。更新してください</span>
        </div>
      {:else}
        <div style="height: {totalSize}px; position: relative;">
          {#each virtualItems as virtualRow (virtualRow.index)}
            {@const row = rows[virtualRow.index]}
            <div
              role="row"
              tabindex="0"
              class="flex absolute w-full hover:bg-base-200 border-b border-base-300/50 items-center px-2 cursor-pointer {rowBgClass(row.original.status, row.original.md5)}"
              style="height: {virtualRow.size}px; transform: translateY({virtualRow.start}px);"
              on:click|stopPropagation={() => handleRowClick(row.original)}
              on:keydown|stopPropagation={(e) => { if (e.key === 'Enter' || e.key === ' ') handleRowClick(row.original) }}
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
  {/if}
</div>
