<script lang="ts">
  import {
    createSvelteTable,
    flexRender,
    getCoreRowModel,
    getSortedRowModel,
    getFilteredRowModel,
    type ColumnDef,
    type SortingState,
    type FilterFn,
  } from '@tanstack/svelte-table'
  import { createVirtualizer } from '@tanstack/svelte-virtual'
  import { onMount, createEventDispatcher } from 'svelte'
  import { ListAllSongs } from '../wailsjs/go/app/SongHandler'
  import type { dto } from '../wailsjs/go/models'
  import SearchInput from './SearchInput.svelte'
  import SortableHeader from './SortableHeader.svelte'
  import InferenceModal from './InferenceModal.svelte'
  import { EventsOn } from '../wailsjs/runtime/runtime'
  import { StartBulkFetch, StopBulkFetch } from '../wailsjs/go/app/IRHandler'

  let inferenceModal: InferenceModal

  // IR一括取得の状態
  let irFetching = false
  let irProgress = { current: 0, total: 0 }
  let irDoneMessage = ''
  let irDoneTimer: ReturnType<typeof setTimeout> | null = null

  function startBulkFetch() {
    irFetching = true
    irProgress = { current: 0, total: 0 }
    irDoneMessage = ''
    if (irDoneTimer) { clearTimeout(irDoneTimer); irDoneTimer = null }
    StartBulkFetch().catch((e: Error) => {
      console.error('StartBulkFetch failed:', e)
      irFetching = false
    })
  }

  function stopBulkFetch() {
    StopBulkFetch()
  }

  const dispatch = createEventDispatcher<{ select: string; deselect: void }>()

  export let selected: string | null = null

  const ROW_HEIGHT = 32

  let songs: dto.SongRowDTO[] = []
  let loading = true
  let globalFilter = ''

  const searchFilter: FilterFn<dto.SongRowDTO> = (row, _columnId, filterValue) => {
    const s = (filterValue as string).toLowerCase()
    const item = row.original
    return (
      item.title.toLowerCase().includes(s) ||
      item.artist.toLowerCase().includes(s) ||
      item.genre.toLowerCase().includes(s) ||
      (item.eventName || '').toLowerCase().includes(s)
    )
  }

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

  $: table = createSvelteTable({
    data: songs,
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

  onMount(() => {
    const offProgress = EventsOn('ir:progress', (data: { current: number; total: number }) => {
      irProgress = data
    })
    const offDone = EventsOn('ir:done', (data: { fetched: number; notFound: number; failed: number; cancelled: boolean }) => {
      irFetching = false
      const parts: string[] = []
      if (data.fetched > 0) parts.push(`${data.fetched}件取得`)
      if (data.notFound > 0) parts.push(`${data.notFound}件未登録`)
      if (data.failed > 0) parts.push(`${data.failed}件失敗`)
      if (data.cancelled) parts.push('中断')
      irDoneMessage = parts.join(', ')
      irDoneTimer = setTimeout(() => { irDoneMessage = '' }, 5000)
      // 楽曲リスト再読み込み
      ListAllSongs().then(s => { songs = s || [] }).catch(console.error)
    })

    // 既存の楽曲リスト読み込み
    ListAllSongs().then(s => { songs = s || [] }).catch(e => {
      console.error('Failed to load songs:', e)
    }).finally(() => { loading = false })

    return () => {
      offProgress()
      offDone()
      if (irDoneTimer) clearTimeout(irDoneTimer)
    }
  })
</script>

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div
  class="h-full flex flex-col bg-base-100 rounded-lg border border-base-300"
  on:click={() => dispatch('deselect')}
>
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="px-4 py-2 bg-base-200 rounded-t-lg flex items-center justify-between gap-2">
    <span class="text-sm font-semibold shrink-0">
      {#if loading}Loading...{:else}{rows.length.toLocaleString()} songs{/if}
    </span>
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
      <button class="btn btn-xs btn-outline" on:click|stopPropagation={() => inferenceModal.open()}>メタ推測</button>
      <SearchInput bind:value={globalFilter} />
    </div>
  </div>

  {#if loading}
    <div class="flex items-center justify-center flex-1">
      <span class="loading loading-spinner"></span>
    </div>
  {:else}
    <SortableHeader table={$table} />

    <!-- 仮想スクロール領域 -->
    <div
      bind:this={scrollElement}
      class="flex-1 overflow-auto"
      role="grid"
      tabindex="-1"
      on:keydown={(e) => { if (e.key === 'Escape') dispatch('deselect') }}
    >
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
    </div>
  {/if}
</div>
<InferenceModal bind:this={inferenceModal} />
