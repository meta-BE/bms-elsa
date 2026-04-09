<script lang="ts">
  import { onMount } from 'svelte'
  import { flexRender, type Table, type Column } from '@tanstack/svelte-table'
  import Icon from './Icon.svelte'

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  export let table: Table<any>

  // リサイズが有効かつcolumnSizingが設定済みの場合、flex伸縮を無効化して固定幅に切り替え
  $: resizeLocked = table.options.enableColumnResizing && Object.keys(table.getState().columnSizing).length > 0

  let scrollbarWidth = 0
  onMount(() => {
    const outer = document.createElement('div')
    outer.style.cssText = 'overflow:scroll;width:100px;height:100px;position:absolute;top:-9999px'
    document.body.appendChild(outer)
    scrollbarWidth = outer.offsetWidth - outer.clientWidth
    document.body.removeChild(outer)
  })

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  function isFilterColumn(column: Column<any, unknown>): boolean {
    const meta = column.columnDef.meta as { filterType?: string } | undefined
    return meta?.filterType === 'select'
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  function getFilterOptions(column: Column<any, unknown>): string[] {
    const meta = column.columnDef.meta as { filterOptions?: string[]; filterSort?: 'asc' | 'desc' } | undefined
    if (meta?.filterOptions) return meta.filterOptions
    try {
      const values = column.getFacetedUniqueValues()
      const opts = Array.from(values.keys())
        .filter((v) => v != null && v !== '')
        .map(String)
      return meta?.filterSort === 'desc' ? opts.sort().reverse() : opts.sort()
    } catch {
      return []
    }
  }

  let openFilterColumnId: string | null = null

  function toggleFilterMenu(columnId: string) {
    openFilterColumnId = openFilterColumnId === columnId ? null : columnId
  }

  function closeFilterMenu() {
    openFilterColumnId = null
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  function selectFilterValue(column: Column<any, unknown>, value: string | undefined) {
    column.setFilterValue(value)
    openFilterColumnId = null
  }

  // カスタムリサイズ: ハンドルを境界として左右カラムの幅を同時に調整（合計幅不変）
  let resizingColumnId: string | null = null
  const MIN_COL_WIDTH = 40

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  function canShowResizeHandle(headerIndex: number, headers: any[]): boolean {
    const current = headers[headerIndex]
    if (!current.column.getCanResize()) return false
    const next = headers[headerIndex + 1]
    return !!next && next.column.getCanResize()
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  function startResize(e: MouseEvent | TouchEvent, headerIndex: number, headers: any[]) {
    const leftHeader = headers[headerIndex]
    const rightHeader = headers[headerIndex + 1]
    if (!rightHeader) return

    const startX = 'touches' in e ? e.touches[0].clientX : e.clientX
    const leftInitial = leftHeader.getSize()
    const rightInitial = rightHeader.getSize()

    resizingColumnId = leftHeader.column.id

    function onMove(ev: MouseEvent | TouchEvent) {
      const clientX = 'touches' in ev ? ev.touches[0].clientX : (ev as MouseEvent).clientX
      const rawDelta = clientX - startX
      const delta = Math.max(-(leftInitial - MIN_COL_WIDTH), Math.min(rawDelta, rightInitial - MIN_COL_WIDTH))

      table.setColumnSizing((prev: Record<string, number>) => ({
        ...prev,
        [leftHeader.column.id]: leftInitial + delta,
        [rightHeader.column.id]: rightInitial - delta,
      }))
    }

    function onEnd() {
      resizingColumnId = null
      document.removeEventListener('mousemove', onMove as EventListener)
      document.removeEventListener('mouseup', onEnd)
      document.removeEventListener('touchmove', onMove as EventListener)
      document.removeEventListener('touchend', onEnd)
    }

    document.addEventListener('mousemove', onMove as EventListener)
    document.addEventListener('mouseup', onEnd)
    document.addEventListener('touchmove', onMove as EventListener)
    document.addEventListener('touchend', onEnd)
  }
</script>

<svelte:window on:click={closeFilterMenu} />

<div class="bg-base-200 border-b border-base-300 px-2 shrink-0" style="padding-right: {scrollbarWidth + 8}px">
  {#each table.getHeaderGroups() as headerGroup}
    <div class="flex">
      {#each headerGroup.headers as header, headerIndex}
        {#if isFilterColumn(header.column)}
          <!-- フィルタヘッダー -->
          <div
            class="relative min-w-0"
            style={resizeLocked || !header.column.columnDef.meta?.flex ? `flex: 0 0 ${header.getSize()}px` : `flex: 1 1 ${header.getSize()}px; min-width: ${header.getSize()}px`}
          >
            <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
            <div
              class="px-2 py-1.5 text-xs font-bold uppercase cursor-pointer select-none hover:bg-base-300 transition-colors truncate"
              on:click|stopPropagation={() => toggleFilterMenu(header.column.id)}
            >
              <span class="flex items-center gap-1">
                {header.column.columnDef.header}
                {#if header.column.getFilterValue()}
                  <Icon name="filter" cls="h-3 w-3" />
                {/if}
              </span>
            </div>
            {#if openFilterColumnId === header.column.id}
              <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
              <div
                class="absolute top-full left-0 z-50 bg-base-100 border border-base-300 rounded shadow-lg py-1 min-w-[120px] max-h-[300px] overflow-auto"
                on:click|stopPropagation
              >
                <button
                  class="w-full text-left px-3 py-1 text-xs hover:bg-base-200 transition-colors"
                  class:font-bold={!header.column.getFilterValue()}
                  on:click={() => selectFilterValue(header.column, undefined)}
                >すべて</button>
                {#each getFilterOptions(header.column) as opt}
                  <button
                    class="w-full text-left px-3 py-1 text-xs hover:bg-base-200 transition-colors"
                    class:font-bold={header.column.getFilterValue() === opt}
                    on:click={() => selectFilterValue(header.column, opt)}
                  >{opt}</button>
                {/each}
              </div>
            {/if}
            {#if canShowResizeHandle(headerIndex, headerGroup.headers)}
              <!-- svelte-ignore a11y-no-static-element-interactions -->
              <div
                class="absolute top-0 right-0 w-1 h-full cursor-col-resize select-none touch-none
                  {resizingColumnId === header.column.id ? 'bg-primary' : 'hover:bg-primary/50'}"
                on:mousedown|stopPropagation={(e) => startResize(e, headerIndex, headerGroup.headers)}
                on:touchstart|stopPropagation={(e) => startResize(e, headerIndex, headerGroup.headers)}
              />
            {/if}
          </div>
        {:else}
          <!-- ソートヘッダー -->
          <div
            class="relative min-w-0"
            style={resizeLocked || !header.column.columnDef.meta?.flex ? `flex: 0 0 ${header.getSize()}px` : `flex: 1 1 ${header.getSize()}px; min-width: ${header.getSize()}px`}
          >
            <div
              role="columnheader"
              tabindex="0"
              class="px-2 py-1.5 text-xs font-bold uppercase cursor-pointer select-none hover:bg-base-300 transition-colors truncate"
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
            {#if canShowResizeHandle(headerIndex, headerGroup.headers)}
              <!-- svelte-ignore a11y-no-static-element-interactions -->
              <div
                class="absolute top-0 right-0 w-1 h-full cursor-col-resize select-none touch-none
                  {resizingColumnId === header.column.id ? 'bg-primary' : 'hover:bg-primary/50'}"
                on:mousedown|stopPropagation={(e) => startResize(e, headerIndex, headerGroup.headers)}
                on:touchstart|stopPropagation={(e) => startResize(e, headerIndex, headerGroup.headers)}
              />
            {/if}
          </div>
        {/if}
      {/each}
    </div>
  {/each}
</div>
