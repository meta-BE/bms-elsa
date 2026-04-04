# ProgressBar コンポーネント 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 進捗バーのUI表示を統一する再利用可能なProgressBarコンポーネントを作成し、5ファイル・7箇所の進捗表示を置き換える。

**Architecture:** clip-path 2色分割方式で、テキストを2層重ねてゲージ境界で色を分割する。`createEventDispatcher` + `cancelable` propで停止ボタンのオプション表示を制御。コンポーネントは `flex-1` で親の幅に従う。

**Tech Stack:** Svelte 4, Tailwind CSS 4, daisyUI 5

---

## ファイル構成

| 操作 | パス | 責務 |
|------|------|------|
| 新規作成 | `frontend/src/components/ProgressBar.svelte` | 進捗バーコンポーネント |
| 変更 | `frontend/src/settings/Settings.svelte:210-264` | 3タスクの進捗表示を差し替え |
| 変更 | `frontend/src/views/SongTable.svelte:228-232` | 同期進捗表示を差し替え |
| 変更 | `frontend/src/components/BulkFetchButton.svelte:64-68` | IR取得進捗表示を差し替え |
| 変更 | `frontend/src/views/DiffImportView.svelte:122-126` | 差分推定進捗表示を差し替え |
| 変更 | `frontend/src/settings/DifficultyTableSettings.svelte:191-193` | テーブル更新進捗表示を差し替え |

---

### Task 1: ProgressBar コンポーネントの作成

**Files:**
- Create: `frontend/src/components/ProgressBar.svelte`

- [ ] **Step 1: コンポーネントファイルを作成**

```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte'

  export let current: number
  export let total: number
  export let cancelable = false

  const dispatch = createEventDispatcher<{ cancel: void }>()

  $: percent = total > 0 ? Math.min((current / total) * 100, 100) : 0
  $: label = `${current.toLocaleString()} / ${total.toLocaleString()}`
</script>

<div class="flex items-center gap-2 flex-1">
  <div class="relative h-4 rounded-full bg-base-300 overflow-hidden flex-1">
    <div
      class="absolute inset-y-0 left-0 bg-primary rounded-full"
      style="width: {percent}%"
    ></div>
    <span class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-base-content/70 select-none">
      {label}
    </span>
    <span
      class="absolute inset-0 flex items-center justify-center text-[10px] font-semibold text-primary-content select-none"
      style="clip-path: inset(0 {100 - percent}% 0 0)"
    >
      {label}
    </span>
  </div>
  {#if cancelable}
    <button class="btn btn-xs btn-error btn-outline" on:click|stopPropagation={() => dispatch('cancel')}>停止</button>
  {/if}
</div>
```

- [ ] **Step 2: ビルドチェック**

Run: `cd frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | grep -E '(Error|ProgressBar)' | head -20`
Expected: ProgressBarに関するエラーなし

- [ ] **Step 3: コミット**

```bash
git add frontend/src/components/ProgressBar.svelte
git commit -m "feat: ProgressBarコンポーネントを新規作成"
```

---

### Task 2: Settings.svelte の3タスクを差し替え

**Files:**
- Modify: `frontend/src/settings/Settings.svelte:210-264`

- [ ] **Step 1: importを追加**

`Settings.svelte` の `<script>` ブロック内に以下を追加:

```typescript
import ProgressBar from '../components/ProgressBar.svelte'
```

- [ ] **Step 2: MinHashスキャンの進捗表示を差し替え**

変更前（210-215行付近）:
```svelte
        {#if scanState === 'running' && scanProgress.total > 0}
          <div class="flex items-center gap-2 text-xs">
            <progress class="progress progress-primary flex-1" value={scanProgress.current} max={scanProgress.total}></progress>
            <span class="text-base-content/50">{scanProgress.current.toLocaleString()}/{scanProgress.total.toLocaleString()}</span>
          </div>
        {/if}
```

変更後:
```svelte
        {#if scanState === 'running' && scanProgress.total > 0}
          <ProgressBar current={scanProgress.current} total={scanProgress.total} />
        {/if}
```

- [ ] **Step 3: 難易度表更新の進捗表示を差し替え**

変更前（233-238行付近）:
```svelte
        {#if dtState === 'running' && dtProgress.total > 0}
          <div class="flex items-center gap-2 text-xs">
            <progress class="progress progress-primary flex-1" value={dtProgress.current} max={dtProgress.total}></progress>
            <span class="text-base-content/50">{dtProgress.current}/{dtProgress.total}</span>
          </div>
        {/if}
```

変更後:
```svelte
        {#if dtState === 'running' && dtProgress.total > 0}
          <ProgressBar current={dtProgress.current} total={dtProgress.total} />
        {/if}
```

- [ ] **Step 4: 動作URL推定の進捗表示を差し替え**

変更前（256-261行付近）:
```svelte
        {#if rewriteState === 'running' && rewriteProgress.total > 0}
          <div class="flex items-center gap-2 text-xs">
            <progress class="progress progress-primary flex-1" value={rewriteProgress.current} max={rewriteProgress.total}></progress>
            <span class="text-base-content/50">{rewriteProgress.current.toLocaleString()}/{rewriteProgress.total.toLocaleString()}</span>
          </div>
        {/if}
```

変更後:
```svelte
        {#if rewriteState === 'running' && rewriteProgress.total > 0}
          <ProgressBar current={rewriteProgress.current} total={rewriteProgress.total} />
        {/if}
```

- [ ] **Step 5: ビルドチェック**

Run: `cd frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | grep -E '(Error|Settings)' | head -20`
Expected: エラーなし

- [ ] **Step 6: コミット**

```bash
git add frontend/src/settings/Settings.svelte
git commit -m "refactor: Settings.svelteの進捗表示をProgressBarに差し替え"
```

---

### Task 3: SongTable.svelte の同期進捗を差し替え

**Files:**
- Modify: `frontend/src/views/SongTable.svelte:228-232`

- [ ] **Step 1: importを追加**

`SongTable.svelte` の `<script>` ブロック内に以下を追加:

```typescript
import ProgressBar from '../components/ProgressBar.svelte'
```

- [ ] **Step 2: 同期進捗表示を差し替え**

変更前（228-232行付近）:
```svelte
      {#if syncing}
        <span class="text-xs text-base-content/70">
          同期中: {syncProgress.current.toLocaleString()} / {syncProgress.total.toLocaleString()}
        </span>
        <button class="btn btn-xs btn-error btn-outline" on:click|stopPropagation={stopSync}>停止</button>
```

変更後:
```svelte
      {#if syncing}
        <ProgressBar current={syncProgress.current} total={syncProgress.total} cancelable on:cancel={stopSync} />
```

注: `on:click|stopPropagation` は ProgressBar 内部で `on:click|stopPropagation` として実装済み。`on:cancel` ハンドラ側の `stopSync` 関数はそのまま。

- [ ] **Step 3: ビルドチェック**

Run: `cd frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | grep -E '(Error|SongTable)' | head -20`
Expected: エラーなし

- [ ] **Step 4: コミット**

```bash
git add frontend/src/views/SongTable.svelte
git commit -m "refactor: SongTableの同期進捗表示をProgressBarに差し替え"
```

---

### Task 4: BulkFetchButton.svelte のIR取得進捗を差し替え

**Files:**
- Modify: `frontend/src/components/BulkFetchButton.svelte:64-68`

- [ ] **Step 1: importを追加**

`BulkFetchButton.svelte` の `<script>` ブロック内に以下を追加:

```typescript
import ProgressBar from './ProgressBar.svelte'
```

注: 同じ `components/` ディレクトリ内なので相対パスは `./`。

- [ ] **Step 2: IR取得進捗表示を差し替え**

変更前（64-68行）:
```svelte
{#if fetching}
  <span class="text-xs text-base-content/70">
    取得中: {progress.current.toLocaleString()} / {progress.total.toLocaleString()}
  </span>
  <button class="btn btn-xs btn-error btn-outline" on:click|stopPropagation={stop}>停止</button>
```

変更後:
```svelte
{#if fetching}
  <ProgressBar current={progress.current} total={progress.total} cancelable on:cancel={stop} />
```

- [ ] **Step 3: ビルドチェック**

Run: `cd frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | grep -E '(Error|BulkFetch)' | head -20`
Expected: エラーなし

- [ ] **Step 4: コミット**

```bash
git add frontend/src/components/BulkFetchButton.svelte
git commit -m "refactor: BulkFetchButtonの進捗表示をProgressBarに差し替え"
```

---

### Task 5: DiffImportView.svelte の差分推定進捗を差し替え

**Files:**
- Modify: `frontend/src/views/DiffImportView.svelte:122-126`

- [ ] **Step 1: importを追加**

`DiffImportView.svelte` の `<script>` ブロック内に以下を追加:

```typescript
import ProgressBar from '../components/ProgressBar.svelte'
```

- [ ] **Step 2: 差分推定進捗表示を差し替え**

変更前（122-126行付近）:
```svelte
      {#if estimating}
        <span class="text-xs text-base-content/70">
          推定中: {estimateProgress.current.toLocaleString()} / {estimateProgress.total.toLocaleString()}
        </span>
        <button class="btn btn-xs btn-error btn-outline" on:click|stopPropagation={handleStopEstimate}>停止</button>
      {/if}
```

変更後:
```svelte
      {#if estimating}
        <ProgressBar current={estimateProgress.current} total={estimateProgress.total} cancelable on:cancel={handleStopEstimate} />
      {/if}
```

- [ ] **Step 3: ビルドチェック**

Run: `cd frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | grep -E '(Error|DiffImport)' | head -20`
Expected: エラーなし

- [ ] **Step 4: コミット**

```bash
git add frontend/src/views/DiffImportView.svelte
git commit -m "refactor: DiffImportViewの推定進捗表示をProgressBarに差し替え"
```

---

### Task 6: DifficultyTableSettings.svelte のテーブル更新進捗を差し替え

**Files:**
- Modify: `frontend/src/settings/DifficultyTableSettings.svelte:191-193`

- [ ] **Step 1: importを追加**

`DifficultyTableSettings.svelte` の `<script>` ブロック内に以下を追加:

```typescript
import ProgressBar from '../components/ProgressBar.svelte'
```

- [ ] **Step 2: テーブル更新進捗表示を差し替え**

変更前（190-197行付近）:
```svelte
    {#if tables.length > 0}
      <div class="flex items-center gap-2 mt-2">
        {#if refreshing}
          <span class="text-sm text-base-content/70">更新中: {refreshProgress.current}/{refreshProgress.total} テーブル完了</span>
          <button class="btn btn-sm btn-error btn-outline" on:click={handleStopRefresh}>停止</button>
        {:else}
          <button class="btn btn-sm btn-outline" on:click={handleRefreshAll}>全て更新</button>
        {/if}
      </div>
    {/if}
```

変更後:
```svelte
    {#if tables.length > 0}
      <div class="flex items-center gap-2 mt-2">
        {#if refreshing}
          <ProgressBar current={refreshProgress.current} total={refreshProgress.total} cancelable on:cancel={handleStopRefresh} />
        {:else}
          <button class="btn btn-sm btn-outline" on:click={handleRefreshAll}>全て更新</button>
        {/if}
      </div>
    {/if}
```

注: 結果リスト（`refreshResults` の `{#each}` ブロック）は変更しない。

- [ ] **Step 3: ビルドチェック**

Run: `cd frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | grep -E '(Error|DifficultyTable)' | head -20`
Expected: エラーなし

- [ ] **Step 4: コミット**

```bash
git add frontend/src/settings/DifficultyTableSettings.svelte
git commit -m "refactor: DifficultyTableSettingsの更新進捗表示をProgressBarに差し替え"
```

---

### Task 7: 最終ビルドチェック

- [ ] **Step 1: フルビルドチェック**

Run: `cd frontend && npx svelte-check --tsconfig ./tsconfig.json`
Expected: エラー 0件

- [ ] **Step 2: Goビルドチェック**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: エラーなし（Go側は変更なしだが念のため）

- [ ] **Step 3: Viteビルドチェック**

Run: `cd frontend && npx vite build`
Expected: ビルド成功
