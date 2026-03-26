<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte'
  import { GetSongDetail, UpdateSongMeta, MoveSongFolder } from '../../wailsjs/go/app/SongHandler'
  import { ListEvents } from '../../wailsjs/go/app/EventHandler'
  import { LookupByMD5, UpdateChartMeta } from '../../wailsjs/go/app/IRHandler'
  import { SelectDirectory } from '../../wailsjs/go/main/App'
  import type { dto } from '../../wailsjs/go/models'
  import { modeLabel, diffLabel } from '../utils/chartLabels'
  import ChartInfoCard from '../components/ChartInfoCard.svelte'
  import IRInfoCard from '../components/IRInfoCard.svelte'
  import OpenFolderButton from '../components/OpenFolderButton.svelte'
  import Icon from '../components/Icon.svelte'
  import AlertModal from '../components/AlertModal.svelte'

  const dispatch = createEventDispatcher<{ close: void; moved: { folderHash: string } }>()

  export let folderHash: string
  export let moved = false

  let detail: dto.SongDetailDTO | null = null
  let selectedChart: dto.ChartDTO | null = null
  let loading = false

  let editReleaseYear = ''

  let allEvents: any[] = []
  let eventSuggestions: any[] = []
  let eventSearchText = ''
  let showEventDropdown = false

  let confirmDialog: HTMLDialogElement
  let resultDialog: HTMLDialogElement
  let alertModal: AlertModal
  let moving = false
  let moveDestParent = ''
  let moveResult: { destPath: string; fileCount: number } | null = null
  let mouseDownOnBackdrop = false

  onMount(async () => {
    allEvents = (await ListEvents()) || []
  })

  $: if (folderHash) loadDetail(folderHash)

  async function loadDetail(hash: string) {
    loading = true
    try {
      detail = await GetSongDetail(hash)
      selectedChart = null
      if (detail) {
        eventSearchText = detail.eventName || ''
        editReleaseYear = detail.releaseYear ? String(detail.releaseYear) : ''
      }
    } catch (e) {
      console.error('Failed to load detail:', e)
    } finally {
      loading = false
    }
  }

  async function selectEvent(eventID: string, shortName: string) {
    if (!detail) return
    eventSearchText = shortName
    showEventDropdown = false
    await UpdateSongMeta(detail.folderHash, editReleaseYear ? parseInt(editReleaseYear) : null, eventID)
    await loadDetail(detail.folderHash)
  }

  async function saveMeta() {
    if (!detail) return
    const year = editReleaseYear ? parseInt(editReleaseYear) : null
    await UpdateSongMeta(detail.folderHash, year, null)
    await loadDetail(detail.folderHash)
  }

  async function lookupIR(chart: dto.ChartDTO) {
    await LookupByMD5(chart.md5, chart.sha256)
    if (detail) await loadDetail(detail.folderHash)
  }

  function selectChart(chart: dto.ChartDTO) {
    selectedChart = chart
  }

  async function saveWorkingUrls(e: CustomEvent<{ bodyUrl: string; diffUrl: string }>) {
    if (!selectedChart) return
    await UpdateChartMeta(selectedChart.md5, e.detail.bodyUrl, e.detail.diffUrl)
    if (detail) await loadDetail(detail.folderHash)
  }

  async function startMove() {
    if (!detail) return
    try {
      const dir = await SelectDirectory()
      if (!dir) return
      moveDestParent = dir
      confirmDialog.showModal()
    } catch (e) {
      // キャンセル
    }
  }

  function cancelMove() {
    confirmDialog.close()
  }

  async function executeMove() {
    if (!detail) return
    moving = true
    try {
      const result = await MoveSongFolder(detail.folderHash, moveDestParent)
      confirmDialog.close()
      moveResult = result
      resultDialog.showModal()
    } catch (err) {
      confirmDialog.close()
      alertModal.open(String(err))
    } finally {
      moving = false
    }
  }

  function closeResult() {
    resultDialog.close()
    dispatch('moved', { folderHash: folderHash })
  }

</script>

{#if loading}
  <div class="flex items-center justify-center h-full">
    <span class="loading loading-spinner"></span>
  </div>
{:else if detail}
  <div class="flex flex-col gap-3">
    <!-- 楽曲ヘッダー -->
    <div class="bg-base-200 rounded-lg p-3">
      <div class="flex justify-between items-start">
        <div class="flex-1 min-w-0">
          <p class="text-xs text-base-content/50">{detail.genre}</p>
          <h2 class="text-lg font-bold truncate">{detail.title}</h2>
          <p class="text-sm text-base-content/70">{detail.artist}</p>
        </div>
        <div class="flex items-center shrink-0 ml-2">
          <OpenFolderButton path={detail.charts[0]?.path} title="インストール先フォルダを開く" />
          <button
            class="btn btn-ghost btn-xs"
            title="フォルダ移動"
            on:click|stopPropagation={startMove}
            disabled={moved}
          >
            <Icon name="folderMove" cls="h-4 w-4" />
          </button>
          <button
            class="btn btn-ghost btn-xs"
            on:click={() => dispatch('close')}
          >
            <Icon name="close" cls="h-4 w-4" />
          </button>
        </div>
      </div>
      <div class="divider my-1"></div>
      <div class="flex gap-2 items-center">
        <label class="text-xs">Event:</label>
        <div class="relative">
          <input
            class="input input-xs input-bordered w-40"
            bind:value={eventSearchText}
            on:input={() => {
              const q = eventSearchText.toLowerCase()
              eventSuggestions = allEvents.filter(ev =>
                ev.shortName.toLowerCase().includes(q) || ev.name.toLowerCase().includes(q)
              ).slice(0, 10)
              showEventDropdown = eventSuggestions.length > 0 && eventSearchText.length > 0
            }}
            on:focus={() => {
              if (eventSearchText) {
                const q = eventSearchText.toLowerCase()
                eventSuggestions = allEvents.filter(ev =>
                  ev.shortName.toLowerCase().includes(q) || ev.name.toLowerCase().includes(q)
                ).slice(0, 10)
                showEventDropdown = eventSuggestions.length > 0
              }
            }}
            on:blur={() => setTimeout(() => { showEventDropdown = false }, 200)}
            placeholder="イベント検索..."
          />
          {#if showEventDropdown}
            <ul class="absolute z-50 bg-base-100 border border-base-300 rounded shadow-lg mt-1 max-h-40 overflow-y-auto w-64">
              {#each eventSuggestions as ev}
                <li>
                  <button
                    class="w-full text-left px-2 py-1 text-xs hover:bg-base-200"
                    on:mousedown|preventDefault={() => selectEvent(ev.bmsSearchId, ev.shortName)}
                  >
                    <span class="font-semibold">{ev.shortName}</span>
                    {#if ev.shortName !== ev.name}
                      <span class="text-base-content/50 ml-1 truncate">({ev.name})</span>
                    {/if}
                  </button>
                </li>
              {/each}
            </ul>
          {/if}
        </div>
        <label class="text-xs ml-2" for="year-input">Year:</label>
        {#if detail?.eventId}
          <span class="text-xs w-16 text-center">{detail.releaseYear}</span>
        {:else}
          <input id="year-input" class="input input-xs input-bordered w-16" type="number" bind:value={editReleaseYear} on:blur={saveMeta} />
        {/if}
      </div>
    </div>

    <!-- 譜面一覧 -->
    <div class="bg-base-200 rounded-lg p-3">
      <h3 class="text-sm font-semibold mb-2">譜面一覧</h3>
      <table class="table table-xs w-full">
        <thead>
          <tr>
            <th class="w-12">Mode</th>
            <th class="w-10">Diff</th>
            <th class="w-10">Lv</th>
            <th>Subtitle</th>
            <th>難易度表</th>
            <th>Path</th>
            <th class="w-8">IR</th>
            <th class="w-16"></th>
          </tr>
        </thead>
        <tbody>
          {#each detail.charts as chart}
            <tr
              class="cursor-pointer hover:bg-base-300"
              class:bg-base-300={selectedChart?.md5 === chart.md5}
              on:click={() => selectChart(chart)}
              on:keydown={(e) => e.key === 'Enter' && selectChart(chart)}
            >
              <td>{modeLabel(chart.mode)}</td>
              <td>{diffLabel(chart.difficulty)}</td>
              <td>☆{chart.level}</td>
              <td class="truncate max-w-[200px]">{chart.subtitle || ''}</td>
              <td>
                {#if chart.difficultyLabels?.length}
                  <div class="flex gap-1 flex-wrap">
                    {#each chart.difficultyLabels as label}
                      <span class="badge badge-sm badge-outline" title={label.tableName}>{label.symbol}{label.level}</span>
                    {/each}
                  </div>
                {/if}
              </td>
              <td class="truncate max-w-[200px] text-base-content/50">{chart.path || ''}</td>
              <td>
                {#if chart.hasIrMeta}
                  <span class="text-success">●</span>
                {/if}
              </td>
              <td>
                <button
                  class="btn btn-ghost btn-xs"
                  on:click|stopPropagation={() => lookupIR(chart)}
                >
                  IR取得
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <!-- 選択中の譜面の詳細情報 -->
    {#if selectedChart}
      <ChartInfoCard chart={selectedChart} />
      <IRInfoCard md5={selectedChart.md5} ir={selectedChart} on:lookup={() => selectedChart && lookupIR(selectedChart)} on:save={saveWorkingUrls} />
    {/if}
  </div>
{/if}

<!-- 移動確認ダイアログ -->
<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<dialog bind:this={confirmDialog} class="modal"
  on:mousedown|self={() => mouseDownOnBackdrop = true}
  on:click|self={() => { if (mouseDownOnBackdrop) cancelMove(); mouseDownOnBackdrop = false }}>
  <div class="modal-box max-w-2xl">
    <h3 class="text-lg font-bold mb-4">フォルダ移動の確認</h3>
    <div class="space-y-2 text-sm">
      <p>楽曲フォルダを移動します。移動元フォルダは削除されます。</p>
      <div class="bg-base-200 rounded p-2 space-y-1">
        <div><span class="text-base-content/50">楽曲:</span> <span class="break-all">{detail?.title}</span></div>
        <div><span class="text-base-content/50">移動先:</span> <span class="break-all">{moveDestParent}</span></div>
      </div>
    </div>
    <div class="modal-action">
      <button class="btn" on:click={cancelMove}>キャンセル</button>
      <button class="btn btn-warning" on:click={executeMove} disabled={moving}>
        {moving ? '移動中...' : '移動実行'}
      </button>
    </div>
  </div>
</dialog>

<!-- 移動結果ダイアログ -->
<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<dialog bind:this={resultDialog} class="modal"
  on:mousedown|self={() => mouseDownOnBackdrop = true}
  on:click|self={() => { if (mouseDownOnBackdrop) closeResult(); mouseDownOnBackdrop = false }}>
  <div class="modal-box max-w-2xl">
    <h3 class="text-lg font-bold mb-4">移動完了</h3>
    {#if moveResult}
      <div class="space-y-2 text-sm">
        <div><span class="text-base-content/50">移動先:</span> <span class="break-all">{moveResult.destPath}</span></div>
        <div><span class="text-base-content/50">ファイル数:</span> {moveResult.fileCount}</div>
      </div>
    {/if}
    <div class="modal-action">
      <button class="btn" on:click={closeResult}>OK</button>
    </div>
  </div>
</dialog>

<AlertModal bind:this={alertModal} />
