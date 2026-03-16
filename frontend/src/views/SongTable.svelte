<script lang="ts">
  import {
    createSvelteTable,
    flexRender,
    getCoreRowModel,
    getSortedRowModel,
    getFilteredRowModel,
    getFacetedRowModel,
    getFacetedUniqueValues,
    type ColumnDef,
    type SortingState,
    type FilterFn,
  } from '@tanstack/svelte-table'
  import { createVirtualizer } from '@tanstack/svelte-virtual'
  import { onMount, createEventDispatcher } from 'svelte'
  import { ListAllSongs } from '../../wailsjs/go/app/SongHandler'
  import type { dto } from '../../wailsjs/go/models'
  import SearchInput from '../components/SearchInput.svelte'
  import SortableHeader from '../components/SortableHeader.svelte'
  import InferenceModal from '../settings/InferenceModal.svelte'
  import { InferWorkingURLs } from '../../wailsjs/go/app/RewriteHandler'
  import { handleArrowNav } from '../utils/arrowNav'
  import Icon from '../components/Icon.svelte'

  let inferenceModal: InferenceModal
  let inferringUrls = false
  let inferUrlResult = ''

  const dispatch = createEventDispatcher<{ select: string; deselect: void }>()

  export let selected: string | null = null
  export let active = true
  export let movedHashes: Set<string> = new Set()

  const ROW_HEIGHT = 32

  let songs: dto.SongRowDTO[] = []
  let loading = true
  let globalFilter = ''
  let pathSearch = false

  const searchFilter: FilterFn<dto.SongRowDTO> = (row, _columnId, filterValue) => {
    const s = (filterValue as string).toLowerCase()
    const item = row.original
    if (pathSearch) {
      return (item.path || '').toLowerCase().includes(s)
    }
    return (
      item.title.toLowerCase().includes(s) ||
      item.artist.toLowerCase().includes(s) ||
      item.genre.toLowerCase().includes(s) ||
      (item.eventName || '').toLowerCase().includes(s)
    )
  }

  const columns: ColumnDef<dto.SongRowDTO>[] = [
    { accessorKey: 'title', header: 'Title', size: 300, meta: { flex: true } },
    { accessorKey: 'artist', header: 'Artist', size: 200, meta: { flex: true } },
    { accessorKey: 'genre', header: 'Genre', size: 140, meta: { flex: true } },
    {
      id: 'bpm',
      header: 'BPM',
      size: 100,
      accessorFn: (row) => {
        if (row.minBpm === row.maxBpm) return String(Math.round(row.minBpm))
        return `${Math.round(row.minBpm)}-${Math.round(row.maxBpm)}`
      },
    },
    {
      id: 'releaseYear',
      header: 'Year',
      size: 60,
      accessorFn: (row) => row.releaseYear ? String(row.releaseYear) : '',
      enableSorting: false,
      filterFn: 'equalsString',
      meta: { filterType: 'select', filterSort: 'desc'},
    },
    {
      id: 'eventName',
      header: 'Event',
      size: 140,
      accessorFn: (row) => row.eventName || '',
      enableSorting: false,
      filterFn: 'equalsString',
      meta: { filterType: 'select' },
    },
    { accessorKey: 'chartCount', header: 'Charts', size: 80, meta: { align: 'right' } },
    {
      id: 'ir',
      header: 'IR',
      size: 40,
      meta: { align: 'center' },
      accessorFn: (row) => row.hasIrMeta ? '●' : '',
    },
  ]

  let sorting: SortingState = []

  function togglePathSearch() {
    pathSearch = !pathSearch
    if (globalFilter) {
      const tmp = globalFilter
      globalFilter = ''
      globalFilter = tmp
    }
  }

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
    getFacetedRowModel: getFacetedRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
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

  async function loadSongs() {
    try {
      songs = (await ListAllSongs()) || []
    } catch (e) {
      console.error('Failed to load songs:', e)
    } finally {
      loading = false
    }
  }

  function handleKeyNav(e: KeyboardEvent) {
    if (!active) return
    handleArrowNav(e, {
      selected,
      items: rows.map(r => r.original),
      getKey: (o: dto.SongRowDTO) => o.folderHash,
      onSelect: (o: dto.SongRowDTO) => dispatch('select', o.folderHash),
      scrollToIndex: (i: number) => $virtualizer.scrollToIndex(i, { align: 'auto' }),
    })
  }

  async function runInferWorkingURLs() {
    inferringUrls = true
    inferUrlResult = ''
    try {
      const result = await InferWorkingURLs()
      inferUrlResult = `${result.applied}件適用 / ${result.skipped}件スキップ / ${result.total}件中`
      setTimeout(() => inferUrlResult = '', 5000)
      loadSongs()
    } catch (e: any) {
      inferUrlResult = e?.message || '推定に失敗しました'
    } finally {
      inferringUrls = false
    }
  }

  onMount(() => { loadSongs() })
</script>

<svelte:window on:keydown={handleKeyNav} />

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
      {#if inferUrlResult}
        <span class="text-xs text-success">{inferUrlResult}</span>
      {/if}
      <button
        class="btn btn-xs btn-outline"
        on:click|stopPropagation={runInferWorkingURLs}
        disabled={inferringUrls}
      >
        {inferringUrls ? '推定中...' : '動作URL推定'}
      </button>
      <button class="btn btn-xs btn-outline" on:click|stopPropagation={() => inferenceModal.open()}>メタ推測</button>
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
    <SortableHeader table={$table} />

    <!-- 仮想スクロール領域 -->
    <div
      bind:this={scrollElement}
      class="flex-1 overflow-y-scroll"
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
              {selected === row.original.folderHash ? 'bg-primary/20' : movedHashes.has(row.original.folderHash) ? 'bg-warning/20' : 'hover:bg-base-200'}"
            style="height: {virtualRow.size}px; transform: translateY({virtualRow.start}px);"
            on:click|stopPropagation={() => dispatch('select', row.original.folderHash)}
            on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') dispatch('select', row.original.folderHash) }}
          >
            {#if movedHashes.has(row.original.folderHash)}
              <span class="badge badge-warning badge-xs shrink-0">移動済み</span>
            {/if}
            {#each row.getVisibleCells() as cell}
              <div
                class="px-2 text-sm truncate {cell.column.columnDef.meta?.align === 'center' ? 'text-center' : cell.column.columnDef.meta?.align === 'right' ? 'text-right' : ''}"
                style={cell.column.columnDef.meta?.flex ? `flex: 1 1 ${cell.column.getSize()}px; min-width: ${cell.column.getSize()}px` : `flex: 0 0 ${cell.column.getSize()}px`}
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
<InferenceModal bind:this={inferenceModal} on:close={loadSongs} />
