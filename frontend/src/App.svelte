<script lang="ts">
  import SongTable from './SongTable.svelte'

  let selectedFolderHash: string | null = null

  function handleSelect(e: CustomEvent<string>) {
    selectedFolderHash = e.detail
  }
</script>

<div data-theme="emerald" class="h-full flex flex-col">
  <div class="navbar bg-base-200 px-4 shrink-0">
    <div class="flex-1">
      <span class="text-xl font-bold">BMS ELSA</span>
    </div>
  </div>

  <div class="flex-1 overflow-hidden p-4 flex gap-4">
    <div class="flex-1 overflow-hidden">
      <SongTable on:select={handleSelect} />
    </div>
    {#if selectedFolderHash}
      <div class="w-96 overflow-auto">
        <!-- SongDetail は Task 11 で追加予定 -->
        <div class="p-4 bg-base-200 rounded-lg">
          <p class="text-sm text-base-content/50">Selected: {selectedFolderHash}</p>
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  :global(body) {
    margin: 0;
  }
</style>
