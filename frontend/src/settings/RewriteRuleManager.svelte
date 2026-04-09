<script lang="ts">
  import { ListRewriteRules, UpsertRewriteRule, DeleteRewriteRule } from '../../wailsjs/go/app/RewriteHandler'
  import type { dto } from '../../wailsjs/go/models'
  import { rewriteRules } from '../stores/rewriteRules'

  let dialog: HTMLDialogElement
  let mouseDownOnBackdrop = false
  let rules: dto.RewriteRuleDTO[] = []
  let error = ''

  let newRuleType = 'replace'
  let newPattern = ''
  let newReplacement = ''
  let newPriority = 0
  let adding = false

  export async function open() {
    error = ''
    resetForm()
    await loadRules()
    dialog.showModal()
  }

  function resetForm() {
    newRuleType = 'replace'
    newPattern = ''
    newReplacement = ''
    newPriority = 0
  }

  async function loadRules() {
    try {
      rules = (await ListRewriteRules()) || []
      rewriteRules.set(rules)
    } catch (e: any) {
      rules = []
      error = e?.message || 'ルール一覧の取得に失敗しました'
    }
  }

  async function handleAdd() {
    if (!newPattern.trim() || !newReplacement.trim()) return
    adding = true
    error = ''
    try {
      await UpsertRewriteRule(0, newRuleType, newPattern.trim(), newReplacement.trim(), newPriority)
      resetForm()
      await loadRules()
    } catch (e: any) {
      error = e?.message || '追加に失敗しました'
    } finally {
      adding = false
    }
  }

  async function handleDelete(id: number) {
    error = ''
    try {
      await DeleteRewriteRule(id)
      await loadRules()
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
    <h3 class="text-lg font-bold mb-4">URL書き換えルール管理</h3>

    {#if error}
      <div class="alert alert-error mb-4 py-2 text-sm">{error}</div>
    {/if}

    {#if rules.length > 0}
      <div class="overflow-x-auto">
        <table class="table table-xs">
          <thead>
            <tr>
              <th>タイプ</th>
              <th>パターン</th>
              <th>置換先</th>
              <th>優先度</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {#each rules as r}
              <tr>
                <td><span class="badge badge-sm badge-outline">{r.ruleType}</span></td>
                <td class="font-mono text-xs">{r.pattern}</td>
                <td class="font-mono text-xs">{r.replacement}</td>
                <td>{r.priority}</td>
                <td>
                  <button class="btn btn-ghost btn-xs text-error" on:click={() => handleDelete(r.id)}>削除</button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}
      <p class="text-sm text-base-content/50">ルールが登録されていません</p>
    {/if}

    <div class="divider text-sm">新規追加</div>

    <div class="flex gap-2 items-end flex-wrap">
      <div class="form-control w-24">
        <label class="label py-0" for="rule-type">
          <span class="label-text text-xs">タイプ</span>
        </label>
        <select id="rule-type" class="select select-bordered select-sm" bind:value={newRuleType}>
          <option value="replace">replace</option>
          <option value="regex">regex</option>
        </select>
      </div>
      <div class="form-control flex-1">
        <label class="label py-0" for="rule-pattern">
          <span class="label-text text-xs">パターン</span>
        </label>
        <input
          id="rule-pattern"
          type="text"
          class="input input-bordered input-sm"
          bind:value={newPattern}
          placeholder="old-host.com/path"
        />
      </div>
      <div class="form-control flex-1">
        <label class="label py-0" for="rule-replacement">
          <span class="label-text text-xs">置換先</span>
        </label>
        <input
          id="rule-replacement"
          type="text"
          class="input input-bordered input-sm"
          bind:value={newReplacement}
          placeholder="new-host.com/path"
        />
      </div>
      <div class="form-control w-20">
        <label class="label py-0" for="rule-priority">
          <span class="label-text text-xs">優先度</span>
        </label>
        <input
          id="rule-priority"
          type="number"
          class="input input-bordered input-sm"
          bind:value={newPriority}
        />
      </div>
      <button
        class="btn btn-sm btn-outline shrink-0"
        on:click={handleAdd}
        disabled={adding || !newPattern.trim() || !newReplacement.trim()}
      >
        {adding ? '追加中...' : '追加'}
      </button>
    </div>

    <div class="modal-action">
      <button class="btn" on:click={handleClose}>閉じる</button>
    </div>
  </div>
</dialog>
