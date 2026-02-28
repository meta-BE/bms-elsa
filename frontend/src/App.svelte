<script lang="ts">
  import SongTable from './SongTable.svelte'
  import SongDetail from './SongDetail.svelte'
  import DifficultyTableView from './DifficultyTableView.svelte'
  import ChartDetail from './ChartDetail.svelte'
  import Settings from './Settings.svelte'
  import type { main } from '../wailsjs/go/models'

  let settingsComponent: Settings
  let containerEl: HTMLDivElement
  let dragging = false
  let splitRatio = 0.6

  // タブ状態
  let activeTab: 'songs' | 'difficulty' = 'songs'

  // 楽曲タブの選択状態
  let selectedFolderHash: string | null = null

  // 難易度表タブの選択状態
  let selectedEntryMD5: string | null = null
  let selectedEntryData: main.DifficultyTableEntryDTO | null = null

  function switchTab(tab: 'songs' | 'difficulty') {
    activeTab = tab
    selectedFolderHash = null
    selectedEntryMD5 = null
    selectedEntryData = null
  }

  // 楽曲タブのハンドラ
  function handleSelect(e: CustomEvent<string>) {
    selectedFolderHash = e.detail
  }

  function handleDeselect() {
    if (activeTab === 'songs') {
      selectedFolderHash = null
    } else {
      selectedEntryMD5 = null
      selectedEntryData = null
    }
  }

  function handleClose() {
    if (activeTab === 'songs') {
      selectedFolderHash = null
    } else {
      selectedEntryMD5 = null
      selectedEntryData = null
    }
  }

  // 難易度表タブのハンドラ
  function handleEntrySelect(e: CustomEvent<{ md5: string; entry: main.DifficultyTableEntryDTO }>) {
    selectedEntryMD5 = e.detail.md5
    selectedEntryData = e.detail.entry
  }

  function handleEntryDeselect() {
    selectedEntryMD5 = null
    selectedEntryData = null
  }

  // ドラッグリサイズ
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

  $: hasDetailPane = activeTab === 'songs' ? !!selectedFolderHash : !!(selectedEntryMD5 && selectedEntryData)
</script>

<div data-theme="emerald" class="h-full flex flex-col">
  <div class="navbar bg-base-200 px-4 shrink-0">
    <div class="flex-1">
      <span class="text-xl font-bold">BMS ELSA</span>
    </div>
    <button class="btn btn-ghost btn-sm" on:click={() => settingsComponent.open()}>
      <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
      </svg>
    </button>
  </div>

  <!-- タブバー -->
  <div class="tabs tabs-bordered bg-base-100 px-4 shrink-0">
    <button
      class="tab"
      class:tab-active={activeTab === 'songs'}
      on:click={() => switchTab('songs')}
    >楽曲一覧</button>
    <button
      class="tab"
      class:tab-active={activeTab === 'difficulty'}
      on:click={() => switchTab('difficulty')}
    >難易度表</button>
  </div>

  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div bind:this={containerEl} class="flex-1 overflow-hidden p-4 flex flex-col" on:click={handleDeselect}>
    <div class="overflow-hidden" style="flex: {hasDetailPane ? splitRatio : 1}">
      {#if activeTab === 'songs'}
        <SongTable on:select={handleSelect} on:deselect={handleDeselect} />
      {:else}
        <DifficultyTableView on:select={handleEntrySelect} on:deselect={handleEntryDeselect} />
      {/if}
    </div>

    {#if hasDetailPane}
      <!-- svelte-ignore a11y-no-noninteractive-tabindex a11y-no-noninteractive-element-interactions -->
      <div
        class="h-1 shrink-0 cursor-row-resize bg-base-300 hover:bg-primary/30 transition-colors my-1 rounded"
        on:mousedown={onDragStart}
        role="separator"
        tabindex="0"
      ></div>
      <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
      <div class="overflow-y-auto" style="flex: {1 - splitRatio}" on:click|stopPropagation>
        {#if activeTab === 'songs' && selectedFolderHash}
          <SongDetail folderHash={selectedFolderHash} on:close={handleClose} />
        {:else if activeTab === 'difficulty' && selectedEntryMD5 && selectedEntryData}
          <ChartDetail md5={selectedEntryMD5} entryData={selectedEntryData} on:close={handleClose} />
        {/if}
      </div>
    {/if}
  </div>
  <Settings bind:this={settingsComponent} />
</div>

<style>
  :global(body) {
    margin: 0;
  }
</style>
