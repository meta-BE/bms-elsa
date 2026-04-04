<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte'
  import { dndzone } from 'svelte-dnd-action'
  import { flip } from 'svelte/animate'
  import { ListDifficultyTables, AddDifficultyTable, RemoveDifficultyTable, RefreshAllDifficultyTablesAsync, StopDifficultyTableRefresh, IsRefreshing, RefreshProgress, ReorderDifficultyTables } from '../../wailsjs/go/app/DifficultyTableHandler'
  import { EventsOn } from '../../wailsjs/runtime/runtime'
  import ProgressBar from '../components/ProgressBar.svelte'

  const dispatch = createEventDispatcher()

  let dialog: HTMLDialogElement
  let mouseDownOnBackdrop = false
  let tables: any[] = []
  let newTableURL = ''
  let addError = ''
  let refreshResults: any[] | null = null
  let refreshing = false
  let refreshProgress = { current: 0, total: 0 }
  let adding = false

  let offProgress: (() => void) | null = null
  let offDone: (() => void) | null = null

  onMount(async () => {
    // ダイアログ再オープン時の進捗復元
    const running = await IsRefreshing()
    if (running) {
      refreshing = true
      const p = await RefreshProgress()
      refreshProgress = { current: p.current || 0, total: p.total || 0 }
    }

    offProgress = EventsOn('dt:refresh-progress', (data: { current: number; total: number }) => {
      if (!refreshing) return
      refreshProgress = { current: data.current, total: data.total }
    })
    offDone = EventsOn('dt:refresh-done', (data: { results: any[] }) => {
      if (!refreshing) return
      refreshing = false
      refreshResults = data.results
      loadTables()
    })
  })

  onDestroy(() => {
    offProgress?.()
    offDone?.()
  })

  const flipDurationMs = 200

  function handleDndConsider(e: CustomEvent) {
    tables = e.detail.items
  }

  async function handleDndFinalize(e: CustomEvent) {
    tables = e.detail.items
    const ids = tables.map((t: any) => t.id)
    try {
      await ReorderDifficultyTables(ids)
    } catch (e) {
      console.error('並び替え保存に失敗:', e)
      await loadTables()
    }
  }

  export async function open() {
    addError = ''
    newTableURL = ''
    refreshResults = null
    // onMountは初回のみなので、再オープン時もバックエンドと状態を同期する
    const running = await IsRefreshing()
    if (running) {
      refreshing = true
      const p = await RefreshProgress()
      refreshProgress = { current: p.current || 0, total: p.total || 0 }
    } else {
      refreshing = false
    }
    await loadTables()
    dialog.showModal()
  }

  async function loadTables() {
    try {
      tables = await ListDifficultyTables() || []
    } catch (e) {
      tables = []
    }
  }

  async function handleAddTable() {
    if (!newTableURL.trim()) return
    addError = ''
    adding = true
    try {
      await AddDifficultyTable(newTableURL.trim())
      newTableURL = ''
      await loadTables()
    } catch (e: any) {
      addError = e?.message || '追加に失敗しました'
    } finally {
      adding = false
    }
  }

  async function handleRemoveTable(id: number) {
    await RemoveDifficultyTable(id)
    await loadTables()
  }

  async function handleRefreshAll() {
    refreshing = true
    refreshResults = null
    refreshProgress = { current: 0, total: 0 }
    try {
      await RefreshAllDifficultyTablesAsync()
    } catch (e: any) {
      refreshing = false
      refreshResults = [{ tableName: '', success: false, error: e?.message || '更新に失敗しました' }]
    }
  }

  function handleStopRefresh() {
    StopDifficultyTableRefresh()
  }

  function handleClose() {
    dialog.close()
    dispatch('close')
  }
</script>

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<dialog bind:this={dialog} class="modal"
  on:mousedown|self={() => mouseDownOnBackdrop = true}
  on:click|self={() => { if (mouseDownOnBackdrop) { dialog.close(); dispatch('close') } mouseDownOnBackdrop = false }}>
  <div class="modal-box max-w-2xl">
    <h3 class="text-lg font-bold mb-4">難易度表設定</h3>

    {#if tables.length > 0}
      <div class="overflow-x-auto">
        <table class="table table-xs">
          <thead>
            <tr>
              <th class="w-6"></th>
              <th>名前</th>
              <th>記号</th>
              <th>譜面数</th>
              <th>最終取得</th>
              <th></th>
            </tr>
          </thead>
          <tbody use:dndzone={{ items: tables, flipDurationMs }} on:consider={handleDndConsider} on:finalize={handleDndFinalize}>
            {#each tables as t (t.id)}
              <tr animate:flip={{ duration: flipDurationMs }}>
                <td class="cursor-grab text-base-content/30 text-center select-none">⠿</td>
                <td>{t.name}</td>
                <td>{t.symbol}</td>
                <td>{t.entryCount}</td>
                <td class="text-xs text-base-content/50">{t.fetchedAt || '未取得'}</td>
                <td>
                  <button class="btn btn-ghost btn-xs text-error" on:click={() => handleRemoveTable(t.id)}>削除</button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}
      <p class="text-sm text-base-content/50">難易度表が登録されていません</p>
    {/if}

    <div class="flex gap-2 mt-2">
      <input
        type="text"
        class="input input-bordered input-sm flex-1"
        bind:value={newTableURL}
        placeholder="https://stellabms.xyz/st/table.html"
        on:keydown={(e) => e.key === 'Enter' && handleAddTable()}
      />
      <button class="btn btn-sm btn-outline" on:click={handleAddTable} disabled={adding}>
        {adding ? '追加中...' : '追加'}
      </button>
    </div>
    {#if addError}
      <div class="alert alert-error mt-2 py-1 text-sm">{addError}</div>
    {/if}

    {#if tables.length > 0}
      <div class="flex items-center gap-2 mt-2">
        {#if refreshing}
          <ProgressBar current={refreshProgress.current} total={refreshProgress.total} cancelable on:cancel={handleStopRefresh} />
        {:else}
          <button class="btn btn-sm btn-outline" on:click={handleRefreshAll}>全て更新</button>
        {/if}
      </div>
    {/if}

    {#if refreshResults}
      <div class="mt-2 text-sm space-y-1">
        {#each refreshResults as r}
          <div class="flex items-center gap-2">
            <span class={r.success ? 'text-success' : 'text-error'}>{r.success ? '✓' : '✗'}</span>
            <span>{r.tableName}</span>
            {#if r.success}
              <span class="text-base-content/50">{r.entryCount}件</span>
            {:else}
              <span class="text-error">{r.error}</span>
            {/if}
          </div>
        {/each}
      </div>
    {/if}

    <div class="modal-action">
      <button class="btn" on:click={handleClose}>閉じる</button>
    </div>
  </div>
</dialog>
