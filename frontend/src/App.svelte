<script lang="ts">
  import SongTable from './SongTable.svelte'
  import SongDetail from './SongDetail.svelte'

  let selectedFolderHash: string | null = null
  let containerEl: HTMLDivElement
  let dragging = false
  let splitRatio = 0.6

  function handleSelect(e: CustomEvent<string>) {
    selectedFolderHash = e.detail
  }

  function handleDeselect() {
    selectedFolderHash = null
  }

  function handleClose() {
    selectedFolderHash = null
  }

  function onDragStart(e: MouseEvent) {
    e.preventDefault()
    dragging = true
    window.addEventListener('mousemove', onDragMove)
    window.addEventListener('mouseup', onDragEnd)
  }

  function onDragMove(e: MouseEvent) {
    if (!dragging || !containerEl) return
    const rect = containerEl.getBoundingClientRect()
    splitRatio = Math.max(0.2, Math.min(0.8, (e.clientY - rect.top) / rect.height))
  }

  function onDragEnd() {
    dragging = false
    window.removeEventListener('mousemove', onDragMove)
    window.removeEventListener('mouseup', onDragEnd)
  }
</script>

<div data-theme="emerald" class="h-full flex flex-col">
  <div class="navbar bg-base-200 px-4 shrink-0">
    <div class="flex-1">
      <span class="text-xl font-bold">BMS ELSA</span>
    </div>
  </div>

  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div bind:this={containerEl} class="flex-1 overflow-hidden p-4 flex flex-col" on:click={handleDeselect}>
    <div class="overflow-hidden" style="flex: {selectedFolderHash ? splitRatio : 1}">
      <SongTable on:select={handleSelect} on:deselect={handleDeselect} />
    </div>
    {#if selectedFolderHash}
      <!-- svelte-ignore a11y-no-noninteractive-tabindex a11y-no-noninteractive-element-interactions -->
      <div
        class="h-1 shrink-0 cursor-row-resize bg-base-300 hover:bg-primary/30 transition-colors my-1 rounded"
        on:mousedown={onDragStart}
        role="separator"
        tabindex="0"
      ></div>
      <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
      <div class="overflow-y-auto" style="flex: {1 - splitRatio}" on:click|stopPropagation>
        <SongDetail folderHash={selectedFolderHash} on:close={handleClose} />
      </div>
    {/if}
  </div>
</div>

<style>
  :global(body) {
    margin: 0;
  }
</style>
