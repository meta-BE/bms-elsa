<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { GetChartDetailByMD5 } from '../../wailsjs/go/app/ChartHandler'
  import { OpenFolder } from '../../wailsjs/go/main/App'
  import { LookupByMD5, UpdateChartMeta } from '../../wailsjs/go/app/IRHandler'
  import type { dto } from '../../wailsjs/go/models'
  import ChartInfoCard from '../components/ChartInfoCard.svelte'
  import IRInfoCard from '../components/IRInfoCard.svelte'

  const dispatch = createEventDispatcher<{ close: void }>()

  export let md5: string

  let chart: dto.ChartDTO | null = null
  let loading = false

  $: if (md5) loadChart(md5)

  async function loadChart(hash: string) {
    loading = true
    chart = null
    try {
      chart = await GetChartDetailByMD5(hash)
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

  async function saveWorkingUrls(e: CustomEvent<{ bodyUrl: string; diffUrl: string }>) {
    if (!chart) return
    await UpdateChartMeta(chart.md5, e.detail.bodyUrl, e.detail.diffUrl)
    await loadChart(md5)
  }

</script>

{#if loading}
  <div class="flex items-center justify-center h-full">
    <span class="loading loading-spinner"></span>
  </div>
{:else}
  <div class="flex flex-col gap-3">
    <!-- 譜面ヘッダー -->
    <div class="bg-base-200 rounded-lg p-3">
      <div class="flex justify-between items-start">
        <div class="flex-1 min-w-0">
          <h2 class="text-lg font-bold truncate">{chart?.title ?? ''}{chart?.subtitle ? ' ' + chart.subtitle : ''}</h2>
          <p class="text-sm text-base-content/70">{chart?.artist ?? ''}{chart?.subArtist ? ' ' + chart.subArtist : ''}</p>
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
    </div>

    {#if chart}
      <ChartInfoCard {chart} />
      <IRInfoCard md5={chart.md5} ir={chart} on:lookup={lookupIR} on:save={saveWorkingUrls} />
    {/if}
  </div>
{/if}
