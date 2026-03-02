<script lang="ts">
  import { ListEventMappings, UpsertEventMapping, DeleteEventMapping } from '../wailsjs/go/app/InferenceHandler'
  import type { dto } from '../wailsjs/go/models'

  let dialog: HTMLDialogElement
  let mouseDownOnBackdrop = false
  let mappings: dto.EventMappingDTO[] = []
  let error = ''

  // 追加フォーム
  let newUrlPattern = ''
  let newEventName = ''
  let newReleaseYear = 0
  let adding = false

  export async function open() {
    error = ''
    resetForm()
    await loadMappings()
    dialog.showModal()
  }

  function resetForm() {
    newUrlPattern = ''
    newEventName = ''
    newReleaseYear = 0
  }

  async function loadMappings() {
    try {
      mappings = (await ListEventMappings()) || []
    } catch (e: any) {
      mappings = []
      error = e?.message || 'マッピング一覧の取得に失敗しました'
    }
  }

  async function handleAdd() {
    if (!newUrlPattern.trim() || !newEventName.trim()) return
    adding = true
    error = ''
    try {
      await UpsertEventMapping(newUrlPattern.trim(), newEventName.trim(), newReleaseYear)
      resetForm()
      await loadMappings()
    } catch (e: any) {
      error = e?.message || '追加に失敗しました'
    } finally {
      adding = false
    }
  }

  async function handleDelete(id: number) {
    error = ''
    try {
      await DeleteEventMapping(id)
      await loadMappings()
    } catch (e: any) {
      error = e?.message || '削除に失敗しました'
    }
  }

  function handleClose() {
    dialog.close()
  }
</script>

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<dialog bind:this={dialog} class="modal"
  on:mousedown|self={() => mouseDownOnBackdrop = true}
  on:click|self={() => { if (mouseDownOnBackdrop) dialog.close(); mouseDownOnBackdrop = false }}>
  <div class="modal-box max-w-2xl">
    <h3 class="text-lg font-bold mb-4">イベントマッピング管理</h3>

    {#if error}
      <div class="alert alert-error mb-4 py-2 text-sm">{error}</div>
    {/if}

    {#if mappings.length > 0}
      <div class="overflow-x-auto">
        <table class="table table-xs">
          <thead>
            <tr>
              <th>URLパターン</th>
              <th>イベント名</th>
              <th>年</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {#each mappings as m}
              <tr>
                <td class="font-mono text-xs">{m.urlPattern}</td>
                <td>{m.eventName}</td>
                <td>{m.releaseYear || '-'}</td>
                <td>
                  <button class="btn btn-ghost btn-xs text-error" on:click={() => handleDelete(m.id)}>削除</button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}
      <p class="text-sm text-base-content/50">マッピングが登録されていません</p>
    {/if}

    <div class="divider text-sm">新規追加</div>

    <div class="flex gap-2 items-end">
      <div class="form-control flex-1">
        <label class="label py-0" for="url-pattern">
          <span class="label-text text-xs">URLパターン</span>
        </label>
        <input
          id="url-pattern"
          type="text"
          class="input input-bordered input-sm"
          bind:value={newUrlPattern}
          placeholder="https://example.com/event/"
        />
      </div>
      <div class="form-control flex-1">
        <label class="label py-0" for="event-name">
          <span class="label-text text-xs">イベント名</span>
        </label>
        <input
          id="event-name"
          type="text"
          class="input input-bordered input-sm"
          bind:value={newEventName}
          placeholder="BOF2024"
        />
      </div>
      <div class="form-control w-20">
        <label class="label py-0" for="release-year">
          <span class="label-text text-xs">年</span>
        </label>
        <input
          id="release-year"
          type="number"
          class="input input-bordered input-sm"
          bind:value={newReleaseYear}
          placeholder="2024"
        />
      </div>
      <button
        class="btn btn-sm btn-outline shrink-0"
        on:click={handleAdd}
        disabled={adding || !newUrlPattern.trim() || !newEventName.trim()}
      >
        {adding ? '追加中...' : '追加'}
      </button>
    </div>

    <div class="modal-action">
      <button class="btn" on:click={handleClose}>閉じる</button>
    </div>
  </div>
</dialog>
