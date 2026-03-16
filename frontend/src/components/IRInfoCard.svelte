<script lang="ts">
  import { createEventDispatcher } from 'svelte'

  const dispatch = createEventDispatcher<{
    lookup: void
    save: { bodyUrl: string; diffUrl: string }
  }>()

  export let md5: string
  // ChartDTO または ChartIRMetaDTO（両方ともIR関連フィールドを持つ）
  export let ir: {
    hasIrMeta: boolean
    lr2irTags?: string
    lr2irBodyUrl?: string
    lr2irDiffUrl?: string
    lr2irNotes?: string
    workingBodyUrl?: string
    workingDiffUrl?: string
  } | null = null

  function linkify(text: string): string {
    const escaped = text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    return escaped.replace(
      /https?:\/\/[^\s<]+/g,
      url => `<a href="${url}" target="_blank" rel="noopener noreferrer" class="link link-primary">${url}</a>`
    )
  }

  let editingWorkingUrl = false
  let editWorkingBodyUrl = ''
  let editWorkingDiffUrl = ''
  let lastMd5 = ''

  // 別のアイテムに切り替わったら編集状態をリセット
  $: if (md5 !== lastMd5) {
    lastMd5 = md5
    editingWorkingUrl = false
  }

  // 編集中でなければ ir の値を同期
  $: if (ir && !editingWorkingUrl) {
    editWorkingBodyUrl = ir.workingBodyUrl || ''
    editWorkingDiffUrl = ir.workingDiffUrl || ''
  }

  function saveAndClose() {
    dispatch('save', { bodyUrl: editWorkingBodyUrl, diffUrl: editWorkingDiffUrl })
    editingWorkingUrl = false
  }
</script>

<div class="bg-base-200 rounded-lg p-3">
  <div class="flex items-center justify-between mb-2">
    <h3 class="text-sm font-semibold"><a href="http://www.dream-pro.info/~lavalse/LR2IR/search.cgi?mode=ranking&bmsmd5={md5}" target="_blank" rel="noopener noreferrer" class="link link-primary">LR2IR情報</a></h3>
    <button class="btn btn-ghost btn-xs" on:click={() => dispatch('lookup')}>IR取得</button>
  </div>
  {#if ir?.hasIrMeta}
    <div class="text-xs space-y-1">
      {#if ir.lr2irTags}
        <p><span class="font-semibold">タグ:</span> {ir.lr2irTags}</p>
      {/if}
      {#if ir.lr2irBodyUrl}
        <p>
          <span class="font-semibold">本体URL:</span>
          <a href={ir.lr2irBodyUrl} target="_blank" rel="noopener noreferrer" class="link link-primary">{ir.lr2irBodyUrl}</a>
        </p>
      {/if}
      {#if ir.lr2irDiffUrl}
        <p>
          <span class="font-semibold">差分URL:</span>
          <a href={ir.lr2irDiffUrl} target="_blank" rel="noopener noreferrer" class="link link-primary">{ir.lr2irDiffUrl}</a>
        </p>
      {/if}
      {#if ir.lr2irNotes}
        <p class="whitespace-pre-wrap"><span class="font-semibold">備考:</span> {@html linkify(ir.lr2irNotes)}</p>
      {/if}
      <div class="divider my-1"></div>
      {#if editingWorkingUrl}
        <div class="flex gap-2 items-center">
          <span class="font-semibold">動作URL(本体):</span>
          <input class="input input-xs input-bordered flex-1" bind:value={editWorkingBodyUrl} on:blur={saveAndClose} />
        </div>
        <div class="flex gap-2 items-center">
          <span class="font-semibold">動作URL(差分):</span>
          <input class="input input-xs input-bordered flex-1" bind:value={editWorkingDiffUrl} on:blur={saveAndClose} />
        </div>
      {:else}
        <div class="flex gap-2 items-center">
          <span class="font-semibold">動作URL(本体):</span>
          {#if editWorkingBodyUrl}
            <a href={editWorkingBodyUrl} target="_blank" rel="noopener noreferrer" class="link link-primary text-xs truncate flex-1">{editWorkingBodyUrl}</a>
          {:else}
            <span class="text-base-content/30 text-xs">未設定</span>
          {/if}
        </div>
        <div class="flex gap-2 items-center justify-between">
          <div class="flex gap-2 items-center flex-1 min-w-0">
            <span class="font-semibold">動作URL(差分):</span>
            {#if editWorkingDiffUrl}
              <a href={editWorkingDiffUrl} target="_blank" rel="noopener noreferrer" class="link link-primary text-xs truncate flex-1">{editWorkingDiffUrl}</a>
            {:else}
              <span class="text-base-content/30 text-xs">未設定</span>
            {/if}
          </div>
          <button class="btn btn-ghost btn-xs" on:click|stopPropagation={() => editingWorkingUrl = true}>
            編集
          </button>
        </div>
      {/if}
    </div>
  {:else}
    <p class="text-xs text-base-content/50">IR情報がありません。「IR取得」ボタンで取得してください。</p>
  {/if}
</div>
