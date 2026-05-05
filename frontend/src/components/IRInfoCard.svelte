<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { rewriteRules } from '../stores/rewriteRules'
  import { applyRewriteRules } from '../lib/urlRewrite'
  import type { PaneId } from '../stores/cardCollapsed'
  import CollapsibleCard from './CollapsibleCard.svelte'

  const dispatch = createEventDispatcher<{
    lookup: void
  }>()

  export let md5: string
  export let ir: {
    hasIrMeta: boolean
    lr2irTags?: string
    lr2irBodyUrl?: string
    lr2irDiffUrl?: string
    lr2irNotes?: string
  } | null = null
  export let paneId: PaneId

  function linkify(text: string): string {
    const escaped = text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    return escaped.replace(
      /https?:\/\/[^\s<]+/g,
      url => {
        const rewritten = applyRewriteRules(url, $rewriteRules)
        return `<a href="${rewritten}" target="_blank" rel="noopener noreferrer" class="link link-primary">${rewritten}</a>`
      }
    )
  }
</script>

<CollapsibleCard {paneId} cardId="irInfo">
  <a slot="title" href="http://www.dream-pro.info/~lavalse/LR2IR/search.cgi?mode=ranking&bmsmd5={md5}" target="_blank" rel="noopener noreferrer" class="link link-primary">LR2IR情報</a>
  <button slot="actions" class="btn btn-ghost btn-xs" on:click={() => dispatch('lookup')}>IR取得</button>
  {#if ir?.hasIrMeta}
    <div class="text-xs space-y-1">
      {#if ir.lr2irTags}
        <p><span class="font-semibold">タグ:</span> {ir.lr2irTags}</p>
      {/if}
      {#if ir.lr2irBodyUrl}
        <p>
          <span class="font-semibold">本体URL:</span>
          <a href={applyRewriteRules(ir.lr2irBodyUrl, $rewriteRules)} target="_blank" rel="noopener noreferrer" class="link link-primary">{applyRewriteRules(ir.lr2irBodyUrl, $rewriteRules)}</a>
        </p>
      {/if}
      {#if ir.lr2irDiffUrl}
        <p>
          <span class="font-semibold">差分URL:</span>
          <a href={applyRewriteRules(ir.lr2irDiffUrl, $rewriteRules)} target="_blank" rel="noopener noreferrer" class="link link-primary">{applyRewriteRules(ir.lr2irDiffUrl, $rewriteRules)}</a>
        </p>
      {/if}
      {#if ir.lr2irNotes}
        <p class="whitespace-pre-wrap"><span class="font-semibold">備考:</span> {@html linkify(ir.lr2irNotes)}</p>
      {/if}
    </div>
  {:else}
    <p class="text-xs text-base-content/50">IR情報がありません。「IR取得」ボタンで取得してください。</p>
  {/if}
</CollapsibleCard>
