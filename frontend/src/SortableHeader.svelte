<script lang="ts">
  import { flexRender, type Table } from '@tanstack/svelte-table'

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  export let table: Table<any>
</script>

<div class="bg-base-200 border-b border-base-300 px-2 shrink-0">
  {#each table.getHeaderGroups() as headerGroup}
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
