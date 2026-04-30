<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import type { dto } from '../../wailsjs/go/models'
  import { rewriteRules } from '../stores/rewriteRules'
  import { applyRewriteRules } from '../lib/urlRewrite'
  import { formatDateYMD } from '../utils/date'
  import Icon from './Icon.svelte'

  export let info: dto.BMSSearchInfoDTO | null = null
  export let loading = false

  const dispatch = createEventDispatcher<{
    lookup: void
    unlink: void
  }>()

  $: hasInfo = info?.hasInfo === true

  function previewUrl(p: dto.BMSSearchPreviewDTO): string {
    switch (p.service) {
      case 'YOUTUBE':
        return `https://www.youtube.com/watch?v=${p.parameter}`
      case 'NICONICO':
        return `https://www.nicovideo.jp/watch/${p.parameter}`
      case 'SOUNDCLOUD':
      default:
        return p.parameter
    }
  }

  function rewrite(url: string): string {
    return applyRewriteRules(url, $rewriteRules)
  }
</script>

<div class="bg-base-200 rounded-lg p-3">
  <div class="flex items-center justify-between mb-2">
    <h3 class="text-sm font-semibold flex items-center gap-1">
      {#if info?.bmsId}
        <a href="https://bmssearch.net/bmses/{info.bmsId}" target="_blank" rel="noopener noreferrer" class="link link-primary">BMS Search情報</a>
      {:else}
        BMS Search情報
      {/if}
      {#if info?.source === 'unofficial'}
        <!-- テキスト検索による自動推定紐付けの場合に警告アイコンを表示 -->
        <span class="tooltip tooltip-right" data-tip="テキスト検索により自動推定された紐付けです">
          <Icon name="search" cls="h-3.5 w-3.5 text-warning" />
        </span>
      {/if}
    </h3>
    <div class="flex items-center gap-1">
      <button class="btn btn-ghost btn-xs" disabled={loading} on:click={() => dispatch('lookup')}>
        {#if loading}
          <span class="loading loading-spinner loading-xs"></span>
        {:else}
          取得
        {/if}
      </button>
      {#if hasInfo}
        <button class="btn btn-ghost btn-xs" on:click={() => dispatch('unlink')}>解除</button>
      {/if}
    </div>
  </div>
  {#if hasInfo && info}
    <div class="text-xs space-y-1">
      <p>
      {#if info.title}
        <span class="font-semibold">タイトル:</span> {info.title} /
      {/if}
      {#if info.artist}
        <span class="font-semibold">アーティスト:</span> {info.artist} /
      {/if}
      {#if info.subArtist}
        <span class="font-semibold">サブアーティスト:</span> {info.subArtist} /
      {/if}
      {#if info.genre}
        <span class="font-semibold">ジャンル:</span> {info.genre} /
      {/if}
      {#if info.publishedAt}
        <span class="font-semibold">公開日:</span> {formatDateYMD(info.publishedAt)}
      {/if}
      </p>
      <p>
      {#if info.exhibitionName}
          <span class="font-semibold">イベント:</span>
          {#if info.exhibitionId}
            <a href="https://bmssearch.net/exhibitions/{info.exhibitionId}" target="_blank" rel="noopener noreferrer" class="link link-primary">{info.exhibitionName}</a>
          {:else}
            {info.exhibitionName}
          {/if}
      {/if}
      </p>
      {#if info.downloads?.length}
        <div>
          <span class="font-semibold">DLリンク:</span>
          <ul class="ml-4 list-disc">
            {#each info.downloads as d}
              <li>
                <a href={rewrite(d.url)} target="_blank" rel="noopener noreferrer" class="link link-primary">{rewrite(d.url)}</a>
                {#if d.description}<span class="text-base-content/60">— {d.description}</span>{/if}
              </li>
            {/each}
          </ul>
        </div>
      {/if}
      {#if info.previews?.length}
        <div>
          <span class="font-semibold">プレビュー:</span>
            {#each info.previews as p, i}
              <a href={previewUrl(p)} target="_blank" rel="noopener noreferrer" class="link link-primary">{p.service}</a>
              {#if i !== info.previews.length - 1} /&nbsp;{/if}
            {/each}
        </div>
      {/if}
      {#if info.relatedLinks?.length}
        <div>
          <span class="font-semibold">関連リンク:</span>
          <ul class="ml-4 list-disc">
            {#each info.relatedLinks as r}
              <li>
                <a href={rewrite(r.url)} target="_blank" rel="noopener noreferrer" class="link link-primary">{rewrite(r.url)}</a>
                {#if r.description}<span class="text-base-content/60">— {r.description}</span>{/if}
              </li>
            {/each}
          </ul>
        </div>
      {/if}
    </div>
  {:else}
    <p class="text-xs text-base-content/50">BMS Search情報がありません。「取得」ボタンで取得してください。</p>
  {/if}
</div>
