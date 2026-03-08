<script lang="ts">
  import { onMount } from 'svelte'
  import { flexRender, type Table, type Column } from '@tanstack/svelte-table'

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  export let table: Table<any>

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
</script>

<svelte:window on:click={closeFilterMenu} />

<div class="bg-base-200 border-b border-base-300 px-2 shrink-0" style="padding-right: {scrollbarWidth + 8}px">
  {#each table.getHeaderGroups() as headerGroup}
    <div class="flex">
      {#each headerGroup.headers as header}
        {#if isFilterColumn(header.column)}
          <!-- フィルタヘッダー -->
          <div
            class="relative"
            style={header.column.columnDef.meta?.flex ? `flex: 1 1 ${header.getSize()}px; min-width: ${header.getSize()}px` : `flex: 0 0 ${header.getSize()}px`}
          >
            <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
            <div
              class="px-2 py-1.5 text-xs font-bold uppercase cursor-pointer select-none hover:bg-base-300 transition-colors truncate"
              on:click|stopPropagation={() => toggleFilterMenu(header.column.id)}
            >
              <span class="flex items-center gap-1">
                {header.column.columnDef.header}
                {#if header.column.getFilterValue()}
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" viewBox="0 0 20 20" fill="currentColor">
                    <path fill-rule="evenodd" d="M3 3a1 1 0 011-1h12a1 1 0 011 1v3a1 1 0 01-.293.707L12 11.414V15a1 1 0 01-.293.707l-2 2A1 1 0 018 17v-5.586L3.293 6.707A1 1 0 013 6V3z" clip-rule="evenodd" />
                  </svg>
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
          </div>
        {:else}
          <!-- ソートヘッダー -->
          <div
            role="columnheader"
            tabindex="0"
            class="px-2 py-1.5 text-xs font-bold uppercase cursor-pointer select-none hover:bg-base-300 transition-colors truncate"
            style={header.column.columnDef.meta?.flex ? `flex: 1 1 ${header.getSize()}px; min-width: ${header.getSize()}px` : `flex: 0 0 ${header.getSize()}px`}
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
        {/if}
      {/each}
    </div>
  {/each}
</div>
