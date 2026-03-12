<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime'
  import { ParseAndEstimate, ExecuteImport, StopEstimate } from '../../wailsjs/go/app/DiffImportHandler'
  import type { app } from '../../wailsjs/go/models'
  import OpenFolderButton from '../components/OpenFolderButton.svelte'

  let candidates: app.DiffImportCandidateDTO[] = []
  let estimating = false
  let estimateProgress = { current: 0, total: 0 }
  let importResult: app.DiffImportResultDTO | null = null
  let importResultTimer: ReturnType<typeof setTimeout> | null = null
  let importing = false
  let dragging = false

  let offProgress: (() => void) | null = null
  let offDone: (() => void) | null = null

  export function handleFileDrop(paths: string[]) {
    dragging = false
    if (estimating) return
    handleDrop(paths)
  }

  onMount(() => {
    offProgress = EventsOn('diff-import:progress', (data: { current: number; total: number }) => {
      estimateProgress = data
    })

    offDone = EventsOn('diff-import:done', () => {
      estimating = false
    })
  })

  onDestroy(() => {
    offProgress?.()
    offDone?.()
    if (importResultTimer) clearTimeout(importResultTimer)
  })

  async function handleDrop(filePaths: string[]) {
    estimating = true
    estimateProgress = { current: 0, total: 0 }
    importResult = null
    try {
      const result = await ParseAndEstimate(filePaths)
      candidates = [...candidates, ...(result || [])]
    } catch (e: any) {
      console.error('ParseAndEstimate failed:', e)
    } finally {
      estimating = false
    }
  }

  function clearCandidate(index: number) {
    candidates = candidates.filter((_, i) => i !== index)
  }

  function clearDestFolder(index: number) {
    candidates = candidates.map((c, i) => {
      if (i === index) {
        return { ...c, destFolder: '' }
      }
      return c
    })
  }

  function clearAll() {
    candidates = []
    importResult = null
  }

  async function handleImport() {
    const toImport = candidates.filter(c => c.destFolder)
    if (toImport.length === 0) return

    importing = true
    importResult = null
    try {
      const result = await ExecuteImport(toImport)
      importResult = result
      // 導入成功分をリストから除去
      const importedPaths = new Set(toImport.map(c => c.filePath))
      candidates = candidates.filter(c => !importedPaths.has(c.filePath))
    } catch (e: any) {
      console.error('ExecuteImport failed:', e)
    } finally {
      importing = false
    }
  }

  function handleStopEstimate() {
    StopEstimate()
  }

  const matchMethodLabels: Record<string, string> = {
    minhash: 'WAV定義',
    ir: '本体URL',
    title: 'タイトル',
  }

  $: importableCount = candidates.filter(c => c.destFolder).length
</script>

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div
  class="h-full flex flex-col bg-base-100 rounded-lg border border-base-300 {dragging ? 'border-primary border-2' : ''}"
  on:dragover|preventDefault={() => { dragging = true }}
  on:dragleave|preventDefault={() => { dragging = false }}
  on:drop|preventDefault={() => { dragging = false }}
>
  <!-- ヘッダー -->
  <div class="px-4 py-2 bg-base-200 rounded-t-lg flex items-center justify-between gap-2">
    <span class="text-sm font-semibold shrink-0">
      {candidates.length} ファイル
      {#if importableCount > 0}
        ({importableCount} 件導入可能)
      {/if}
    </span>
    <div class="flex items-center gap-2">
      {#if estimating}
        <span class="text-xs text-base-content/70">
          推定中: {estimateProgress.current.toLocaleString()} / {estimateProgress.total.toLocaleString()}
        </span>
        <button class="btn btn-xs btn-error btn-outline" on:click|stopPropagation={handleStopEstimate}>停止</button>
      {/if}
      {#if importResult}
        <span class="text-xs text-success">
          導入完了: {importResult.success}件成功
          {#if importResult.failed > 0}
            , {importResult.failed}件失敗
          {/if}
        </span>
      {/if}
      {#if candidates.length > 0}
        <button class="btn btn-xs btn-ghost" on:click|stopPropagation={clearAll}>全クリア</button>
      {/if}
    </div>
  </div>

  {#if candidates.length === 0 && !estimating}
    <!-- 空状態 -->
    <div class="flex-1 flex items-center justify-center text-base-content/40">
      <div class="text-center">
        <svg xmlns="http://www.w3.org/2000/svg" class="h-12 w-12 mx-auto mb-2 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
        </svg>
        <p class="text-sm">BMS/BME/BMLファイルをドラッグ＆ドロップして差分を追加</p>
      </div>
    </div>
  {:else}
    <!-- テーブル -->
    <div class="flex-1 overflow-auto">
      <table class="table table-xs table-pin-rows">
        <thead>
          <tr>
            <th>ファイル名</th>
            <th>TITLE</th>
            <th>ARTIST</th>
            <th>推定先</th>
            <th class="w-16">スコア</th>
            <th class="w-20">推定方法</th>
            <th class="w-16"></th>
          </tr>
        </thead>
        <tbody>
          {#each candidates as candidate, i}
            <tr class="hover:bg-base-200">
              <td class="text-sm font-mono max-w-48">
                <span class="flex items-center gap-1">
                  <OpenFolderButton path={candidate.filePath} size="xs" title="ファイルのフォルダを開く" />
                  <span class="truncate" title={candidate.filePath}>{candidate.fileName}</span>
                </span>
              </td>
              <td class="text-sm truncate max-w-48">
                {candidate.title}{candidate.subtitle ? ' ' + candidate.subtitle : ''}
              </td>
              <td class="text-sm truncate max-w-48">
                {candidate.artist}{candidate.subartist ? ' ' + candidate.subartist : ''}
              </td>
              <td class="text-sm max-w-64">
                {#if candidate.destFolder}
                  <span class="flex items-center gap-1">
                    <OpenFolderButton path={candidate.destFolder} size="xs" title="推定先フォルダを開く" />
                    <span class="truncate text-success" title={candidate.destFolder}>{candidate.destFolder}</span>
                  </span>
                {:else}
                  <span class="text-base-content/30">-</span>
                {/if}
              </td>
              <td class="text-sm font-mono">
                {#if candidate.score > 0}
                  {Math.round(candidate.score * 10)}
                {:else}
                  -
                {/if}
              </td>
              <td class="text-xs">{matchMethodLabels[candidate.matchMethod] || candidate.matchMethod || '-'}</td>
              <td>
                <div class="flex gap-1">
                  <button
                    class="btn btn-xs btn-ghost"
                    title="推定先をクリア"
                    disabled={!candidate.destFolder}
                    on:click|stopPropagation={() => clearDestFolder(i)}
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </button>
                  <button
                    class="btn btn-xs btn-ghost text-error"
                    title="削除"
                    on:click|stopPropagation={() => clearCandidate(i)}
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                    </svg>
                  </button>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <!-- フッター -->
    <div class="px-4 py-2 border-t border-base-300 flex items-center justify-between">
      <div class="text-xs text-base-content/60">
        {#if importResult && importResult.errors && importResult.errors.length > 0}
          <details>
            <summary class="cursor-pointer text-error">エラー詳細 ({importResult.errors.length}件)</summary>
            <ul class="mt-1 ml-4 list-disc">
              {#each importResult.errors as err}
                <li>{err}</li>
              {/each}
            </ul>
          </details>
        {/if}
      </div>
      <button
        class="btn btn-sm btn-primary"
        disabled={importableCount === 0 || importing || estimating}
        on:click|stopPropagation={handleImport}
      >
        {#if importing}
          導入中...
        {:else}
          推定先に導入 ({importableCount})
        {/if}
      </button>
    </div>
  {/if}
</div>
