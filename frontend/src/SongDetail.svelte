<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { GetSongDetail, UpdateSongMeta } from '../wailsjs/go/app/SongHandler'
  import { LookupByMD5, UpdateChartMeta } from '../wailsjs/go/app/IRHandler'
  import { OpenFolder } from '../wailsjs/go/main/App'
  import type { dto } from '../wailsjs/go/models'
  import { modeLabel, diffLabel } from './utils/chartLabels'
  import ChartInfoCard from './ChartInfoCard.svelte'
  import IRInfoCard from './IRInfoCard.svelte'

  const dispatch = createEventDispatcher<{ close: void }>()

  export let folderHash: string

  let detail: dto.SongDetailDTO | null = null
  let selectedChart: dto.ChartDTO | null = null
  let loading = false

  let editEventName = ''
  let editReleaseYear = ''

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
  }

  async function saveWorkingUrls(e: CustomEvent<{ bodyUrl: string; diffUrl: string }>) {
    if (!selectedChart) return
    await UpdateChartMeta(selectedChart.md5, e.detail.bodyUrl, e.detail.diffUrl)
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
        <div class="flex items-center shrink-0 ml-2">
          {#if detail.charts[0]?.path}
            <button
              class="btn btn-ghost btn-xs"
              title="インストール先フォルダを開く"
              on:click={() => OpenFolder(detail.charts[0].path)}
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 19a2 2 0 01-2-2V7a2 2 0 012-2h4l2 2h4a2 2 0 012 2v1M5 19h14a2 2 0 002-2v-5a2 2 0 00-2-2H9a2 2 0 00-2 2v5a2 2 0 01-2 2z" />
              </svg>
            </button>
          {/if}
          <button
            class="btn btn-ghost btn-xs"
            on:click={() => dispatch('close')}
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
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

    <!-- 選択中の譜面の詳細情報 -->
    {#if selectedChart}
      <ChartInfoCard chart={selectedChart} />
      <IRInfoCard md5={selectedChart.md5} ir={selectedChart} on:lookup={() => selectedChart && lookupIR(selectedChart)} on:save={saveWorkingUrls} />
    {/if}
  </div>
{/if}
