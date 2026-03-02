<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { RunAutoInference } from '../wailsjs/go/app/InferenceHandler'
  import { UpdateSongMeta } from '../wailsjs/go/app/SongHandler'
  import type { dto } from '../wailsjs/go/models'

  const dispatch = createEventDispatcher<{ close: void }>()

  let dialog: HTMLDialogElement
  let mouseDownOnBackdrop = false

  // Phase管理
  let phase: 'loading' | 'result' | 'manual' = 'loading'
  let error = ''

  // 自動推測結果
  let result: dto.InferenceResultDTO | null = null

  // 手動確認の状態
  let currentIndex = 0
  let manualEventName = ''
  let manualReleaseYear = 0
  let saving = false

  export async function open() {
    error = ''
    result = null
    phase = 'loading'
    currentIndex = 0
    dialog.showModal()
    await runAuto()
  }

  async function runAuto() {
    try {
      result = await RunAutoInference()
      phase = 'result'
    } catch (e: any) {
      error = e?.message || '自動推測に失敗しました'
      phase = 'result'
    }
  }

  function startManual() {
    if (!result || !result.unmatchedSongs || result.unmatchedSongs.length === 0) return
    currentIndex = 0
    resetManualForm()
    phase = 'manual'
  }

  function resetManualForm() {
    manualEventName = ''
    manualReleaseYear = 0
  }

  $: currentSong = (result?.unmatchedSongs && currentIndex < result.unmatchedSongs.length)
    ? result.unmatchedSongs[currentIndex]
    : null

  async function handleSaveAndNext() {
    if (!currentSong) return
    saving = true
    error = ''
    try {
      // event_name と release_year を nullable で渡す
      const eventVal = manualEventName.trim() || null
      const yearVal = manualReleaseYear || null
      await UpdateSongMeta(currentSong.folderHash, yearVal, eventVal)
      goNext()
    } catch (e: any) {
      error = e?.message || '保存に失敗しました'
    } finally {
      saving = false
    }
  }

  function handleSkip() {
    goNext()
  }

  function goNext() {
    error = ''
    if (result && currentIndex < result.unmatchedSongs.length - 1) {
      currentIndex++
      resetManualForm()
    } else {
      handleClose()
    }
  }

  function handleClose() {
    dialog.close()
    dispatch('close')
  }
</script>

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<dialog bind:this={dialog} class="modal"
  on:mousedown|self={() => mouseDownOnBackdrop = true}
  on:click|self={() => { if (mouseDownOnBackdrop) dialog.close(); mouseDownOnBackdrop = false }}>
  <div class="modal-box max-w-2xl">
    <h3 class="text-lg font-bold mb-4">メタ情報推測</h3>

    {#if error}
      <div class="alert alert-error mb-4 py-2 text-sm">{error}</div>
    {/if}

    <!-- Phase 1: ローディング -->
    {#if phase === 'loading'}
      <div class="flex flex-col items-center justify-center py-8 gap-4">
        <span class="loading loading-spinner loading-lg"></span>
        <p class="text-sm text-base-content/70">自動推測を実行中...</p>
      </div>

    <!-- Phase 1: 結果サマリー -->
    {:else if phase === 'result'}
      {#if result}
        <div class="space-y-2 py-4">
          <p class="text-sm">
            <span class="font-semibold">{result.autoSetCount}</span> 曲を自動設定しました
          </p>
          {#if result.unmatchedSongs && result.unmatchedSongs.length > 0}
            <p class="text-sm">
              <span class="font-semibold">{result.unmatchedSongs.length}</span> 曲が未マッチです
              {#if result.noIRCount > 0}
                <span class="text-base-content/50">（うちIR未取得 {result.noIRCount} 曲）</span>
              {/if}
            </p>
          {:else}
            <p class="text-sm text-success">すべての曲が自動マッチしました</p>
          {/if}
        </div>

        {#if result.unmatchedSongs && result.unmatchedSongs.length > 0}
          <div class="modal-action">
            <button class="btn" on:click={handleClose}>閉じる</button>
            <button class="btn btn-primary" on:click={startManual}>手動確認を開始</button>
          </div>
        {:else}
          <div class="modal-action">
            <button class="btn" on:click={handleClose}>閉じる</button>
          </div>
        {/if}
      {:else}
        <div class="modal-action">
          <button class="btn" on:click={handleClose}>閉じる</button>
        </div>
      {/if}

    <!-- Phase 2: 手動確認 -->
    {:else if phase === 'manual' && currentSong}
      <div class="flex items-center justify-between mb-4">
        <span class="badge badge-outline">{currentIndex + 1} / {result?.unmatchedSongs.length}</span>
      </div>

      <div class="space-y-2 mb-4">
        <div class="grid grid-cols-[4rem_1fr] gap-1 text-sm">
          <span class="text-base-content/50">Title</span>
          <span class="font-semibold">{currentSong.title}</span>
          <span class="text-base-content/50">Artist</span>
          <span>{currentSong.artist}</span>
          <span class="text-base-content/50">Genre</span>
          <span>{currentSong.genre}</span>
          <span class="text-base-content/50">IR</span>
          <span>
            {currentSong.chartCount} 譜面中 {currentSong.irCount} 件取得済み
          </span>
        </div>

        {#if currentSong.bodyUrls && currentSong.bodyUrls.length > 0}
          <div class="mt-2">
            <p class="text-xs text-base-content/50 mb-1">IR URL一覧:</p>
            <div class="space-y-1 max-h-32 overflow-y-auto">
              {#each currentSong.bodyUrls as url}
                <a
                  href={url}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="link link-primary text-xs block truncate"
                >{url}</a>
              {/each}
            </div>
          </div>
        {/if}
      </div>

      <div class="flex gap-2 items-end mb-4">
        <div class="form-control flex-1">
          <label class="label py-0" for="manual-event-name">
            <span class="label-text text-xs">イベント名</span>
          </label>
          <input
            id="manual-event-name"
            type="text"
            class="input input-bordered input-sm"
            bind:value={manualEventName}
            placeholder="BOF2024"
          />
        </div>
        <div class="form-control w-24">
          <label class="label py-0" for="manual-release-year">
            <span class="label-text text-xs">年</span>
          </label>
          <input
            id="manual-release-year"
            type="number"
            class="input input-bordered input-sm"
            bind:value={manualReleaseYear}
            placeholder="2024"
          />
        </div>
      </div>

      <div class="modal-action">
        <button class="btn btn-sm" on:click={handleClose}>終了</button>
        <button class="btn btn-sm btn-ghost" on:click={handleSkip}>スキップ</button>
        <button
          class="btn btn-sm btn-primary"
          on:click={handleSaveAndNext}
          disabled={saving || (!manualEventName.trim() && !manualReleaseYear)}
        >
          {saving ? '保存中...' : '保存して次へ'}
        </button>
      </div>
    {/if}
  </div>
</dialog>
