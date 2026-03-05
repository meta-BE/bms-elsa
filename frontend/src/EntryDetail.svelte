<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { GetChartDetailByMD5, GetChartMetaByMD5, GetDifficultyTableEntry, OpenFolder } from '../wailsjs/go/main/App'
  import { LookupByMD5, UpdateChartMeta } from '../wailsjs/go/app/IRHandler'
  import type { dto, main } from '../wailsjs/go/models'
  import { modeLabel, diffLabel } from './utils/chartLabels'

  const dispatch = createEventDispatcher<{ close: void }>()

  export let md5: string
  export let tableID: number

  let entryData: main.DifficultyTableEntryDTO | null = null
  let chart: dto.ChartDTO | null = null
  let irMeta: dto.ChartIRMetaDTO | null = null
  let loading = false
  let editWorkingBodyUrl = ''
  let editWorkingDiffUrl = ''
  let editingWorkingUrl = false

  $: if (md5 && tableID) loadEntry(md5, tableID)

  // IR情報の統一アクセス（chart or irMeta）
  $: ir = chart ?? irMeta

  async function loadEntry(hash: string, tid: number) {
    loading = true
    entryData = null
    chart = null
    irMeta = null
    editingWorkingUrl = false
    try {
      entryData = await GetDifficultyTableEntry(tid, hash)
      chart = await GetChartDetailByMD5(hash)
      if (chart) {
        editWorkingBodyUrl = chart.workingBodyUrl || ''
        editWorkingDiffUrl = chart.workingDiffUrl || ''
      } else {
        irMeta = await GetChartMetaByMD5(hash)
        if (irMeta) {
          editWorkingBodyUrl = irMeta.workingBodyUrl || ''
          editWorkingDiffUrl = irMeta.workingDiffUrl || ''
        }
      }
    } catch (e) {
      console.error('Failed to load entry detail:', e)
    } finally {
      loading = false
    }
  }

  async function lookupIR() {
    await LookupByMD5(md5, chart?.sha256 || '')
    await loadEntry(md5, tableID)
  }

  async function saveWorkingUrls() {
    if (!ir) return
    await UpdateChartMeta(md5, editWorkingBodyUrl, editWorkingDiffUrl)
    await loadEntry(md5, tableID)
  }

</script>

{#if loading}
  <div class="flex items-center justify-center h-full">
    <span class="loading loading-spinner"></span>
  </div>
{:else if entryData}
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
        <div class="flex items-center shrink-0 ml-2">
          {#if chart?.path}
            <button
              class="btn btn-ghost btn-xs"
              title="インストール先フォルダを開く"
              on:click={() => OpenFolder(chart.path)}
            >📁</button>
          {/if}
          <button
            class="btn btn-ghost btn-xs"
            on:click={() => dispatch('close')}
          >✕</button>
        </div>
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
    {/if}

    <!-- IR情報（導入済・未導入共通） -->
    <div class="bg-base-200 rounded-lg p-3">
      <div class="flex items-center justify-between mb-2">
        <h3 class="text-sm font-semibold"><a href="http://www.dream-pro.info/~lavalse/LR2IR/search.cgi?mode=ranking&bmsmd5={md5}" target="_blank" rel="noopener noreferrer" class="link link-primary">LR2IR情報</a></h3>
        <button class="btn btn-ghost btn-xs" on:click={lookupIR}>IR取得</button>
      </div>
      {#if ir?.hasIrMeta}
        <div class="text-xs space-y-1">
          {#if ir.lr2irTags}
            <p><span class="font-semibold">タグ:</span> {ir.lr2irTags}</p>
          {/if}
          {#if ir.lr2irBodyUrl}
            <p>
              <span class="font-semibold">本体URL:</span>
              <a href={ir.lr2irBodyUrl} target="_blank" rel="noopener noreferrer" class="link link-primary">{ir.lr2irBodyUrl}</a>
            </p>
          {/if}
          {#if ir.lr2irDiffUrl}
            <p>
              <span class="font-semibold">差分URL:</span>
              <a href={ir.lr2irDiffUrl} target="_blank" rel="noopener noreferrer" class="link link-primary">{ir.lr2irDiffUrl}</a>
            </p>
          {/if}
          {#if ir.lr2irNotes}
            <p><span class="font-semibold">備考:</span> {ir.lr2irNotes}</p>
          {/if}
          <div class="divider my-1"></div>
          {#if editingWorkingUrl}
            <div class="flex gap-2 items-center">
              <label class="font-semibold" for="entry-working-body-url">動作URL(本体):</label>
              <input id="entry-working-body-url" class="input input-xs input-bordered flex-1" bind:value={editWorkingBodyUrl} on:blur={() => { saveWorkingUrls(); editingWorkingUrl = false }} />
            </div>
            <div class="flex gap-2 items-center">
              <label class="font-semibold" for="entry-working-diff-url">動作URL(差分):</label>
              <input id="entry-working-diff-url" class="input input-xs input-bordered flex-1" bind:value={editWorkingDiffUrl} on:blur={() => { saveWorkingUrls(); editingWorkingUrl = false }} />
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
      {:else}
        <p class="text-xs text-base-content/50">IR情報がありません。「IR取得」ボタンで取得してください。</p>
      {/if}
    </div>
  </div>
{/if}
