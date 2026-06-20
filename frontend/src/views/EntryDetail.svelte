<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { GetChartDetailByMD5, GetChartMetaByMD5, ListChartPathsByMD5, DeleteChartFile } from '../../wailsjs/go/app/ChartHandler'
  import { GetDifficultyTableEntry } from '../../wailsjs/go/app/DifficultyTableHandler'
  import { LookupByMD5 } from '../../wailsjs/go/app/IRHandler'
  import type { dto } from '../../wailsjs/go/models'
  import BMSSearchInfoCard from '../components/BMSSearchInfoCard.svelte'
  import {
    GetBMSSearchInfoByMD5,
    LookupBMSSearchByMD5,
    UnlinkBMSSearchByMD5,
  } from '../../wailsjs/go/app/BMSSearchHandler'
  import ChartInfoCard from '../components/ChartInfoCard.svelte'
  import IRInfoCard from '../components/IRInfoCard.svelte'
  import InstallCandidateCard from '../components/InstallCandidateCard.svelte'
  import OpenFolderButton from '../components/OpenFolderButton.svelte'
  import Icon from '../components/Icon.svelte'
  import AlertModal from '../components/AlertModal.svelte'

  const dispatch = createEventDispatcher<{ close: void }>()

  export let md5: string
  export let tableID: number

  let entryData: dto.DifficultyTableEntryDTO | null = null
  let chart: dto.ChartDTO | null = null
  let irMeta: dto.ChartIRMetaDTO | null = null
  let loading = false
  let bmsSearchInfo: dto.BMSSearchInfoDTO | null = null
  let bmsSearchLoading = false
  let dupPaths: string[] = []
  // ファイル削除
  let deleteDialog: HTMLDialogElement
  let mouseDownOnBackdrop = false
  let pendingDeletePath = ''
  let alertModal: AlertModal

  $: if (md5 && tableID) loadEntry(md5, tableID)

  // IR情報の統一アクセス（chart or irMeta）
  $: ir = chart ?? irMeta

  async function loadEntry(hash: string, tid: number) {
    loading = true
    entryData = null
    chart = null
    irMeta = null
    dupPaths = []
    try {
      entryData = await GetDifficultyTableEntry(tid, hash)
      if (entryData?.status === 'duplicate') {
        try {
          dupPaths = (await ListChartPathsByMD5(hash)) ?? []
        } catch (e) {
          console.error('Failed to load duplicate paths:', e)
          dupPaths = []
        }
      }
      chart = await GetChartDetailByMD5(hash, '')
      if (!chart) {
        irMeta = await GetChartMetaByMD5(hash)
      }
      try {
        bmsSearchInfo = await GetBMSSearchInfoByMD5(hash)
      } catch (e) {
        console.error('Failed to load BMS Search info:', e)
        bmsSearchInfo = null
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

  async function lookupBMSSearch() {
    bmsSearchLoading = true
    try {
      bmsSearchInfo = await LookupBMSSearchByMD5(md5)
    } finally {
      bmsSearchLoading = false
    }
  }

  async function unlinkBMSSearch() {
    await UnlinkBMSSearchByMD5(md5)
    bmsSearchInfo = await GetBMSSearchInfoByMD5(md5)
  }

  function requestDelete(path: string) {
    pendingDeletePath = path
    deleteDialog.showModal()
  }

  async function executeDelete() {
    const path = pendingDeletePath
    deleteDialog.close()
    // 仕様上、削除後の一覧再読込・除去は行わない（削除済みパスは表示に残る）
    try {
      await DeleteChartFile(path)
    } catch (err) {
      alertModal.open(String(err))
    } finally {
      pendingDeletePath = ''
    }
  }

  function cancelDelete() {
    deleteDialog.close()
    pendingDeletePath = ''
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
          {#if entryData.status !== 'duplicate'}
            <OpenFolderButton path={chart?.path} title="インストール先フォルダを開く" />
          {/if}
          <button
            class="btn btn-ghost btn-xs"
            on:click={() => dispatch('close')}
          >
            <Icon name="close" cls="h-4 w-4" />
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

    <!-- ファイルパス一覧（重複時のみ） -->
    {#if entryData.status === 'duplicate' && dupPaths.length > 0}
      <div class="bg-base-200 rounded-lg p-3">
        <div class="text-sm font-semibold mb-2">ファイルパス一覧 ({dupPaths.length}件)</div>
        <div class="space-y-1">
          {#each dupPaths as p}
            <div class="text-xs text-base-content/70 flex items-center gap-1">
              <OpenFolderButton path={p} size="xs" title="フォルダを開く" />
              <span class="flex-1 break-all">{p}</span>
              <button
                class="btn btn-ghost btn-xs shrink-0"
                title="ファイルを削除"
                on:click={() => requestDelete(p)}
              >
                <Icon name="trash" cls="h-3 w-3" />
              </button>
            </div>
          {/each}
        </div>
      </div>
    {/if}

    <!-- 譜面メタデータ（導入済の場合のみ） -->
    {#if chart}
      <ChartInfoCard {chart} paneId="entry" />
    {/if}

    <!-- 導入先推定（未導入の場合のみ） -->
    {#if !chart}
      <InstallCandidateCard {md5} {tableID} paneId="entry" />
    {/if}

    <!-- IR情報（導入済・未導入共通） -->
    <IRInfoCard {md5} {ir} paneId="entry" on:lookup={lookupIR} />
    <BMSSearchInfoCard
      info={bmsSearchInfo}
      loading={bmsSearchLoading}
      paneId="entry"
      on:lookup={lookupBMSSearch}
      on:unlink={unlinkBMSSearch}
    />
  </div>
{/if}

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<dialog bind:this={deleteDialog} class="modal"
  on:mousedown|self={() => mouseDownOnBackdrop = true}
  on:click|self={() => { if (mouseDownOnBackdrop) cancelDelete(); mouseDownOnBackdrop = false }}>
  <div class="modal-box max-w-2xl">
    <h3 class="text-lg font-bold mb-4">ファイル削除の確認</h3>
    <div class="space-y-2 text-sm">
      <p>このファイルを完全に削除します。元に戻せません。</p>
      <div class="bg-base-200 rounded p-3 break-all">{pendingDeletePath}</div>
    </div>
    <div class="modal-action">
      <button class="btn" on:click={cancelDelete}>キャンセル</button>
      <button class="btn btn-error" on:click={executeDelete}>削除</button>
    </div>
  </div>
</dialog>

<AlertModal bind:this={alertModal} />
