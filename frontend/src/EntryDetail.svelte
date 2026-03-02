<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { GetChartDetailByMD5 } from '../wailsjs/go/main/App'
  import { LookupByMD5, UpdateChartMeta } from '../wailsjs/go/app/IRHandler'
  import type { dto, main } from '../wailsjs/go/models'
  import { modeLabel, diffLabel } from './utils/chartLabels'

  const dispatch = createEventDispatcher<{ close: void }>()

  export let md5: string
  export let entryData: main.DifficultyTableEntryDTO

  let chart: dto.ChartDTO | null = null
  let loading = false
  let editWorkingBodyUrl = ''
  let editWorkingDiffUrl = ''

  $: if (md5) loadChart(md5)

  async function loadChart(hash: string) {
    loading = true
    chart = null
    try {
      chart = await GetChartDetailByMD5(hash)
      if (chart) {
        editWorkingBodyUrl = chart.workingBodyUrl || ''
        editWorkingDiffUrl = chart.workingDiffUrl || ''
      }
    } catch (e) {
      console.error('Failed to load chart detail:', e)
      chart = null
    } finally {
      loading = false
    }
  }

  async function lookupIR() {
    if (!chart) return
    await LookupByMD5(chart.md5, chart.sha256)
    await loadChart(md5)
  }

  async function saveWorkingUrls() {
    if (!chart) return
    await UpdateChartMeta(chart.md5, editWorkingBodyUrl, editWorkingDiffUrl)
    await loadChart(md5)
  }

</script>

{#if loading}
  <div class="flex items-center justify-center h-full">
    <span class="loading loading-spinner"></span>
  </div>
{:else}
  <div class="flex flex-col gap-3">
    <!-- エントリ基本情報 -->
    <div class="bg-base-200 rounded-lg p-3">
      <div class="flex justify-between items-start">
        <div class="flex-1 min-w-0">
          <h2 class="text-lg font-bold truncate">{chart ? (chart.title + (chart.subtitle ? ' ' + chart.subtitle : '')) : entryData.title}</h2>
          <p class="text-sm text-base-content/70">{chart ? (chart.artist + (chart.subArtist ? ' ' + chart.subArtist : '')) : entryData.artist}</p>
          <div class="flex items-center gap-2 mt-1">
            <span class="badge badge-sm">Lv. {entryData.level}</span>
            {#if !chart}
              <span class="badge badge-sm badge-warning">未導入</span>
            {:else if entryData.status === 'duplicate'}
              <span class="badge badge-sm badge-warning">重複</span>
            {:else}
              <span class="badge badge-sm badge-success">導入済</span>
            {/if}
          </div>
        </div>
        <button
          class="btn btn-ghost btn-xs shrink-0 ml-2"
          on:click={() => dispatch('close')}
        >✕</button>
      </div>
      {#if entryData.url || entryData.urlDiff}
        <div class="divider my-1"></div>
        <div class="text-xs space-y-1">
          {#if entryData.url}
            <p>
              <span class="font-semibold">URL:</span>
              <a href={entryData.url} target="_blank" rel="noopener noreferrer" class="link link-primary">{entryData.url}</a>
            </p>
          {/if}
          {#if entryData.urlDiff}
            <p>
              <span class="font-semibold">差分URL:</span>
              <a href={entryData.urlDiff} target="_blank" rel="noopener noreferrer" class="link link-primary">{entryData.urlDiff}</a>
            </p>
          {/if}
        </div>
      {/if}
    </div>

    <!-- 譜面メタデータ（導入済の場合のみ） -->
    {#if chart}
      <div class="bg-base-200 rounded-lg p-3">
        <h3 class="text-sm font-semibold mb-2">譜面情報</h3>
        <div class="text-xs space-y-1">
          <div class="flex items-center gap-4">
            <span><span class="font-semibold">Mode:</span> {modeLabel(chart.mode)}</span>
            <span><span class="font-semibold">Difficulty:</span> {diffLabel(chart.difficulty)}</span>
            <span><span class="font-semibold">Level:</span> ☆{chart.level}</span>
          </div>
          <p>
            <span class="font-semibold">BPM:</span>
            {#if chart.minBpm === chart.maxBpm}
              {Math.round(chart.minBpm)}
            {:else}
              {Math.round(chart.minBpm)}-{Math.round(chart.maxBpm)}
            {/if}
          </p>
          {#if chart.difficultyLabels?.length}
            <div class="flex items-center gap-1 flex-wrap">
              <span class="font-semibold">難易度表:</span>
              {#each chart.difficultyLabels as label}
                <span class="badge badge-sm badge-outline" title={label.tableName}>{label.symbol}{label.level}</span>
              {/each}
            </div>
          {/if}
        </div>
      </div>

      <!-- IR情報 -->
      <div class="bg-base-200 rounded-lg p-3">
        <div class="flex items-center justify-between mb-2">
          <h3 class="text-sm font-semibold">LR2IR情報</h3>
          <button class="btn btn-ghost btn-xs" on:click={lookupIR}>IR取得</button>
        </div>
        {#if chart.hasIrMeta}
          <div class="text-xs space-y-1">
            {#if chart.lr2irTags}
              <p><span class="font-semibold">タグ:</span> {chart.lr2irTags}</p>
            {/if}
            {#if chart.lr2irBodyUrl}
              <p>
                <span class="font-semibold">本体URL:</span>
                <a href={chart.lr2irBodyUrl} target="_blank" rel="noopener noreferrer" class="link link-primary">{chart.lr2irBodyUrl}</a>
              </p>
            {/if}
            {#if chart.lr2irDiffUrl}
              <p>
                <span class="font-semibold">差分URL:</span>
                <a href={chart.lr2irDiffUrl} target="_blank" rel="noopener noreferrer" class="link link-primary">{chart.lr2irDiffUrl}</a>
              </p>
            {/if}
            {#if chart.lr2irNotes}
              <p><span class="font-semibold">備考:</span> {chart.lr2irNotes}</p>
            {/if}
            <div class="divider my-1"></div>
            <div class="flex gap-2 items-center">
              <label class="font-semibold" for="entry-working-body-url">動作URL(本体):</label>
              <input id="entry-working-body-url" class="input input-xs input-bordered flex-1" bind:value={editWorkingBodyUrl} on:blur={saveWorkingUrls} />
            </div>
            <div class="flex gap-2 items-center">
              <label class="font-semibold" for="entry-working-diff-url">動作URL(差分):</label>
              <input id="entry-working-diff-url" class="input input-xs input-bordered flex-1" bind:value={editWorkingDiffUrl} on:blur={saveWorkingUrls} />
            </div>
          </div>
        {:else}
          <p class="text-xs text-base-content/50">IR情報がありません。「IR取得」ボタンで取得してください。</p>
        {/if}
      </div>
    {/if}
  </div>
{/if}
