<script lang="ts">
  import SongTable from './SongTable.svelte'
  import SongDetail from './SongDetail.svelte'
  import DifficultyTableView from './DifficultyTableView.svelte'
  import ChartDetail from './ChartDetail.svelte'
  import ChartListView from './ChartListView.svelte'
  import EntryDetail from './EntryDetail.svelte'
  import SplitPane from './SplitPane.svelte'
  import Settings from './Settings.svelte'
  import EventMappingManager from './EventMappingManager.svelte'
  import type { main } from '../wailsjs/go/models'

  let settingsComponent: Settings
  let eventMappingComponent: EventMappingManager
  let splitRatio = 0.6

  // タブ状態
  let activeTab: 'songs' | 'charts' | 'difficulty' = 'songs'

  // 楽曲タブの選択状態
  let selectedFolderHash: string | null = null

  // 譜面タブの選択状態
  let selectedChartMD5: string | null = null

  // 難易度表タブの選択状態
  let selectedEntryMD5: string | null = null
  let selectedEntryData: main.DifficultyTableEntryDTO | null = null

  function switchTab(tab: 'songs' | 'charts' | 'difficulty') {
    activeTab = tab
  }

  // 楽曲タブのハンドラ
  function handleSelect(e: CustomEvent<string>) {
    if (selectedFolderHash === e.detail) {
      selectedFolderHash = null
    } else {
      selectedFolderHash = e.detail
    }
  }

  function handleDeselect() {
    if (activeTab === 'songs') {
      selectedFolderHash = null
    } else if (activeTab === 'charts') {
      selectedChartMD5 = null
    } else {
      selectedEntryMD5 = null
      selectedEntryData = null
    }
  }

  function handleClose() {
    if (activeTab === 'songs') {
      selectedFolderHash = null
    } else if (activeTab === 'charts') {
      selectedChartMD5 = null
    } else {
      selectedEntryMD5 = null
      selectedEntryData = null
    }
  }

  // 譜面タブのハンドラ
  function handleChartSelect(e: CustomEvent<{ md5: string }>) {
    if (selectedChartMD5 === e.detail.md5) {
      selectedChartMD5 = null
    } else {
      selectedChartMD5 = e.detail.md5
    }
  }

  function handleChartDeselect() {
    selectedChartMD5 = null
  }

  // 難易度表タブのハンドラ
  function handleEntrySelect(e: CustomEvent<{ md5: string; entry: main.DifficultyTableEntryDTO }>) {
    if (selectedEntryMD5 === e.detail.md5) {
      selectedEntryMD5 = null
      selectedEntryData = null
    } else {
      selectedEntryMD5 = e.detail.md5
      selectedEntryData = e.detail.entry
    }
  }

  function handleEntryDeselect() {
    selectedEntryMD5 = null
    selectedEntryData = null
  }

</script>

<div data-theme="emerald" class="h-full flex flex-col">
  <div class="navbar bg-base-200 px-4 shrink-0">
    <div class="flex-1">
      <span class="text-xl font-bold">BMS ELSA</span>
    </div>
    <button class="btn btn-ghost btn-sm" on:click={() => eventMappingComponent.open()} title="イベントマッピング">
      <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
      </svg>
    </button>
    <button class="btn btn-ghost btn-sm" on:click={() => settingsComponent.open()}>
      <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
      </svg>
    </button>
  </div>

  <!-- タブバー -->
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="tabs tabs-bordered bg-base-100 px-4 shrink-0" on:click={handleDeselect}>
    <button
      class="tab"
      class:tab-active={activeTab === 'songs'}
      on:click|stopPropagation={() => switchTab('songs')}
    >楽曲一覧</button>
    <button
      class="tab"
      class:tab-active={activeTab === 'charts'}
      on:click|stopPropagation={() => switchTab('charts')}
    >譜面一覧</button>
    <button
      class="tab"
      class:tab-active={activeTab === 'difficulty'}
      on:click|stopPropagation={() => switchTab('difficulty')}
    >難易度表</button>
  </div>

  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="flex-1 overflow-hidden p-4" on:click={handleDeselect}>
    <!-- 楽曲一覧タブ -->
    <div class="h-full" class:hidden={activeTab !== 'songs'}>
      <SplitPane showDetail={!!selectedFolderHash} bind:splitRatio>
        <SongTable slot="list" selected={selectedFolderHash} on:select={handleSelect} on:deselect={handleDeselect} />
        <svelte:fragment slot="detail">
          {#if selectedFolderHash}
            <SongDetail folderHash={selectedFolderHash} on:close={handleClose} />
          {/if}
        </svelte:fragment>
      </SplitPane>
    </div>

    <!-- 譜面一覧タブ -->
    <div class="h-full" class:hidden={activeTab !== 'charts'}>
      <SplitPane showDetail={!!selectedChartMD5} bind:splitRatio>
        <ChartListView slot="list" selected={selectedChartMD5} on:select={handleChartSelect} on:deselect={handleChartDeselect} />
        <svelte:fragment slot="detail">
          {#if selectedChartMD5}
            <ChartDetail md5={selectedChartMD5} on:close={() => { selectedChartMD5 = null }} />
          {/if}
        </svelte:fragment>
      </SplitPane>
    </div>

    <!-- 難易度表タブ -->
    <div class="h-full" class:hidden={activeTab !== 'difficulty'}>
      <SplitPane showDetail={!!(selectedEntryMD5 && selectedEntryData)} bind:splitRatio>
        <DifficultyTableView slot="list" selected={selectedEntryMD5} on:select={handleEntrySelect} on:deselect={handleEntryDeselect} />
        <svelte:fragment slot="detail">
          {#if selectedEntryMD5 && selectedEntryData}
            <EntryDetail md5={selectedEntryMD5} entryData={selectedEntryData} on:close={handleClose} />
          {/if}
        </svelte:fragment>
      </SplitPane>
    </div>
  </div>
  <Settings bind:this={settingsComponent} />
  <EventMappingManager bind:this={eventMappingComponent} />
</div>

<style>
  :global(body) {
    margin: 0;
  }
</style>
