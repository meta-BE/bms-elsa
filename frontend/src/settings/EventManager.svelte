<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { ListEvents, UpdateEventShortName, RefreshEventsFromBMSSearch } from '../../wailsjs/go/app/EventHandler'

  const dispatch = createEventDispatcher()

  let dialog: HTMLDialogElement
  let mouseDownOnBackdrop = false
  let events: any[] = []
  let refreshing = false
  let refreshResult = ''

  export async function open() {
    refreshResult = ''
    await loadEvents()
    dialog.showModal()
  }

  async function loadEvents() {
    try {
      events = (await ListEvents()) || []
    } catch (e) {
      events = []
    }
  }

  async function handleShortNameChange(id: number, value: string) {
    if (!value.trim()) return
    await UpdateEventShortName(id, value.trim())
  }

  async function handleRefresh() {
    refreshing = true
    refreshResult = ''
    try {
      const added = await RefreshEventsFromBMSSearch()
      refreshResult = `${added}件の新規イベントを追加`
      await loadEvents()
    } catch (e: any) {
      refreshResult = e?.message || '更新に失敗しました'
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
  <div class="modal-box max-w-4xl max-h-[80vh]">
    <h3 class="text-lg font-bold mb-4">イベントマスター管理</h3>

    <div class="flex items-center gap-2 mb-4">
      <button class="btn btn-sm btn-outline" on:click={handleRefresh} disabled={refreshing}>
        {refreshing ? 'BMS Search取得中...' : 'BMS Searchから更新'}
      </button>
      {#if refreshResult}
        <span class="text-sm text-success">{refreshResult}</span>
      {/if}
    </div>

    {#if events.length > 0}
      <div class="overflow-y-auto max-h-[50vh]">
        <table class="table table-xs">
          <thead class="sticky top-0 bg-base-100 z-10">
            <tr>
              <th>正式名称</th>
              <th>短縮名</th>
              <th class="w-16">年</th>
            </tr>
          </thead>
          <tbody>
            {#each events as ev (ev.id)}
              <tr>
                <td class="text-xs max-w-xs truncate" title={ev.name}>{ev.name}</td>
                <td>
                  <input
                    class="input input-xs input-bordered w-full"
                    value={ev.shortName}
                    on:blur={(e) => handleShortNameChange(ev.id, e.currentTarget.value)}
                  />
                </td>
                <td class="text-xs text-center">{ev.releaseYear}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}
      <p class="text-sm text-base-content/50">イベントが登録されていません</p>
    {/if}

    <div class="modal-action">
      <button class="btn" on:click={handleClose}>閉じる</button>
    </div>
  </div>
</dialog>
