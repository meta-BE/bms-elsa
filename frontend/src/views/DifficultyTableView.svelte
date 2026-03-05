<script lang="ts">
  import {
    createSvelteTable,
    flexRender,
    getCoreRowModel,
    getSortedRowModel,
    getFilteredRowModel,
    type ColumnDef,
    type SortingState,
    type TableOptions,
  } from '@tanstack/svelte-table'
  import { createVirtualizer } from '@tanstack/svelte-virtual'
  import { writable } from 'svelte/store'
  import { onMount, onDestroy, createEventDispatcher } from 'svelte'
  import { ListDifficultyTables, ListDifficultyTableEntries } from '../../wailsjs/go/main/App'
  import type { main } from '../../wailsjs/go/models'
  import SearchInput from '../components/SearchInput.svelte'
  import SortableHeader from '../components/SortableHeader.svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime'
  import { StartDifficultyTableBulkFetch, StopBulkFetch } from '../../wailsjs/go/app/IRHandler'
  import { handleArrowNav } from '../utils/arrowNav'

  const dispatch = createEventDispatcher<{
    select: { md5: string; tableID: number }
    deselect: void
  }>()

  const ROW_HEIGHT = 32

  let tables: main.DifficultyTableDTO[] = []
  let selectedTableId: number | null = null
  let entries: main.DifficultyTableEntryDTO[] = []
  let loading = false
  let loadingEntries = false
  export let selected: string | null = null
  export let active = true
  let searchText = ''

  // IR一括取得の状態
  let irFetching = false
  let irProgress = { current: 0, total: 0 }
  let irDoneMessage = ''
  let irDoneTimer: ReturnType<typeof setTimeout> | null = null

  function startBulkFetch() {
    if (!selectedTableId) return
    irFetching = true
    irProgress = { current: 0, total: 0 }
    irDoneMessage = ''
    if (irDoneTimer) { clearTimeout(irDoneTimer); irDoneTimer = null }
    StartDifficultyTableBulkFetch(selectedTableId).catch((e: Error) => {
      console.error('[IR] StartDifficultyTableBulkFetch failed:', e)
      irFetching = false
    })
  }

  function stopBulkFetch() {
    StopBulkFetch()
  }

  const columns: ColumnDef<main.DifficultyTableEntryDTO>[] = [
    {
      accessorKey: 'level',
      header: 'Level',
      size: 80,
      sortingFn: (rowA, rowB, columnId) => {
        const a = Number(rowA.getValue(columnId)) || 0
        const b = Number(rowB.getValue(columnId)) || 0
        return a - b
      },
    },
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
      enableSorting: false,
      filterFn: 'equalsString',
      meta: { filterType: 'select', filterOptions: ['導入済', '未導入', '重複'] },
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
    getFilteredRowModel: getFilteredRowModel(),
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

  let offProgress: (() => void) | null = null
  let offDone: (() => void) | null = null

  onMount(async () => {
    offProgress = EventsOn('ir:progress', (data: { current: number; total: number }) => {
      irProgress = data
    })
    offDone = EventsOn('ir:done', (data: { total: number; fetched: number; notFound: number; failed: number; cancelled: boolean }) => {
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
    })

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

  onDestroy(() => {
    offProgress?.()
    offDone?.()
    if (irDoneTimer) clearTimeout(irDoneTimer)
  })

  async function loadEntries(tableId: number) {
    loadingEntries = true
    try {
      entries = (await ListDifficultyTableEntries(tableId)) || []
    } catch (e) {
      console.error('Failed to load entries:', e)
      entries = []
    } finally {
      loadingEntries = false
    }
    applyFilter()
  }

  function applyFilter() {
    const filtered = searchText
      ? entries.filter(e => {
          const s = searchText.toLowerCase()
          return e.title.toLowerCase().includes(s) || e.artist.toLowerCase().includes(s)
        })
      : entries
    options.update((o) => ({ ...o, data: filtered }))
  }

  async function handleTableChange(e: Event) {
    const target = e.target as HTMLSelectElement
    const id = Number(target.value)
    selectedTableId = id
    searchText = ''
    dispatch('deselect')
    await loadEntries(id)
  }

  function statusBgClass(status: string): string {
    if (status === 'not_installed') return 'bg-base-200 hover:bg-base-300'
    if (status === 'duplicate') return 'bg-warning/20'
    return ''
  }

  function handleKeyNav(e: KeyboardEvent) {
    if (!active || !selectedTableId) return
    handleArrowNav(e, {
      selected,
      rows,
      getKey: (o: main.DifficultyTableEntryDTO) => o.md5,
      onSelect: (o: main.DifficultyTableEntryDTO) => dispatch('select', { md5: o.md5, tableID: selectedTableId! }),
      scrollToIndex: (i: number) => $virtualizer.scrollToIndex(i, { align: 'auto' }),
    })
  }

  function handleRowClick(entry: main.DifficultyTableEntryDTO) {
    if (selected === entry.md5) {
      dispatch('deselect')
    } else {
      dispatch('select', { md5: entry.md5, tableID: selectedTableId! })
    }
  }
</script>

<svelte:window on:keydown={handleKeyNav} />

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div
  class="h-full flex flex-col bg-base-100 rounded-lg border border-base-300"
  on:click={() => dispatch('deselect')}
>
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="px-4 py-2 bg-base-200 rounded-t-lg flex items-center justify-between gap-2">
    {#if loading}
      <span class="text-sm font-semibold">Loading...</span>
    {:else if tables.length === 0}
      <span class="text-sm text-base-content/50">Settings画面から難易度表を追加してください</span>
    {:else}
      <div class="flex items-center gap-2">
        <span class="text-sm font-semibold shrink-0">{rows.length.toLocaleString()} charts</span>
        <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
        <select
          class="select select-bordered select-sm"
          value={selectedTableId}
          on:change={handleTableChange}
          on:click|stopPropagation
        >
          {#each tables as t}
            <option value={t.id}>{t.symbol} / {t.name} ({t.entryCount})</option>
          {/each}
        </select>
      </div>
      <div class="flex items-center gap-2">
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
        <SearchInput bind:value={searchText} on:input={applyFilter} on:clear={applyFilter} />
      </div>
    {/if}
  </div>

  {#if tables.length > 0}
    <SortableHeader table={$table} />

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
              class="flex absolute w-full border-b border-base-300/50 items-center px-2 cursor-pointer {selected === row.original.md5 ? 'bg-primary/20' : statusBgClass(row.original.status) + ' hover:bg-base-200'}"
              style="height: {virtualRow.size}px; transform: translateY({virtualRow.start}px);"
              on:click|stopPropagation={() => handleRowClick(row.original)}
              on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') handleRowClick(row.original) }}
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
