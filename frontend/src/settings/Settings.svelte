<script lang="ts">
  import { GetConfig, SaveConfig, SelectFile } from '../../wailsjs/go/main/App'

  let dialog: HTMLDialogElement
  let songdataDBPath = ''
  let fileLog = false
  let saved = false
  let error = ''
  let mouseDownOnBackdrop = false

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
