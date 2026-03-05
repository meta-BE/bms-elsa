<script lang="ts">
  import { flexRender, type Table, type Column } from '@tanstack/svelte-table'

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  export let table: Table<any>

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  function isFilterColumn(column: Column<any, unknown>): boolean {
    const meta = column.columnDef.meta as { filterType?: string } | undefined
    return meta?.filterType === 'select'
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  function getFilterOptions(column: Column<any, unknown>): string[] {
    const meta = column.columnDef.meta as { filterOptions?: string[] } | undefined
    if (meta?.filterOptions) return meta.filterOptions
    try {
      const values = column.getFacetedUniqueValues()
      return Array.from(values.keys())
        .filter((v) => v != null && v !== '')
        .map(String)
        .sort()
    } catch {
      return []
    }
  }
</script>

<div class="bg-base-200 border-b border-base-300 px-2 shrink-0">
  {#each table.getHeaderGroups() as headerGroup}
    <div class="flex">
      {#each headerGroup.headers as header}
        {#if isFilterColumn(header.column)}
          <div
            class="px-1 py-1 text-xs truncate"
            style="width: {header.getSize()}px; min-width: {header.getSize()}px"
          >
            <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
            <select
              class="select select-xs w-full min-h-0 h-6 font-bold uppercase"
              value={String(header.column.getFilterValue() ?? '')}
              on:change={(e) => {
                const val = e.currentTarget.value
                header.column.setFilterValue(val || undefined)
              }}
              on:click|stopPropagation
            >
              <option value="">{header.column.columnDef.header}</option>
              {#each getFilterOptions(header.column) as opt}
                <option value={opt}>{opt}</option>
              {/each}
            </select>
          </div>
        {:else}
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
        {/if}
      {/each}
    </div>
  {/each}
</div>
