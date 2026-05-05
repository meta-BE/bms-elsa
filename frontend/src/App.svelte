<script lang="ts">
  import SongListView from './views/SongListView.svelte'
  import SongDetail from './views/SongDetail.svelte'
  import DifficultyTableView from './views/DifficultyTableView.svelte'
  import ChartDetail from './views/ChartDetail.svelte'
  import ChartListView from './views/ChartListView.svelte'
  import EntryDetail from './views/EntryDetail.svelte'
  import DuplicateView from './views/DuplicateView.svelte'
  import DuplicateDetail from './views/DuplicateDetail.svelte'
  import DiffImportView from './views/DiffImportView.svelte'
  import SplitPane from './components/SplitPane.svelte'
  import ContextMenu from './components/ContextMenu.svelte'
  import Settings from './settings/Settings.svelte'
  import EventManager from './settings/EventManager.svelte'
  import RewriteRuleManager from './settings/RewriteRuleManager.svelte'
  import { OpenURL } from '../wailsjs/go/main/App'
  import { OnFileDrop } from '../wailsjs/runtime/runtime'
  import { rewriteRules } from './stores/rewriteRules'
  import { initCardCollapsed } from './stores/cardCollapsed'
  import { ListRewriteRules } from '../wailsjs/go/app/RewriteHandler'
  import Icon from './components/Icon.svelte'
  import { onMount } from 'svelte'
  let settingsComponent: Settings
  let eventManagerComponent: EventManager
  let rewriteRuleComponent: RewriteRuleManager
  let diffImportView: DiffImportView
  let duplicateViewRef: DuplicateView
  let splitRatio = 0.6

  // タブ状態
  let activeTab: 'songs' | 'charts' | 'difficulty' | 'duplicates' | 'diff-import' = 'songs'

  // 楽曲タブの選択状態
  let selectedFolderHash: string | null = null

  // 譜面タブの選択状態
  let selectedChart: { md5: string; folderHash: string } | null = null

  // 難易度表タブの選択状態
  let selectedEntryMD5: string | null = null
  let selectedTableID: number | null = null

  // 重複検知タブの選択状態
  let selectedDuplicateGroup: any = null

  function switchTab(tab: 'songs' | 'charts' | 'difficulty' | 'duplicates' | 'diff-import') {
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
      selectedChart = null
    } else {
      selectedEntryMD5 = null
      selectedTableID = null
    }
  }

  function handleClose() {
    if (activeTab === 'songs') {
      selectedFolderHash = null
    } else if (activeTab === 'charts') {
      selectedChart = null
    } else {
      selectedEntryMD5 = null
      selectedTableID = null
    }
  }

  // 譜面タブのハンドラ
  function handleChartSelect(e: CustomEvent<{ md5: string; folderHash: string }>) {
    if (selectedChart?.md5 === e.detail.md5 && selectedChart?.folderHash === e.detail.folderHash) {
      selectedChart = null
    } else {
      selectedChart = { md5: e.detail.md5, folderHash: e.detail.folderHash }
    }
  }

  function handleChartDeselect() {
    selectedChart = null
  }

  // 難易度表タブのハンドラ
  function handleEntrySelect(e: CustomEvent<{ md5: string; tableID: number }>) {
    if (selectedEntryMD5 === e.detail.md5) {
      selectedEntryMD5 = null
      selectedTableID = null
    } else {
      selectedEntryMD5 = e.detail.md5
      selectedTableID = e.detail.tableID
    }
  }

  function handleEntryDeselect() {
    selectedEntryMD5 = null
    selectedTableID = null
  }

  // 重複検知タブのハンドラ
  function handleDuplicateSelect(e: CustomEvent) {
    selectedDuplicateGroup = e.detail
  }

  function handleMemberMerged(e: CustomEvent<{ folderHash: string }>) {
    duplicateViewRef?.removeMember(e.detail.folderHash)
    if (selectedDuplicateGroup) {
      selectedDuplicateGroup = { ...selectedDuplicateGroup }
      if (selectedDuplicateGroup.Members.length <= 1) {
        selectedDuplicateGroup = null
      }
    }
  }

  // 楽曲フォルダ移動済みの状態（セッション中のみ）
  let movedFolderHashes: Set<string> = new Set()
  let songListViewComponent: SongListView

  function handleSongMoved(e: CustomEvent<{ folderHash: string }>) {
    movedFolderHashes = new Set([...movedFolderHashes, e.detail.folderHash])
    selectedFolderHash = null
  }

  function handleSongMetaUpdated(e: CustomEvent<{ folderHash: string; eventName: string | null; releaseYear: number | null }>) {
    songListViewComponent?.updateRow(e.detail.folderHash, e.detail.eventName, e.detail.releaseYear)
  }

  // 外部リンクをシステムブラウザで開く
  // capture: true でstopPropagationより先に実行する
  onMount(() => {
    ListRewriteRules().then(rules => {
      rewriteRules.set(rules ?? [])
    })

    initCardCollapsed()

    document.addEventListener('click', (e) => {
      const anchor = (e.target as Element).closest('a[href]')
      if (!anchor) return
      const href = anchor.getAttribute('href')
      if (href && (href.startsWith('http://') || href.startsWith('https://'))) {
        e.preventDefault()
        e.stopPropagation()
        OpenURL(href)
      }
    }, true)

    // ファイルドロップはグローバルに1つしか登録できないため、ここで一元管理
    OnFileDrop((_x: number, _y: number, paths: string[]) => {
      if (activeTab === 'diff-import') {
        diffImportView?.handleFileDrop(paths)
      }
    }, false)
  })

</script>

<div data-theme="emerald" class="h-full flex flex-col">
  <div class="navbar bg-base-200 px-4 shrink-0">
    <div class="flex-1">
      <span class="text-xl font-bold">BMS ELSA</span>
    </div>
    <button class="btn btn-ghost btn-sm" on:click={() => rewriteRuleComponent.open()} title="URL書き換えルール">
      <Icon name="arrowPath" cls="h-5 w-5" />
    </button>
    <button class="btn btn-ghost btn-sm" on:click={() => eventManagerComponent.open()} title="イベント管理">
      <Icon name="calendar" cls="h-5 w-5" />
    </button>
    <button class="btn btn-ghost btn-sm" on:click={() => settingsComponent.open()} title="設定">
      <Icon name="cog" cls="h-5 w-5" />
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
    <button
      class="tab"
      class:tab-active={activeTab === 'duplicates'}
      on:click|stopPropagation={() => switchTab('duplicates')}
    >重複検知</button>
    <button
      class="tab"
      class:tab-active={activeTab === 'diff-import'}
      on:click|stopPropagation={() => switchTab('diff-import')}
    >差分導入</button>
  </div>

  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="flex-1 overflow-hidden p-4" on:click={handleDeselect}>
    <!-- 楽曲一覧タブ -->
    <div class="h-full" class:hidden={activeTab !== 'songs'}>
      <SplitPane showDetail={!!selectedFolderHash} bind:splitRatio>
        <SongListView bind:this={songListViewComponent} slot="list" selected={selectedFolderHash} movedHashes={movedFolderHashes} active={activeTab === 'songs'} on:select={handleSelect} on:deselect={handleDeselect} />
        <svelte:fragment slot="detail">
          {#if selectedFolderHash}
            <SongDetail folderHash={selectedFolderHash} moved={movedFolderHashes.has(selectedFolderHash)} on:close={handleClose} on:moved={handleSongMoved} on:metaUpdated={handleSongMetaUpdated} />
          {/if}
        </svelte:fragment>
      </SplitPane>
    </div>

    <!-- 譜面一覧タブ -->
    <div class="h-full" class:hidden={activeTab !== 'charts'}>
      <SplitPane showDetail={!!selectedChart} bind:splitRatio>
        <ChartListView slot="list" selected={selectedChart ? selectedChart.md5 + ':' + selectedChart.folderHash : null} active={activeTab === 'charts'} on:select={handleChartSelect} on:deselect={handleChartDeselect} />
        <svelte:fragment slot="detail">
          {#if selectedChart}
            <ChartDetail md5={selectedChart.md5} folderHash={selectedChart.folderHash} on:close={() => { selectedChart = null }} />
          {/if}
        </svelte:fragment>
      </SplitPane>
    </div>

    <!-- 難易度表タブ -->
    <div class="h-full" class:hidden={activeTab !== 'difficulty'}>
      <SplitPane showDetail={!!(selectedEntryMD5 && selectedTableID)} bind:splitRatio>
        <DifficultyTableView slot="list" selected={selectedEntryMD5} active={activeTab === 'difficulty'} on:select={handleEntrySelect} on:deselect={handleEntryDeselect} />
        <svelte:fragment slot="detail">
          {#if selectedEntryMD5 && selectedTableID}
            <EntryDetail md5={selectedEntryMD5} tableID={selectedTableID} on:close={handleClose} />
          {/if}
        </svelte:fragment>
      </SplitPane>
    </div>

    <!-- 重複検知タブ -->
    <div class="h-full" class:hidden={activeTab !== 'duplicates'}>
      <SplitPane showDetail={!!selectedDuplicateGroup} bind:splitRatio>
        <DuplicateView slot="list" active={activeTab === 'duplicates'} on:select={handleDuplicateSelect} bind:this={duplicateViewRef} />
        <svelte:fragment slot="detail">
          {#if selectedDuplicateGroup}
            <DuplicateDetail group={selectedDuplicateGroup} on:memberMerged={handleMemberMerged} />
          {/if}
        </svelte:fragment>
      </SplitPane>
    </div>

    <!-- 差分導入タブ -->
    <div class="h-full" class:hidden={activeTab !== 'diff-import'}>
      <DiffImportView bind:this={diffImportView} />
    </div>
  </div>
  <Settings bind:this={settingsComponent} />
  <EventManager bind:this={eventManagerComponent} />
  <RewriteRuleManager bind:this={rewriteRuleComponent} />
  <ContextMenu />
</div>

<style>
  :global(body) {
    margin: 0;
  }
</style>
