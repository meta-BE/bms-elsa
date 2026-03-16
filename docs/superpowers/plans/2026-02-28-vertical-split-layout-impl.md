# 上下分割レイアウト 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 詳細パネルを右側から画面下部に移動し、ドラッグでリサイズ可能な上下分割レイアウトにする

**Architecture:** App.svelte のレイアウトを flex-row から flex-col に変更。ドラッグリサイズは mousedown/mousemove/mouseup で splitRatio を制御。SongTable に deselect イベント、SongDetail に close イベントを追加。

**Tech Stack:** Svelte 4, TailwindCSS, DaisyUI 5

**設計ドキュメント:** `docs/plans/2026-02-28-vertical-split-layout-design.md`

---

## Task 1: SongDetail に閉じるボタンを追加

**Files:**
- Modify: `frontend/src/SongDetail.svelte`

**Step 1: createEventDispatcher を追加し close イベントを定義**

`SongDetail.svelte` の `<script>` セクション先頭（import 群の後）に以下を追加:

```typescript
import { createEventDispatcher } from 'svelte'

const dispatch = createEventDispatcher<{ close: void }>()
```

**Step 2: 楽曲ヘッダーに閉じるボタンを追加**

既存の楽曲ヘッダー部分（`<div class="bg-base-200 rounded-lg p-3">` 内、`<h2>` の直前）に閉じるボタンを追加:

```svelte
    <div class="bg-base-200 rounded-lg p-3">
      <div class="flex justify-between items-start">
        <div class="flex-1 min-w-0">
          <h2 class="text-lg font-bold truncate">{detail.title}</h2>
          <p class="text-sm text-base-content/70">{detail.artist}</p>
          <p class="text-xs text-base-content/50">{detail.genre}</p>
        </div>
        <button
          class="btn btn-ghost btn-xs shrink-0 ml-2"
          on:click={() => dispatch('close')}
        >✕</button>
      </div>
      <div class="divider my-1"></div>
```

つまり、既存の `<h2>`, `<p>`, `<p>` を `<div class="flex justify-between items-start">` で囲み、閉じるボタンを右に配置する。

**Step 3: ビルド確認**

```bash
cd /path/to/bms-elsa/frontend
npm run check
npm run build
```

Expected: エラーなし

**Step 4: コミット**

```bash
git add frontend/src/SongDetail.svelte
git commit -m "feat: 詳細パネルに閉じるボタンを追加"
```

---

## Task 2: SongTable に空き部分クリックで deselect イベントを追加

**Files:**
- Modify: `frontend/src/SongTable.svelte`

**Step 1: EventDispatcher に deselect イベントを追加**

既存の `dispatch` 定義を変更:

```typescript
// 変更前
const dispatch = createEventDispatcher<{ select: string }>()

// 変更後
const dispatch = createEventDispatcher<{ select: string; deselect: void }>()
```

**Step 2: 仮想スクロール領域のコンテナに click ハンドラーを追加**

`bind:this={scrollElement}` のある div（140行目付近）に `on:click` を追加:

```svelte
  <div
    bind:this={scrollElement}
    class="flex-1 overflow-auto"
    on:click={() => dispatch('deselect')}
  >
```

**Step 3: 行クリックに stopPropagation を追加**

行の `on:click`（157行目）を `on:click|stopPropagation` に変更:

```svelte
// 変更前
on:click={() => dispatch('select', row.original.folderHash)}

// 変更後
on:click|stopPropagation={() => dispatch('select', row.original.folderHash)}
```

同様に `on:keydown`（158行目）にも `|stopPropagation` を追加:

```svelte
// 変更前
on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') dispatch('select', row.original.folderHash) }}

// 変更後
on:keydown|stopPropagation={(e) => { if (e.key === 'Enter' || e.key === ' ') dispatch('select', row.original.folderHash) }}
```

**Step 4: ビルド確認**

```bash
cd /path/to/bms-elsa/frontend
npm run check
npm run build
```

Expected: エラーなし

**Step 5: コミット**

```bash
git add frontend/src/SongTable.svelte
git commit -m "feat: テーブル空き部分クリックでdeselectイベントを発火"
```

---

## Task 3: App.svelte を上下分割レイアウトに変更

**Files:**
- Modify: `frontend/src/App.svelte`

**Step 1: App.svelte を以下の内容に置き換え**

```svelte
<script lang="ts">
  import SongTable from './SongTable.svelte'
  import SongDetail from './SongDetail.svelte'

  let selectedFolderHash: string | null = null
  let containerEl: HTMLDivElement
  let dragging = false
  let splitRatio = 0.6

  function handleSelect(e: CustomEvent<string>) {
    selectedFolderHash = e.detail
  }

  function handleDeselect() {
    selectedFolderHash = null
  }

  function handleClose() {
    selectedFolderHash = null
  }

  function onDragStart(e: MouseEvent) {
    e.preventDefault()
    dragging = true
    window.addEventListener('mousemove', onDragMove)
    window.addEventListener('mouseup', onDragEnd)
  }

  function onDragMove(e: MouseEvent) {
    if (!dragging || !containerEl) return
    const rect = containerEl.getBoundingClientRect()
    splitRatio = Math.max(0.2, Math.min(0.8, (e.clientY - rect.top) / rect.height))
  }

  function onDragEnd() {
    dragging = false
    window.removeEventListener('mousemove', onDragMove)
    window.removeEventListener('mouseup', onDragEnd)
  }
</script>

<div data-theme="emerald" class="h-full flex flex-col">
  <div class="navbar bg-base-200 px-4 shrink-0">
    <div class="flex-1">
      <span class="text-xl font-bold">BMS ELSA</span>
    </div>
  </div>

  <div bind:this={containerEl} class="flex-1 overflow-hidden p-4 flex flex-col">
    <div class="overflow-hidden" style="flex: {selectedFolderHash ? splitRatio : 1}">
      <SongTable on:select={handleSelect} on:deselect={handleDeselect} />
    </div>
    {#if selectedFolderHash}
      <div
        class="h-1 shrink-0 cursor-row-resize bg-base-300 hover:bg-primary/30 transition-colors my-1 rounded"
        on:mousedown={onDragStart}
        role="separator"
      ></div>
      <div class="overflow-y-auto" style="flex: {1 - splitRatio}">
        <SongDetail folderHash={selectedFolderHash} on:close={handleClose} />
      </div>
    {/if}
  </div>
</div>

<style>
  :global(body) {
    margin: 0;
  }
</style>
```

変更のポイント:
- レイアウトを `flex gap-4`（横並び）から `flex flex-col`（縦並び）に変更
- テーブル領域: `style="flex: {splitRatio}"` で比率制御。パネル未選択時は `flex: 1` で全高使用
- ドラッグハンドル: `h-1 cursor-row-resize bg-base-300 hover:bg-primary/30`
- 詳細パネル: `style="flex: {1 - splitRatio}"` で残り領域を使用
- `on:deselect` と `on:close` ハンドラーで `selectedFolderHash = null`
- ドラッグ中の splitRatio は 0.2〜0.8 にクランプ

**Step 2: ビルド確認**

```bash
cd /path/to/bms-elsa/frontend
npm run check
npm run build
```

Expected: エラーなし

**Step 3: wails dev で動作確認**

```bash
cd /path/to/bms-elsa
wails dev
```

確認項目:
- テーブルが全幅を使用している
- 行クリックで下部に詳細パネルが表示される
- ドラッグハンドルでリサイズできる
- 詳細パネルの×ボタンで閉じる
- テーブルの空き部分（行以外）クリックで閉じる
- 別の行クリックで詳細が切り替わる
- カラムヘッダークリックでソートが動作する

**Step 4: コミット**

```bash
git add frontend/src/App.svelte
git commit -m "feat: 上下分割レイアウトとドラッグリサイズを実装"
```

---

## タスク依存関係

```
Task 1 (SongDetail閉じるボタン) ─┐
Task 2 (SongTable deselect)     ─┴── Task 3 (App.svelte上下分割)
```

Task 1 と Task 2 は独立して実装可能。Task 3 は Task 1, 2 の完了後に実施。
