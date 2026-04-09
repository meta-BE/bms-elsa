<script lang="ts">
  import { GetConfig, SaveConfig, SelectFile } from '../../wailsjs/go/main/App'
  import { onMount, onDestroy } from 'svelte'
  import { EventsOn, EventsEmit } from '../../wailsjs/runtime/runtime'
  import { IsMinHashScanRunning } from '../../wailsjs/go/app/ScanHandler'
  import { IsRefreshing, RefreshProgress } from '../../wailsjs/go/app/DifficultyTableHandler'
  import { IsInferring } from '../../wailsjs/go/app/RewriteHandler'
  import { IsDuplicateScanRunning } from '../../wailsjs/go/app/DuplicateHandler'
  import ProgressBar from '../components/ProgressBar.svelte'

  let dialog: HTMLDialogElement
  let songdataDBPath = ''
  let fileLog = false
  let saved = false
  let error = ''
  let mouseDownOnBackdrop = false

  // バックグラウンドタスクの状態
  let scanState: 'running' | 'done' | 'error' = 'done'
  let scanProgress = { current: 0, total: 0 }
  let scanError = ''
  let scanResult = ''

  let dtState: 'running' | 'done' | 'error' = 'done'
  let dtProgress = { current: 0, total: 0 }
  let dtError = ''
  let dtResult = ''

  let rewriteState: 'running' | 'done' | 'error' = 'done'
  let rewriteProgress = { current: 0, total: 0 }
  let rewriteError = ''
  let rewriteResult = ''

  let dupState: 'running' | 'done' | 'error' = 'done'
  let dupProgress = { current: 0, total: 0 }
  let dupError = ''
  let dupResult = ''

  export async function open() {
    saved = false
    error = ''
    try {
      const cfg = await GetConfig()
      songdataDBPath = cfg.songdataDBPath || ''
      fileLog = cfg.fileLog || false
    } catch (e) {
      songdataDBPath = ''
    }
    // バックグラウンドタスクの現在状態を取得
    try {
      if (await IsMinHashScanRunning()) {
        scanState = 'running'
      }
    } catch {}
    try {
      if (await IsRefreshing()) {
        dtState = 'running'
        const p = await RefreshProgress()
        dtProgress = { current: p.current, total: p.total }
      }
    } catch {}
    try {
      if (await IsInferring()) {
        rewriteState = 'running'
      }
    } catch {}
    try {
      if (await IsDuplicateScanRunning()) {
        dupState = 'running'
        dupProgress = { current: 0, total: 1 }
      }
    } catch {}
    dialog.showModal()
  }

  async function handleBrowse() {
    try {
      const path = await SelectFile()
      if (path) {
        songdataDBPath = path
      }
    } catch (e) {
      // キャンセル時は何もしない
    }
  }

  async function handleSave() {
    error = ''
    try {
      await SaveConfig({ songdataDBPath, fileLog })
      saved = true
    } catch (e: any) {
      error = e?.message || '保存に失敗しました'
    }
  }

  function handleClose() {
    dialog.close()
  }

  async function resetColumnWidths(viewId: string) {
    try {
      const cfg = await GetConfig()
      if (cfg.columnWidths) {
        const columnWidths = { ...cfg.columnWidths }
        delete columnWidths[viewId]
        await SaveConfig({ ...cfg, columnWidths })
      }
      EventsEmit('column-width-reset', viewId)
    } catch (e: any) {
      error = e?.message || 'リセットに失敗しました'
    }
  }

  let offScanProgress: (() => void) | null = null
  let offScanDone: (() => void) | null = null
  let offDtProgress: (() => void) | null = null
  let offDtDone: (() => void) | null = null
  let offRewriteProgress: (() => void) | null = null
  let offRewriteDone: (() => void) | null = null
  let offDupProgress: (() => void) | null = null
  let offDupDone: (() => void) | null = null

  onMount(() => {
    offScanProgress = EventsOn('scan:progress', (data: { current: number; total: number }) => {
      scanState = 'running'
      scanProgress = data
    })
    offScanDone = EventsOn('scan:done', (data: { total: number; computed: number; skipped: number; failed: number; cancelled: boolean }) => {
      if (data.failed > 0) {
        scanState = 'error'
        scanError = `${data.failed}件失敗`
      } else {
        scanState = 'done'
      }
      scanProgress = { current: data.total, total: data.total }
      const parts: string[] = []
      if (data.computed > 0) parts.push(`${data.computed}件計算`)
      if (data.skipped > 0) parts.push(`${data.skipped}件スキップ`)
      if (data.failed > 0) parts.push(`${data.failed}件失敗`)
      if (data.cancelled) parts.push('中断')
      scanResult = parts.join(' / ') || '対象なし'
    })
    offDtProgress = EventsOn('dt:refresh-progress', (data: { current: number; total: number; tableName: string; success: boolean; error: string }) => {
      dtState = 'running'
      dtProgress = { current: data.current, total: data.total }
      if (data.error) {
        dtError = `${data.tableName}: ${data.error}`
      }
    })
    offDtDone = EventsOn('dt:refresh-done', (data: { results: Array<{ tableName: string; success: boolean; error: string }> }) => {
      const succeeded = data.results.filter(r => !r.error)
      const errors = data.results.filter(r => r.error)
      if (errors.length > 0) {
        dtState = 'error'
        dtError = errors.map(e => e.tableName).join(', ')
        dtResult = `${succeeded.length}件成功 / ${errors.length}件失敗（${errors.map(e => e.tableName).join(', ')}）`
      } else {
        dtState = 'done'
        dtResult = `${succeeded.length}件成功`
      }
    })
    offRewriteProgress = EventsOn('rewrite:progress', (data: { current: number; total: number }) => {
      rewriteState = 'running'
      rewriteProgress = data
    })
    offRewriteDone = EventsOn('rewrite:done', (data: { applied: number; skipped: number; total: number; error: string }) => {
      if (data.error) {
        rewriteState = 'error'
        rewriteError = data.error
      } else {
        rewriteState = 'done'
        rewriteResult = `${data.applied}件適用 / ${data.skipped}件スキップ`
      }
    })
    offDupProgress = EventsOn('dup:progress', (data: { current: number; total: number }) => {
      dupState = 'running'
      dupProgress = data
    })
    offDupDone = EventsOn('dup:done', (data: { groups: number; error: string }) => {
      if (data.error) {
        dupState = 'error'
        dupError = data.error
      } else {
        dupState = 'done'
        dupResult = `${data.groups}グループ検出`
      }
      dupProgress = { current: 1, total: 1 }
    })
  })

  onDestroy(() => {
    offScanProgress?.()
    offScanDone?.()
    offDtProgress?.()
    offDtDone?.()
    offRewriteProgress?.()
    offRewriteDone?.()
    offDupProgress?.()
    offDupDone?.()
  })
</script>

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<dialog bind:this={dialog} class="modal"
  on:mousedown|self={() => mouseDownOnBackdrop = true}
  on:click|self={() => { if (mouseDownOnBackdrop) dialog.close(); mouseDownOnBackdrop = false }}>
  <div class="modal-box max-w-2xl">
    <h3 class="text-lg font-bold mb-4">設定</h3>

    <div class="form-control w-full">
      <label class="label" for="songdata-path">
        <span class="label-text">songdata.db のパス</span>
      </label>
      <div class="flex gap-2">
        <input
          id="songdata-path"
          type="text"
          class="input input-bordered flex-1"
          bind:value={songdataDBPath}
          placeholder="/path/to/beatoraja/songdata.db"
        />
        <button class="btn btn-outline" on:click={handleBrowse}>参照</button>
      </div>
    </div>

    <div class="form-control mt-4">
      <label class="label cursor-pointer justify-start gap-3">
        <input type="checkbox" class="checkbox" bind:checked={fileLog} />
        <div>
          <span class="label-text">ファイル別ログを出力</span>
          <span class="label-text-alt block text-base-content/50">
            フォルダマージ・差分導入時に個別ファイルの移動ログを system.log に記録します
          </span>
        </div>
      </label>
    </div>

    <div class="divider text-xs text-base-content/50">カラム幅</div>

    <div class="space-y-2">
      <p class="text-sm text-base-content/70">各テーブルのカラム幅をデフォルトに戻します</p>
      <div class="flex flex-wrap gap-2">
        <button class="btn btn-xs btn-outline" on:click={() => resetColumnWidths('chartList')}>楽曲一覧</button>
        <button class="btn btn-xs btn-outline" on:click={() => resetColumnWidths('songList')}>譜面一覧</button>
        <button class="btn btn-xs btn-outline" on:click={() => resetColumnWidths('difficultyTable')}>難易度表</button>
        <button class="btn btn-xs btn-outline" on:click={() => resetColumnWidths('diffImport')}>差分導入</button>
      </div>
    </div>

    <!-- バックグラウンドタスク -->
    <div class="divider text-xs text-base-content/50">バックグラウンドタスク</div>

    <div class="space-y-3">
      <!-- MinHashスキャン -->
      <div>
        <div class="flex items-center justify-between text-sm mb-1">
          <span>MinHashスキャン</span>
          {#if scanState === 'running'}
            <span class="text-xs text-base-content/50">実行中...</span>
          {:else if scanState === 'error'}
            <span class="text-xs text-error">エラー</span>
          {:else}
            <span class="text-xs text-success">完了</span>
          {/if}
        </div>
        {#if scanState === 'running' && scanProgress.total > 0}
          <ProgressBar current={scanProgress.current} total={scanProgress.total} />
        {/if}
        {#if scanState !== 'running' && scanResult}
          <p class="text-xs text-base-content/50">{scanResult}</p>
        {/if}
      </div>

      <!-- 難易度表更新 -->
      <div>
        <div class="flex items-center justify-between text-sm mb-1">
          <span>難易度表更新</span>
          {#if dtState === 'running'}
            <span class="text-xs text-base-content/50">実行中...</span>
          {:else if dtState === 'error'}
            <span class="text-xs text-error">エラー</span>
          {:else}
            <span class="text-xs text-success">完了</span>
          {/if}
        </div>
        {#if dtState === 'running' && dtProgress.total > 0}
          <ProgressBar current={dtProgress.current} total={dtProgress.total} />
        {/if}
        {#if dtState !== 'running' && dtResult}
          <p class="text-xs text-base-content/50">{dtResult}</p>
        {/if}
      </div>

      <!-- 動作URL推定 -->
      <div>
        <div class="flex items-center justify-between text-sm mb-1">
          <span>動作URL推定</span>
          {#if rewriteState === 'running'}
            <span class="text-xs text-base-content/50">実行中...</span>
          {:else if rewriteState === 'error'}
            <span class="text-xs text-error">エラー</span>
          {:else}
            <span class="text-xs text-success">完了</span>
          {/if}
        </div>
        {#if rewriteState === 'running' && rewriteProgress.total > 0}
          <ProgressBar current={rewriteProgress.current} total={rewriteProgress.total} />
        {/if}
        {#if rewriteState !== 'running' && rewriteResult}
          <p class="text-xs text-base-content/50">{rewriteResult}</p>
        {/if}
      </div>

      <!-- 重複検知スキャン -->
      <div>
        <div class="flex items-center justify-between text-sm mb-1">
          <span>重複検知スキャン</span>
          {#if dupState === 'running'}
            <span class="text-xs text-base-content/50">実行中...</span>
          {:else if dupState === 'error'}
            <span class="text-xs text-error">エラー</span>
          {:else}
            <span class="text-xs text-success">完了</span>
          {/if}
        </div>
        {#if dupState === 'running' && dupProgress.total > 0}
          <ProgressBar current={dupProgress.current} total={dupProgress.total} />
        {/if}
        {#if dupState !== 'running' && dupResult}
          <p class="text-xs text-base-content/50">{dupResult}</p>
        {/if}
      </div>
    </div>

    {#if saved}
      <div class="alert alert-info mt-4">
        <span>保存しました。設定を反映するにはアプリを再起動してください。</span>
      </div>
    {/if}

    {#if error}
      <div class="alert alert-error mt-4">
        <span>{error}</span>
      </div>
    {/if}

    <div class="modal-action">
      <button class="btn" on:click={handleClose}>閉じる</button>
      <button class="btn btn-primary" on:click={handleSave}>保存</button>
    </div>
  </div>
</dialog>
