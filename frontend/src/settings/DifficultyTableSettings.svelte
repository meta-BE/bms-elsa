<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { ListDifficultyTables, AddDifficultyTable, RemoveDifficultyTable, RefreshAllDifficultyTables } from '../../wailsjs/go/app/DifficultyTableHandler'

  const dispatch = createEventDispatcher()

  let dialog: HTMLDialogElement
  let mouseDownOnBackdrop = false
  let tables: any[] = []
  let newTableURL = ''
  let addError = ''
  let refreshResults: any[] | null = null
  let refreshing = false
  let adding = false

  export async function open() {
    addError = ''
    newTableURL = ''
    refreshResults = null
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
    try {
      refreshResults = await RefreshAllDifficultyTables()
      await loadTables()
    } catch (e: any) {
      refreshResults = [{ tableName: '', success: false, error: e?.message || '更新に失敗しました' }]
    } finally {
      refreshing = false
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
  on:click|self={() => { if (mouseDownOnBackdrop) { dialog.close(); dispatch('close') } mouseDownOnBackdrop = false }}>
  <div class="modal-box max-w-2xl">
    <h3 class="text-lg font-bold mb-4">難易度表設定</h3>

    {#if tables.length > 0}
      <div class="overflow-x-auto">
        <table class="table table-xs">
          <thead>
            <tr>
              <th>名前</th>
              <th>記号</th>
              <th>譜面数</th>
              <th>最終取得</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {#each tables as t}
              <tr>
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
      <button class="btn btn-sm btn-outline mt-2" on:click={handleRefreshAll} disabled={refreshing}>
        {refreshing ? '更新中...' : '全て更新'}
      </button>
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
