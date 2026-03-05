<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { GetChartDetailByMD5, GetChartMetaByMD5 } from '../../wailsjs/go/app/ChartHandler'
  import { GetDifficultyTableEntry } from '../../wailsjs/go/app/DifficultyTableHandler'
  import { OpenFolder } from '../../wailsjs/go/main/App'
  import { LookupByMD5, UpdateChartMeta } from '../../wailsjs/go/app/IRHandler'
  import type { dto } from '../../wailsjs/go/models'
  import ChartInfoCard from '../components/ChartInfoCard.svelte'
  import IRInfoCard from '../components/IRInfoCard.svelte'

  const dispatch = createEventDispatcher<{ close: void }>()

  export let md5: string
  export let tableID: number

  let entryData: dto.DifficultyTableEntryDTO | null = null
  let chart: dto.ChartDTO | null = null
  let irMeta: dto.ChartIRMetaDTO | null = null
  let loading = false

  $: if (md5 && tableID) loadEntry(md5, tableID)

  // IR情報の統一アクセス（chart or irMeta）
  $: ir = chart ?? irMeta

  async function loadEntry(hash: string, tid: number) {
    loading = true
    entryData = null
    chart = null
    irMeta = null
    try {
      entryData = await GetDifficultyTableEntry(tid, hash)
      chart = await GetChartDetailByMD5(hash)
      if (!chart) {
        irMeta = await GetChartMetaByMD5(hash)
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

  async function saveWorkingUrls(e: CustomEvent<{ bodyUrl: string; diffUrl: string }>) {
    await UpdateChartMeta(md5, e.detail.bodyUrl, e.detail.diffUrl)
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
      <ChartInfoCard {chart} />
    {/if}

    <!-- IR情報（導入済・未導入共通） -->
    <IRInfoCard {md5} {ir} on:lookup={lookupIR} on:save={saveWorkingUrls} />
  </div>
{/if}
