<script lang="ts">
  import SongTable from './SongTable.svelte'
</script>

<div data-theme="emerald" class="h-full flex flex-col">
  <div class="navbar bg-base-200 px-4 shrink-0">
    <div class="flex-1">
      <span class="text-xl font-bold">BMS ELSA</span>
    </div>
    <div class="flex-none">
      <button class="btn btn-primary btn-sm">Scan</button>
    </div>
  </div>

  <div class="flex-1 overflow-hidden p-4">
    <SongTable />
  </div>
</div>

<style>
  :global(body) {
    margin: 0;
  }
</style>
