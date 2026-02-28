<script lang="ts">
  import { GetConfig, SaveConfig, SelectFile } from '../wailsjs/go/main/App'

  let dialog: HTMLDialogElement
  let songdataDBPath = ''
  let saved = false
  let error = ''

  export async function open() {
    saved = false
    error = ''
    try {
      const cfg = await GetConfig()
      songdataDBPath = cfg.songdataDBPath || ''
    } catch (e) {
      songdataDBPath = ''
    }
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
      await SaveConfig({ songdataDBPath })
      saved = true
    } catch (e: any) {
      error = e?.message || '保存に失敗しました'
    }
  }

  function handleClose() {
    dialog.close()
  }
</script>

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<dialog bind:this={dialog} class="modal" on:click|self={handleClose}>
  <div class="modal-box">
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
      <label class="label" for="songdata-path">
        <span class="label-text-alt text-base-content/50">
          未指定の場合は ~/.beatoraja/songdata.db → ~/beatoraja/songdata.db の順で自動検出
        </span>
      </label>
    </div>

    {#if saved}
      <div class="alert alert-success mt-4">
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
