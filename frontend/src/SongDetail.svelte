<script lang="ts">
  import { GetSongDetail, UpdateSongMeta } from '../wailsjs/go/app/SongHandler'
  import { LookupByMD5, UpdateChartMeta } from '../wailsjs/go/app/IRHandler'
  import type { dto } from '../wailsjs/go/models'

  export let folderHash: string

  let detail: dto.SongDetailDTO | null = null
  let selectedChart: dto.ChartDTO | null = null
  let loading = false

  let editEventName = ''
  let editReleaseYear = ''
  let editWorkingBodyUrl = ''
  let editWorkingDiffUrl = ''

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
    if (chart) {
      editWorkingBodyUrl = chart.workingBodyUrl || ''
      editWorkingDiffUrl = chart.workingDiffUrl || ''
    }
  }

  async function saveWorkingUrls() {
    if (!selectedChart) return
    await UpdateChartMeta(selectedChart.md5, selectedChart.sha256, editWorkingBodyUrl, editWorkingDiffUrl)
    if (detail) await loadDetail(detail.folderHash)
  }

  function modeLabel(mode: number): string {
    const labels: Record<number, string> = { 5: '5K', 7: '7K', 9: 'PMS', 10: '10K', 14: '14K', 25: '24K' }
    return labels[mode] || `${mode}K`
  }

  function diffLabel(diff: number): string {
    const labels = ['', 'BEG', 'NOR', 'HYP', 'ANO', 'INS']
    return labels[diff] || ''
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
      <h2 class="text-lg font-bold">{detail.title}</h2>
      <p class="text-sm text-base-content/70">{detail.artist}</p>
      <p class="text-xs text-base-content/50">{detail.genre}</p>
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
      {#each detail.charts as chart}
        <!-- button のネストは HTML 仕様違反のため div + role で代替 -->
        <div
          class="flex items-center gap-2 py-1 px-2 rounded cursor-pointer hover:bg-base-300 text-xs"
          class:bg-base-300={selectedChart?.md5 === chart.md5}
          role="button"
          tabindex="0"
          on:click={() => selectChart(chart)}
          on:keydown={(e) => e.key === 'Enter' && selectChart(chart)}
        >
          <span class="w-8">{modeLabel(chart.mode)}</span>
          <span class="w-8">{diffLabel(chart.difficulty)}</span>
          <span class="w-8">☆{chart.level}</span>
          <span class="flex-1 truncate text-base-content/50">{chart.md5.slice(0, 8)}...</span>
          <button
            class="btn btn-ghost btn-xs"
            on:click|stopPropagation={() => lookupIR(chart)}
          >
            IR取得
          </button>
          {#if chart.hasIrMeta}
            <span class="text-success text-xs">●</span>
          {/if}
        </div>
      {/each}
    </div>

    <!-- 選択中の譜面のIR情報 -->
    {#if selectedChart && selectedChart.hasIrMeta}
      <div class="bg-base-200 rounded-lg p-3">
        <h3 class="text-sm font-semibold mb-2">LR2IR情報</h3>
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
          <div class="flex gap-2 items-center">
            <label class="font-semibold" for="working-body-url">動作URL(本体):</label>
            <input id="working-body-url" class="input input-xs input-bordered flex-1" bind:value={editWorkingBodyUrl} on:blur={saveWorkingUrls} />
          </div>
          <div class="flex gap-2 items-center">
            <label class="font-semibold" for="working-diff-url">動作URL(差分):</label>
            <input id="working-diff-url" class="input input-xs input-bordered flex-1" bind:value={editWorkingDiffUrl} on:blur={saveWorkingUrls} />
          </div>
        </div>
      </div>
    {/if}
  </div>
{/if}
