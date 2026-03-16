<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { GetChartDetailByMD5 } from '../../wailsjs/go/app/ChartHandler'
  import { LookupByMD5, UpdateChartMeta } from '../../wailsjs/go/app/IRHandler'
  import type { dto } from '../../wailsjs/go/models'
  import ChartInfoCard from '../components/ChartInfoCard.svelte'
  import IRInfoCard from '../components/IRInfoCard.svelte'
  import OpenFolderButton from '../components/OpenFolderButton.svelte'
  import Icon from '../components/Icon.svelte'

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
          <OpenFolderButton path={chart?.path} title="インストール先フォルダを開く" />
          <button
            class="btn btn-ghost btn-xs"
            on:click={() => dispatch('close')}
          >
            <Icon name="close" cls="h-4 w-4" />
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
