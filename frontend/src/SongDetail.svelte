<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { GetSongDetail, UpdateSongMeta } from '../wailsjs/go/app/SongHandler'
  import { LookupByMD5, UpdateChartMeta } from '../wailsjs/go/app/IRHandler'
  import type { dto } from '../wailsjs/go/models'
  import { modeLabel, diffLabel } from './utils/chartLabels'

  const dispatch = createEventDispatcher<{ close: void }>()

  export let folderHash: string

  let detail: dto.SongDetailDTO | null = null
  let selectedChart: dto.ChartDTO | null = null
  let loading = false

  let editEventName = ''
  let editReleaseYear = ''
  let editWorkingBodyUrl = ''
  let editWorkingDiffUrl = ''
  let editingWorkingUrl = false

  $: if (folderHash) loadDetail(folderHash)

  async function loadDetail(hash: string) {
    loading = true
    try {
      detail = await GetSongDetail(hash)
      selectedChart = null
      if (detail) {
        editEventName = detail.eventName || ''
        editReleaseYear = detail.releaseYear ? String(detail.releaseYear) : ''
      }
    } catch (e) {
      console.error('Failed to load detail:', e)
    } finally {
      loading = false
    }
  }

  async function saveMeta() {
    if (!detail) return
    const year = editReleaseYear ? parseInt(editReleaseYear) : null
    const event = editEventName || null
    await UpdateSongMeta(detail.folderHash, year, event)
    await loadDetail(detail.folderHash)
  }

  async function lookupIR(chart: dto.ChartDTO) {
    await LookupByMD5(chart.md5, chart.sha256)
    if (detail) await loadDetail(detail.folderHash)
  }

  function selectChart(chart: dto.ChartDTO) {
    selectedChart = chart
    editingWorkingUrl = false
    if (chart) {
      editWorkingBodyUrl = chart.workingBodyUrl || ''
      editWorkingDiffUrl = chart.workingDiffUrl || ''
    }
  }

  async function saveWorkingUrls() {
    if (!selectedChart) return
    await UpdateChartMeta(selectedChart.md5, editWorkingBodyUrl, editWorkingDiffUrl)
    if (detail) await loadDetail(detail.folderHash)
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
        <button
          class="btn btn-ghost btn-xs shrink-0 ml-2"
          on:click={() => dispatch('close')}
        >✕</button>
      </div>
      <div class="divider my-1"></div>
      <div class="flex gap-2 items-center">
        <label class="text-xs" for="event-input">Event:</label>
        <input id="event-input" class="input input-xs input-bordered w-32" bind:value={editEventName} on:blur={saveMeta} />
        <label class="text-xs ml-2" for="year-input">Year:</label>
        <input id="year-input" class="input input-xs input-bordered w-16" type="number" bind:value={editReleaseYear} on:blur={saveMeta} />
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

    <!-- 選択中の譜面のIR情報 -->
    {#if selectedChart && selectedChart.hasIrMeta}
      <div class="bg-base-200 rounded-lg p-3">
        <h3 class="text-sm font-semibold mb-2"><a href="http://www.dream-pro.info/~lavalse/LR2IR/search.cgi?mode=ranking&bmsmd5={selectedChart.md5}" target="_blank" rel="noopener noreferrer" class="link link-primary">LR2IR情報</a></h3>
        <div class="text-xs space-y-1">
          {#if selectedChart.lr2irTags}
            <p><span class="font-semibold">タグ:</span> {selectedChart.lr2irTags}</p>
          {/if}
          {#if selectedChart.lr2irBodyUrl}
            <p>
              <span class="font-semibold">本体URL:</span>
              <a href={selectedChart.lr2irBodyUrl} target="_blank" rel="noopener noreferrer" class="link link-primary">{selectedChart.lr2irBodyUrl}</a>
            </p>
          {/if}
          {#if selectedChart.lr2irDiffUrl}
            <p>
              <span class="font-semibold">差分URL:</span>
              <a href={selectedChart.lr2irDiffUrl} target="_blank" rel="noopener noreferrer" class="link link-primary">{selectedChart.lr2irDiffUrl}</a>
            </p>
          {/if}
          {#if selectedChart.lr2irNotes}
            <p><span class="font-semibold">備考:</span> {selectedChart.lr2irNotes}</p>
          {/if}
          <div class="divider my-1"></div>
          {#if editingWorkingUrl}
            <div class="flex gap-2 items-center">
              <label class="font-semibold" for="working-body-url">動作URL(本体):</label>
              <input id="working-body-url" class="input input-xs input-bordered flex-1" bind:value={editWorkingBodyUrl} on:blur={() => { saveWorkingUrls(); editingWorkingUrl = false }} />
            </div>
            <div class="flex gap-2 items-center">
              <label class="font-semibold" for="working-diff-url">動作URL(差分):</label>
              <input id="working-diff-url" class="input input-xs input-bordered flex-1" bind:value={editWorkingDiffUrl} on:blur={() => { saveWorkingUrls(); editingWorkingUrl = false }} />
            </div>
          {:else}
            <div class="flex gap-2 items-center">
              <span class="font-semibold">動作URL(本体):</span>
              {#if editWorkingBodyUrl}
                <a href={editWorkingBodyUrl} target="_blank" rel="noopener noreferrer" class="link link-primary text-xs truncate flex-1">{editWorkingBodyUrl}</a>
              {:else}
                <span class="text-base-content/30 text-xs">未設定</span>
              {/if}
            </div>
            <div class="flex gap-2 items-center justify-between">
              <div class="flex gap-2 items-center flex-1 min-w-0">
                <span class="font-semibold">動作URL(差分):</span>
                {#if editWorkingDiffUrl}
                  <a href={editWorkingDiffUrl} target="_blank" rel="noopener noreferrer" class="link link-primary text-xs truncate flex-1">{editWorkingDiffUrl}</a>
                {:else}
                  <span class="text-base-content/30 text-xs">未設定</span>
                {/if}
              </div>
              <button class="btn btn-ghost btn-xs" on:click|stopPropagation={() => editingWorkingUrl = true}>
                編集
              </button>
            </div>
          {/if}
        </div>
      </div>
    {/if}
  </div>
{/if}
