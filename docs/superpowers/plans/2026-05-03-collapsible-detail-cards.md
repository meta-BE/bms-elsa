# 詳細画面カード最小化機能 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 詳細画面（楽曲・譜面・難易度表エントリ）の各カード UI を「ペイン × カード種別」単位で最小化可能にし、`config.json` に状態を永続化する。

**Architecture:** 共通ラッパー `CollapsibleCard.svelte` を新設し、既存 4 種カードコンポーネント（`ChartInfoCard`/`IRInfoCard`/`BMSSearchInfoCard`/`InstallCandidateCard`）の外枠とタイトル行を `CollapsibleCard` に移管。状態は Svelte writable store で持ち、Go 側 `Config.CardCollapsed` フィールドへ即時保存（既存 `columnWidths` と同じパターン）。

**Tech Stack:** Go 1.24 + Wails v2 / Svelte 4 + TypeScript / TailwindCSS + DaisyUI 5 / Heroicons

**Spec:** `docs/superpowers/specs/2026-05-03-collapsible-detail-cards-design.md`

---

## 前提と検証方法

このプロジェクトにはフロントエンド単体テスト基盤がなく、検証は以下で行う:

- **型チェック**: `cd frontend && npm run check`（`svelte-check` + `tsc`）
- **Go ビルド**: `go build ./...`（プロジェクトルートで実行。`go build .` はルートにバイナリを出力するため不可）
- **動作確認**: `wails dev` で起動して手動確認（最終タスク）

各タスクの完了基準は「型チェックが通る + `go build ./...` が通る」を最低条件とする。

各 commit のメッセージ末尾に必ず以下を付けること:

```
Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
```

---

## ファイル構造（変更一覧）

| ファイル | 種別 | 担当範囲 |
|---|---|---|
| `app.go` | 変更 | `Config` 構造体に `CardCollapsed` フィールド追加 |
| `frontend/wailsjs/go/models.ts` | 自動生成 | `wails dev` 起動時に再生成。手で編集しない |
| `frontend/src/components/icons.ts` | 変更 | `chevronRight` / `chevronDown` を追加 |
| `frontend/src/stores/cardCollapsed.ts` | 新規 | 開閉状態の writable store + 初期化 / トグル関数 |
| `frontend/src/components/CollapsibleCard.svelte` | 新規 | 共通ラッパー（外枠 + タイトル + 最小化トグル + slots） |
| `frontend/src/components/ChartInfoCard.svelte` | 変更 | `CollapsibleCard` ラップ + `paneId` props 追加 |
| `frontend/src/components/IRInfoCard.svelte` | 変更 | 同上 + `actions` slot に「IR取得」 |
| `frontend/src/components/BMSSearchInfoCard.svelte` | 変更 | 同上 + `actions` slot に「取得 / 解除」 |
| `frontend/src/components/InstallCandidateCard.svelte` | 変更 | 同上 |
| `frontend/src/views/SongDetail.svelte` | 変更 | 各カードに `paneId="song"` を渡す |
| `frontend/src/views/ChartDetail.svelte` | 変更 | 各カードに `paneId="chart"` を渡す |
| `frontend/src/views/EntryDetail.svelte` | 変更 | 各カードに `paneId="entry"` を渡す |
| `frontend/src/App.svelte` | 変更 | `onMount` で `initCardCollapsed()` を呼び出し |
| `docs/TODO.md` | 変更 | 該当項目をチェック済みに |

---

## Task 1: Go `Config` 構造体に `CardCollapsed` フィールドを追加

**Files:**
- Modify: `app.go:226-231`

- [ ] **Step 1: `Config` 構造体にフィールドを追加**

`app.go` の `Config` 構造体を以下に変更する（`app.go:226` 付近）:

```go
// Config はアプリケーション設定
type Config struct {
	SongdataDBPath string                        `json:"songdataDBPath"`
	FileLog        bool                          `json:"fileLog"`
	ColumnWidths   map[string]map[string]float64 `json:"columnWidths,omitempty"`
	CardCollapsed  map[string]map[string]bool    `json:"cardCollapsed,omitempty"`
}
```

- [ ] **Step 2: ビルド確認**

```bash
go build ./...
```

期待結果: エラーなく完了。

- [ ] **Step 3: コミット**

```bash
git add app.go
git commit -m "$(cat <<'EOF'
feat: Config に CardCollapsed フィールドを追加

詳細画面の各カード UI の最小化状態をペイン×カード種別単位で
config.json に永続化するためのフィールド。omitempty 付きのため
既存設定との後方互換性は問題なし。

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: chevron アイコンを追加

**Files:**
- Modify: `frontend/src/components/icons.ts`

- [ ] **Step 1: アイコン定義を追加**

`frontend/src/components/icons.ts` の `icons` オブジェクトの `search` の下、最後の `}`（`} as const`）の前に以下を追加する。スタイルは既存の 24/outline / strokeWidth 1.5 系統に揃える（heroicons v2 24/outline/chevron-right, chevron-down）:

```ts
  // heroicons v2: 24/outline/chevron-right
  chevronRight: {
    viewBox: '0 0 24 24',
    type: 'stroke',
    strokeWidth: 1.5,
    paths: [{ d: 'M8.25 4.5l7.5 7.5-7.5 7.5' }],
  },
  // heroicons v2: 24/outline/chevron-down
  chevronDown: {
    viewBox: '0 0 24 24',
    type: 'stroke',
    strokeWidth: 1.5,
    paths: [{ d: 'm19.5 8.25-7.5 7.5-7.5-7.5' }],
  },
```

- [ ] **Step 2: 型チェック**

```bash
cd frontend && npm run check
```

期待結果: 既存と同じ警告のみ、新規エラーなし。

- [ ] **Step 3: コミット**

```bash
git add frontend/src/components/icons.ts
git commit -m "$(cat <<'EOF'
feat: chevronRight / chevronDown アイコンを追加

カード最小化トグルボタンで使用。Heroicons v2 24/outline、
strokeWidth 1.5 で既存アイコン群と統一。

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: `cardCollapsed` ストアを作成

**Files:**
- Create: `frontend/src/stores/cardCollapsed.ts`

- [ ] **Step 1: ストアファイルを新規作成**

`frontend/src/stores/cardCollapsed.ts` を新規作成する:

```ts
import { writable, get } from 'svelte/store'
import { GetConfig, SaveConfig } from '../../wailsjs/go/main/App'

export type PaneId = 'song' | 'chart' | 'entry'
export type CardId = 'chartInfo' | 'irInfo' | 'bmsSearch' | 'installCandidate'
export type CollapsedMap = Partial<Record<PaneId, Partial<Record<CardId, boolean>>>>

export const cardCollapsed = writable<CollapsedMap>({})

let initialized = false

// 起動時に config.json から読み込み、ストアへ反映する
export async function initCardCollapsed(): Promise<void> {
  if (initialized) return
  initialized = true
  const cfg = await GetConfig()
  cardCollapsed.set((cfg.cardCollapsed as CollapsedMap | undefined) ?? {})
}

// paneId × cardId の最小化状態をトグルし、config.json へ即時保存する。
// 値が false 相当（=展開）になるケースはキー自体を削除して JSON を肥大化させない。
export async function toggleCard(paneId: PaneId, cardId: CardId): Promise<void> {
  cardCollapsed.update(curr => {
    const pane = { ...(curr[paneId] ?? {}) }
    if (pane[cardId]) {
      delete pane[cardId]
    } else {
      pane[cardId] = true
    }
    const updated: CollapsedMap = { ...curr }
    if (Object.keys(pane).length === 0) {
      delete updated[paneId]
    } else {
      updated[paneId] = pane
    }
    return updated
  })
  const cfg = await GetConfig()
  await SaveConfig({ ...cfg, cardCollapsed: get(cardCollapsed) } as any)
}
```

メモ:
- `SaveConfig({ ... } as any)` の `as any` は、Wails 自動生成の `Config` 型がまだ新フィールドを認識していない可能性に対する一時的回避。`wails dev` 等で `models.ts` が再生成されれば不要。Task 9 終了後に手動で外せるか確認する。

- [ ] **Step 2: 型チェック**

```bash
cd frontend && npm run check
```

期待結果: エラーなし（`as any` で型エラー回避済みのため）。

- [ ] **Step 3: コミット**

```bash
git add frontend/src/stores/cardCollapsed.ts
git commit -m "$(cat <<'EOF'
feat: cardCollapsed ストアを追加

詳細画面カードの開閉状態を保持する Svelte writable store。
toggleCard 呼び出し時に config.json へ即時保存し、false 相当
の値はキーごと削除する。

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: `CollapsibleCard` コンポーネントを作成

**Files:**
- Create: `frontend/src/components/CollapsibleCard.svelte`

- [ ] **Step 1: コンポーネントを新規作成**

`frontend/src/components/CollapsibleCard.svelte` を新規作成する:

```svelte
<script lang="ts">
  import { cardCollapsed, toggleCard, type PaneId, type CardId } from '../stores/cardCollapsed'
  import Icon from './Icon.svelte'

  export let paneId: PaneId
  export let cardId: CardId

  $: collapsed = $cardCollapsed[paneId]?.[cardId] === true
</script>

<div class="bg-base-200 rounded-lg p-3">
  <div class="flex items-center justify-between" class:mb-2={!collapsed}>
    <div class="flex items-center gap-1 min-w-0">
      <button
        class="btn btn-ghost btn-xs btn-square"
        on:click={() => toggleCard(paneId, cardId)}
        title={collapsed ? '展開' : '最小化'}
      >
        <Icon name={collapsed ? 'chevronRight' : 'chevronDown'} cls="h-4 w-4" />
      </button>
      <h3 class="text-sm font-semibold truncate">
        <slot name="title" />
      </h3>
    </div>
    {#if !collapsed}
      <slot name="actions" />
    {/if}
  </div>
  {#if !collapsed}
    <slot />
  {/if}
</div>
```

ポイント:
- 外枠 `bg-base-200 rounded-lg p-3` を本コンポーネントに集約
- 最小化時は `actions` slot とデフォルト slot を描画しない、`mb-2` も外す
- `title` slot は単純テキスト以外（`<a>` タグ、警告アイコン等）も入る前提

- [ ] **Step 2: 型チェック**

```bash
cd frontend && npm run check
```

期待結果: エラーなし。

- [ ] **Step 3: コミット**

```bash
git add frontend/src/components/CollapsibleCard.svelte
git commit -m "$(cat <<'EOF'
feat: CollapsibleCard コンポーネントを追加

詳細画面カードの共通ラッパー。外枠 / タイトル / 最小化トグル /
title・actions・default の3スロットを提供する。状態は
cardCollapsed ストアで paneId × cardId 単位に管理。

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: `ChartInfoCard` を `CollapsibleCard` でラップ

**Files:**
- Modify: `frontend/src/components/ChartInfoCard.svelte`
- Modify: `frontend/src/views/SongDetail.svelte`（呼び出し1箇所）
- Modify: `frontend/src/views/ChartDetail.svelte`（呼び出し1箇所）
- Modify: `frontend/src/views/EntryDetail.svelte`（呼び出し1箇所）

- [ ] **Step 1: `ChartInfoCard.svelte` を書き換え**

`frontend/src/components/ChartInfoCard.svelte` の全内容を以下で置き換える:

```svelte
<script lang="ts">
  import type { dto } from '../../wailsjs/go/models'
  import type { PaneId } from '../stores/cardCollapsed'
  import { modeLabel, diffLabel } from '../utils/chartLabels'
  import CollapsibleCard from './CollapsibleCard.svelte'

  export let chart: dto.ChartDTO
  export let paneId: PaneId
</script>

<CollapsibleCard {paneId} cardId="chartInfo">
  <span slot="title">譜面情報</span>
  <div class="text-xs space-y-1">
    <div class="flex items-center gap-4">
      <span><span class="font-semibold">Mode:</span> {modeLabel(chart.mode)}</span>
      <span><span class="font-semibold">Difficulty:</span> {diffLabel(chart.difficulty)}</span>
      <span><span class="font-semibold">Level:</span> ☆{chart.level}</span>
      <span><span class="font-semibold">Notes:</span> {chart.notes?.toLocaleString() ?? '-'}</span>
    </div>
    <p>
      <span class="font-semibold">BPM:</span>
      {#if chart.minBpm === chart.maxBpm}
        {Math.round(chart.minBpm)}
      {:else}
        {Math.round(chart.minBpm)}-{Math.round(chart.maxBpm)}
      {/if}
    </p>
    {#if chart.difficultyLabels?.length}
      <div class="flex items-center gap-1 flex-wrap">
        <span class="font-semibold">難易度表:</span>
        {#each chart.difficultyLabels as label}
          <span class="badge badge-sm badge-outline" title={label.tableName}>{label.symbol}{label.level}</span>
        {/each}
      </div>
    {/if}
    {#if chart.path}
      <p class="truncate">
        <span class="font-semibold">パス:</span>
        <span class="text-base-content/50">{chart.path}</span>
      </p>
    {/if}
  </div>
</CollapsibleCard>
```

- [ ] **Step 2: `SongDetail.svelte` の呼び出しを更新**

`frontend/src/views/SongDetail.svelte:327` の以下の行:

```svelte
<ChartInfoCard chart={selectedChart} />
```

を以下に置き換える:

```svelte
<ChartInfoCard chart={selectedChart} paneId="song" />
```

- [ ] **Step 3: `ChartDetail.svelte` の呼び出しを更新**

`frontend/src/views/ChartDetail.svelte:99` の以下の行:

```svelte
<ChartInfoCard {chart} />
```

を以下に置き換える:

```svelte
<ChartInfoCard {chart} paneId="chart" />
```

- [ ] **Step 4: `EntryDetail.svelte` の呼び出しを更新**

`frontend/src/views/EntryDetail.svelte:135` の以下の行:

```svelte
<ChartInfoCard {chart} />
```

を以下に置き換える:

```svelte
<ChartInfoCard {chart} paneId="entry" />
```

- [ ] **Step 5: 型チェック**

```bash
cd frontend && npm run check
```

期待結果: 新規エラーなし。`paneId` を渡し忘れた場合は型エラーになる。

- [ ] **Step 6: コミット**

```bash
git add frontend/src/components/ChartInfoCard.svelte \
        frontend/src/views/SongDetail.svelte \
        frontend/src/views/ChartDetail.svelte \
        frontend/src/views/EntryDetail.svelte
git commit -m "$(cat <<'EOF'
feat: ChartInfoCard を CollapsibleCard でラップ

外枠とタイトル行を CollapsibleCard に移管。各 Detail ビュー
からは paneId を渡すようにした。

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: `IRInfoCard` を `CollapsibleCard` でラップ

**Files:**
- Modify: `frontend/src/components/IRInfoCard.svelte`
- Modify: `frontend/src/views/SongDetail.svelte`
- Modify: `frontend/src/views/ChartDetail.svelte`
- Modify: `frontend/src/views/EntryDetail.svelte`

- [ ] **Step 1: `IRInfoCard.svelte` を書き換え**

`frontend/src/components/IRInfoCard.svelte` の全内容を以下で置き換える:

```svelte
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
```

- [ ] **Step 2: `SongDetail.svelte` の呼び出しを更新**

`frontend/src/views/SongDetail.svelte:328` の以下の行:

```svelte
<IRInfoCard md5={selectedChart.md5} ir={selectedChart} on:lookup={() => selectedChart && lookupIR(selectedChart)} />
```

を以下に置き換える:

```svelte
<IRInfoCard md5={selectedChart.md5} ir={selectedChart} paneId="song" on:lookup={() => selectedChart && lookupIR(selectedChart)} />
```

- [ ] **Step 3: `ChartDetail.svelte` の呼び出しを更新**

`frontend/src/views/ChartDetail.svelte:106` の以下の行:

```svelte
<IRInfoCard md5={chart.md5} ir={chart} on:lookup={lookupIR} />
```

を以下に置き換える:

```svelte
<IRInfoCard md5={chart.md5} ir={chart} paneId="chart" on:lookup={lookupIR} />
```

- [ ] **Step 4: `EntryDetail.svelte` の呼び出しを更新**

`frontend/src/views/EntryDetail.svelte:144` の以下の行:

```svelte
<IRInfoCard {md5} {ir} on:lookup={lookupIR} />
```

を以下に置き換える:

```svelte
<IRInfoCard {md5} {ir} paneId="entry" on:lookup={lookupIR} />
```

- [ ] **Step 5: 型チェック**

```bash
cd frontend && npm run check
```

期待結果: 新規エラーなし。

- [ ] **Step 6: コミット**

```bash
git add frontend/src/components/IRInfoCard.svelte \
        frontend/src/views/SongDetail.svelte \
        frontend/src/views/ChartDetail.svelte \
        frontend/src/views/EntryDetail.svelte
git commit -m "$(cat <<'EOF'
feat: IRInfoCard を CollapsibleCard でラップ

タイトル（LR2IR情報リンク）は title slot、IR取得ボタンは
actions slot に配置。各 Detail ビューからは paneId を渡す。

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: `BMSSearchInfoCard` を `CollapsibleCard` でラップ

**Files:**
- Modify: `frontend/src/components/BMSSearchInfoCard.svelte`
- Modify: `frontend/src/views/SongDetail.svelte`
- Modify: `frontend/src/views/ChartDetail.svelte`
- Modify: `frontend/src/views/EntryDetail.svelte`

- [ ] **Step 1: `BMSSearchInfoCard.svelte` を書き換え**

`frontend/src/components/BMSSearchInfoCard.svelte` の全内容を以下で置き換える:

```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import type { dto } from '../../wailsjs/go/models'
  import { rewriteRules } from '../stores/rewriteRules'
  import { applyRewriteRules } from '../lib/urlRewrite'
  import { formatDateYMD } from '../utils/date'
  import type { PaneId } from '../stores/cardCollapsed'
  import Icon from './Icon.svelte'
  import CollapsibleCard from './CollapsibleCard.svelte'

  export let info: dto.BMSSearchInfoDTO | null = null
  export let loading = false
  export let paneId: PaneId

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

<CollapsibleCard {paneId} cardId="bmsSearch">
  <span slot="title" class="flex items-center gap-1">
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
  </span>
  <div slot="actions" class="flex items-center gap-1">
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
</CollapsibleCard>
```

- [ ] **Step 2: `SongDetail.svelte` の呼び出しを更新**

`frontend/src/views/SongDetail.svelte:260-265` の `<BMSSearchInfoCard>` 要素に `paneId="song"` を追加する。差分:

```svelte
<BMSSearchInfoCard
  info={bmsSearchInfo}
  loading={bmsSearchLoading}
  paneId="song"
  on:lookup={lookupBMSSearch}
  on:unlink={unlinkBMSSearch}
/>
```

- [ ] **Step 3: `ChartDetail.svelte` の呼び出しを更新**

`frontend/src/views/ChartDetail.svelte:100-105` の `<BMSSearchInfoCard>` 要素に `paneId="chart"` を追加する:

```svelte
<BMSSearchInfoCard
  info={bmsSearchInfo}
  loading={bmsSearchLoading}
  paneId="chart"
  on:lookup={lookupBMSSearch}
  on:unlink={unlinkBMSSearch}
/>
```

- [ ] **Step 4: `EntryDetail.svelte` の呼び出しを更新**

`frontend/src/views/EntryDetail.svelte:145-150` の `<BMSSearchInfoCard>` 要素に `paneId="entry"` を追加する:

```svelte
<BMSSearchInfoCard
  info={bmsSearchInfo}
  loading={bmsSearchLoading}
  paneId="entry"
  on:lookup={lookupBMSSearch}
  on:unlink={unlinkBMSSearch}
/>
```

- [ ] **Step 5: 型チェック**

```bash
cd frontend && npm run check
```

期待結果: 新規エラーなし。

- [ ] **Step 6: コミット**

```bash
git add frontend/src/components/BMSSearchInfoCard.svelte \
        frontend/src/views/SongDetail.svelte \
        frontend/src/views/ChartDetail.svelte \
        frontend/src/views/EntryDetail.svelte
git commit -m "$(cat <<'EOF'
feat: BMSSearchInfoCard を CollapsibleCard でラップ

タイトル（リンク + 警告アイコン）は title slot、取得 / 解除
ボタンは actions slot に配置。各 Detail ビューからは paneId
を渡す。

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: `InstallCandidateCard` を `CollapsibleCard` でラップ

**Files:**
- Modify: `frontend/src/components/InstallCandidateCard.svelte`
- Modify: `frontend/src/views/EntryDetail.svelte`

- [ ] **Step 1: `InstallCandidateCard.svelte` を書き換え**

`frontend/src/components/InstallCandidateCard.svelte` の全内容を以下で置き換える:

```svelte
<script lang="ts">
  import { EstimateInstallLocation } from '../../wailsjs/go/app/DifficultyTableHandler'
  import type { PaneId } from '../stores/cardCollapsed'
  import OpenFolderButton from './OpenFolderButton.svelte'
  import CollapsibleCard from './CollapsibleCard.svelte'

  export let md5: string
  export let tableID: number
  export let paneId: PaneId

  type Candidate = {
    folderPath: string
    title: string
    artist: string
    matchTypes: string[]
    score: number
  }

  let candidates: Candidate[] = []
  let loading = false

  $: if (md5 && tableID) load(md5, tableID)

  async function load(hash: string, tid: number) {
    loading = true
    candidates = []
    try {
      candidates = (await EstimateInstallLocation(hash, tid)) || []
    } catch (e) {
      console.error('Failed to estimate install location:', e)
    } finally {
      loading = false
    }
  }

  function matchLabel(mt: string): string {
    switch (mt) {
      case 'title': return 'タイトル一致'
      case 'base_title': return 'タイトル類似'
      case 'body_url': return 'URL一致'
      case 'artist': return 'アーティスト一致'
      default: return mt
    }
  }
</script>

<CollapsibleCard {paneId} cardId="installCandidate">
  <span slot="title">導入先の推定</span>
  {#if loading}
    <div class="flex justify-center py-2">
      <span class="loading loading-spinner loading-sm"></span>
    </div>
  {:else if candidates.length === 0}
    <p class="text-sm text-base-content/50">一致する導入済み楽曲が見つかりませんでした</p>
  {:else}
    <div class="space-y-2">
      {#each candidates as c}
        <div class="flex items-center justify-between gap-2">
          <div class="min-w-0 flex-1">
            <p class="text-sm truncate">{c.title} / {c.artist}</p>
            <p class="text-xs text-base-content/50 truncate">{c.folderPath}</p>
            <div class="flex gap-1 mt-0.5">
              {#each c.matchTypes as mt}
                <span class="badge badge-xs">{matchLabel(mt)}</span>
              {/each}
            </div>
          </div>
          <OpenFolderButton path={c.folderPath} />
        </div>
      {/each}
    </div>
  {/if}
</CollapsibleCard>
```

- [ ] **Step 2: `EntryDetail.svelte` の呼び出しを更新**

`frontend/src/views/EntryDetail.svelte:140` の以下の行:

```svelte
<InstallCandidateCard {md5} {tableID} />
```

を以下に置き換える:

```svelte
<InstallCandidateCard {md5} {tableID} paneId="entry" />
```

- [ ] **Step 3: 型チェック**

```bash
cd frontend && npm run check
```

期待結果: 新規エラーなし。

- [ ] **Step 4: コミット**

```bash
git add frontend/src/components/InstallCandidateCard.svelte \
        frontend/src/views/EntryDetail.svelte
git commit -m "$(cat <<'EOF'
feat: InstallCandidateCard を CollapsibleCard でラップ

EntryDetail からのみ使われるため paneId="entry" を渡す。

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: `App.svelte` で `initCardCollapsed()` を起動時に呼び出し

**Files:**
- Modify: `frontend/src/App.svelte`
- Modify (任意): `frontend/src/stores/cardCollapsed.ts`（`as any` 削除を試みる）

- [ ] **Step 1: `App.svelte` の import 追加**

`frontend/src/App.svelte:18` の以下の行:

```ts
import { rewriteRules } from './stores/rewriteRules'
```

の直後に以下を追加する:

```ts
import { initCardCollapsed } from './stores/cardCollapsed'
```

- [ ] **Step 2: `App.svelte` の `onMount` に呼び出しを追加**

`frontend/src/App.svelte:139-142` の以下の箇所:

```ts
onMount(() => {
    ListRewriteRules().then(rules => {
      rewriteRules.set(rules ?? [])
    })
```

の `ListRewriteRules` 呼び出しの直後（同じ `onMount` ブロック内）に以下を追加する:

```ts
    initCardCollapsed()
```

完成イメージ:

```ts
onMount(() => {
    ListRewriteRules().then(rules => {
      rewriteRules.set(rules ?? [])
    })

    initCardCollapsed()

    document.addEventListener('click', (e) => {
      // ... 既存処理
```

`initCardCollapsed()` は fire-and-forget でよい（理由: ストアの初期値 `{}` でも全展開動作になり、初期化遅延中に一瞬展開状態で表示されても問題ないため。spec §6-a 参照）。

- [ ] **Step 3: Wails 自動生成型の更新確認 + `as any` 削除可否チェック**

`wails dev` を一度起動して止めると `frontend/wailsjs/go/models.ts` の `Config` 型に `cardCollapsed` が追加されているはず。確認:

```bash
grep -A 5 "export class Config" frontend/wailsjs/go/models.ts
```

期待出力例:

```ts
export class Config {
    songdataDBPath: string;
    fileLog: boolean;
    columnWidths?: Record<string, any>;
    cardCollapsed?: Record<string, any>;
```

`cardCollapsed?: Record<string, any>` が追加されていれば、`stores/cardCollapsed.ts` の `as any` を外せるか試す。`as any` を外して `npm run check` が通るならコミットに含める。型エラーが出る場合はそのまま残す（`columnWidths` も `Record<string, any>` 扱いのため、深いネスト型一致は期待できない可能性がある）。

判断:
- 通った場合: `stores/cardCollapsed.ts` の `as any` を削除して同じコミットに含める
- 通らなかった場合: `as any` を残し、コミットメッセージにそのことを明記

- [ ] **Step 4: 型チェック**

```bash
cd frontend && npm run check
```

期待結果: 新規エラーなし。

- [ ] **Step 5: コミット**

```bash
git add frontend/src/App.svelte frontend/src/stores/cardCollapsed.ts frontend/wailsjs/go/models.ts frontend/wailsjs/go/main/App.d.ts frontend/wailsjs/go/main/App.js 2>/dev/null
git commit -m "$(cat <<'EOF'
feat: 起動時に cardCollapsed ストアを初期化

App.svelte の onMount に initCardCollapsed を追加し、
config.json の cardCollapsed を読み込む。fire-and-forget で
よい（初期値 {} でも全展開動作になるため）。

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: 動作確認 + TODO.md 更新

**Files:**
- Modify: `docs/TODO.md`

- [ ] **Step 1: `wails dev` で起動して手動動作確認**

```bash
wails dev
```

以下のシナリオを順に確認する:

1. **基本動作（楽曲一覧タブ）**
   - 楽曲一覧から楽曲を選択 → 右側に SongDetail 表示
   - 譜面行をクリック → ChartInfoCard / IRInfoCard / BMSSearchInfoCard が出現
   - 各カードの左上 chevron ボタンをクリック → カード本体が消えてタイトル行のみになる
   - もう一度クリック → 展開される

2. **永続化**
   - SongDetail で `IRInfoCard` を最小化
   - アプリを完全終了して `wails dev` で再起動
   - 同じ楽曲を選択 → `IRInfoCard` が最小化されたまま復元されている

3. **ペイン独立性（ペイン × カード種別が独立）**
   - 楽曲一覧タブの SongDetail で `ChartInfoCard` を最小化
   - 譜面一覧タブに切り替え → 譜面選択 → ChartDetail を表示
   - ChartDetail の `ChartInfoCard` は **展開されている**（ペイン独立）
   - 同じく難易度表タブで EntryDetail の `ChartInfoCard` も **展開されている**

4. **EntryDetail 限定の InstallCandidateCard**
   - 難易度表タブの未導入エントリを選択 → InstallCandidateCard が出現
   - 最小化 → アプリ再起動 → 復元される

5. **`config.json` の確認**

   実行ファイル隣接の `config.json` を開いて `cardCollapsed` フィールドが追加されているか確認:
   ```bash
   # 開発時は wails dev のビルド出力ディレクトリの config.json
   find . -name config.json -not -path "*/node_modules/*" 2>/dev/null
   cat <そのパス>
   ```

   期待: `cardCollapsed: { "song": { "irInfo": true }, ... }` のような構造で、最小化していないキーは含まれない。

6. **不正値の許容性**
   - `config.json` を手動編集して `"cardCollapsed": { "song": { "unknownCard": true }, "unknownPane": { "chartInfo": true } }` のように未定義キーを混ぜる
   - 再起動 → アプリは普通に動作する（不正キーは無視される）

7. **DuplicateDetail への影響なし**
   - 重複検知タブで重複グループを選択 → DuplicateDetail が出現
   - 最小化機能は対象外なのでカードは従来通り（変更なし）

すべての項目が期待どおりであることを確認する。問題があれば該当 Task に戻って修正。

- [ ] **Step 2: `docs/TODO.md` の項目をチェック済みに更新**

`docs/TODO.md:95` の以下の行:

```markdown
- [ ] 詳細画面の各カードUIを最小化可能に（ペイン×カード種別ごとに開閉状態を保存、最小化ボタンは左上）
```

を以下に置き換える:

```markdown
- [x] 詳細画面の各カードUIを最小化可能に（ペイン×カード種別ごとに開閉状態を保存、最小化ボタンは左上）
```

- [ ] **Step 3: コミット**

```bash
git add docs/TODO.md
git commit -m "$(cat <<'EOF'
docs: TODO.md - 詳細画面カード最小化機能を完了済みに

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 4: 全体整合性の最終チェック**

```bash
cd frontend && npm run check
cd .. && go build ./...
```

両方ともエラーなく完了することを確認する。

---

## 完了条件（spec §9 と対応）

- [ ] 4 種のカード（`ChartInfoCard`/`IRInfoCard`/`BMSSearchInfoCard`/`InstallCandidateCard`）が `paneId × cardId` で独立に最小化・展開できる
- [ ] 開閉状態が `config.json` の `cardCollapsed` に永続化される
- [ ] アプリ再起動後も状態が復元される
- [ ] 最小化ボタンが各カードのタイトル左に表示される
- [ ] 未保存時はすべて展開状態で表示される
- [ ] `npm run check` / `go build ./...` ともエラーなし
