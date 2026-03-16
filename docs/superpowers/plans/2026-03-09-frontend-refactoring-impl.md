# フロントエンドリファクタリング Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** フロントエンドの2つのリファクタリング（Wails生成型活用、IR一括取得パターン共通化）を実施する

**Architecture:** DuplicateView/DuplicateDetailのローカル型をWails生成型（`similarity`名前空間）に置換し、ChartListView/DifficultyTableViewで重複するIR一括取得のUI・ロジックを`BulkFetchButton.svelte`コンポーネントに抽出する

**Tech Stack:** Svelte 4, TypeScript, Wails v2

---

### Task 1: DuplicateViewのローカル型をWails生成型に置換

**Files:**
- Modify: `frontend/src/views/DuplicateView.svelte:1-57`

**Step 1: import追加・ローカル型削除**

`DuplicateView.svelte` の `<script>` を以下のように変更する:

```typescript
<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { ScanDuplicates } from '../../wailsjs/go/main/App'
  import type { similarity } from '../../wailsjs/go/models'

  const dispatch = createEventDispatcher()

  export let active = false

  let groups: similarity.DuplicateGroup[] = []
  let scanning = false
  let scanned = false
  let selectedGroupID: number | null = null

  async function handleScan() {
    scanning = true
    try {
      const result = await ScanDuplicates()
      groups = (result || []).sort((a, b) => b.Score - a.Score)
      scanned = true
    } finally {
      scanning = false
    }
  }

  function handleSelect(group: similarity.DuplicateGroup) {
    selectedGroupID = group.ID
    dispatch('select', group)
  }

  $: selectedGroup = groups.find(g => g.ID === selectedGroupID) || null
</script>
```

変更点:
- `type { similarity }` をimport
- ローカル型 `ScoreResult`, `DuplicateMember`, `DuplicateGroup` の3つを削除
- `groups` の型を `similarity.DuplicateGroup[]` に変更
- `handleSelect` の引数型を `similarity.DuplicateGroup` に変更

テンプレート部分は変更不要（フィールド名が同一のため）。

**Step 2: ビルド確認**

Run: `cd frontend && npm run check`
Expected: エラーなし

**Step 3: コミット**

```bash
git add frontend/src/views/DuplicateView.svelte
git commit -m "refactor: DuplicateViewのローカル型をWails生成型に置換"
```

---

### Task 2: DuplicateDetailのインライン型をWails生成型に置換

**Files:**
- Modify: `frontend/src/views/DuplicateDetail.svelte:1-19`

**Step 1: import追加・インライン型置換**

`DuplicateDetail.svelte` の `<script>` 冒頭を以下のように変更する:

```typescript
<script lang="ts">
  import { GetSongDetail } from '../../wailsjs/go/app/SongHandler'
  import type { dto, similarity } from '../../wailsjs/go/models'

  export let group: similarity.DuplicateGroup | null = null
```

変更点:
- `similarity` をimportに追加
- `group` プロパティの型を `similarity.DuplicateGroup | null` に変更（インライン型定義18行を1行に）

テンプレート部分は変更不要（フィールド名が同一のため）。

**Step 2: ビルド確認**

Run: `cd frontend && npm run check`
Expected: エラーなし

**Step 3: コミット**

```bash
git add frontend/src/views/DuplicateDetail.svelte
git commit -m "refactor: DuplicateDetailのインライン型をWails生成型に置換"
```

---

### Task 3: BulkFetchButton コンポーネントの作成

**Files:**
- Create: `frontend/src/components/BulkFetchButton.svelte`

**Step 1: コンポーネント作成**

ChartListView.svelte のIR一括取得ロジック（状態変数、start/stop関数、EventsOnリスナー、テンプレート）を抽出して新コンポーネントを作成する。

```svelte
<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime'

  // 開始APIを呼ぶ関数。呼び出し元が差し替える
  export let startFn: () => Promise<void>
  export let stopFn: () => void

  const dispatch = createEventDispatcher<{ done: void }>()

  let fetching = false
  let progress = { current: 0, total: 0 }
  let doneMessage = ''
  let doneTimer: ReturnType<typeof setTimeout> | null = null

  export function start() {
    fetching = true
    progress = { current: 0, total: 0 }
    doneMessage = ''
    if (doneTimer) { clearTimeout(doneTimer); doneTimer = null }
    startFn().catch((e: Error) => {
      console.error('[IR] BulkFetch failed:', e)
      fetching = false
    })
  }

  function stop() {
    stopFn()
  }

  let offProgress: (() => void) | null = null
  let offDone: (() => void) | null = null

  onMount(() => {
    offProgress = EventsOn('ir:progress', (data: { current: number; total: number }) => {
      progress = data
    })
    offDone = EventsOn('ir:done', (data: { total: number; fetched: number; notFound: number; failed: number; cancelled: boolean }) => {
      fetching = false
      const parts: string[] = []
      if (data.total === 0) {
        doneMessage = '対象なし'
      } else {
        if (data.fetched > 0) parts.push(`${data.fetched}件取得`)
        if (data.notFound > 0) parts.push(`${data.notFound}件未登録`)
        if (data.failed > 0) parts.push(`${data.failed}件失敗`)
        if (data.cancelled) parts.push('中断')
        doneMessage = parts.join(', ') || '完了'
      }
      doneTimer = setTimeout(() => { doneMessage = '' }, 5000)
      dispatch('done')
    })
  })

  onDestroy(() => {
    offProgress?.()
    offDone?.()
    if (doneTimer) clearTimeout(doneTimer)
  })
</script>

{#if fetching}
  <span class="text-xs text-base-content/70">
    取得中: {progress.current.toLocaleString()} / {progress.total.toLocaleString()}
  </span>
  <button class="btn btn-xs btn-error btn-outline" on:click|stopPropagation={stop}>停止</button>
{:else if doneMessage}
  <span class="text-xs text-success">{doneMessage}</span>
{:else}
  <button class="btn btn-xs btn-outline" on:click|stopPropagation={start}>IR取得</button>
{/if}
```

**Step 2: ビルド確認**

Run: `cd frontend && npm run check`
Expected: エラーなし（未使用コンポーネントだが型エラーがないことを確認）

**Step 3: コミット**

```bash
git add frontend/src/components/BulkFetchButton.svelte
git commit -m "feat: IR一括取得の共通コンポーネント BulkFetchButton を追加"
```

---

### Task 4: ChartListView に BulkFetchButton を適用

**Files:**
- Modify: `frontend/src/views/ChartListView.svelte`

**Step 1: IR関連コードを BulkFetchButton に置換**

削除するもの:
- 状態変数: `irFetching`, `irProgress`, `irDoneMessage`, `irDoneTimer`（35-38行目）
- 関数: `startBulkFetch`, `stopBulkFetch`（64-80行目）
- EventsOnリスナー: `offProgress`, `offDone` の宣言（190-191行目）
- onMount内: `ir:progress` と `ir:done` のEventsOn登録（196-216行目）
- onDestroy内: `offProgress?.()`, `offDone?.()`, `irDoneTimer` クリーンアップ（243-245行目）
- テンプレート: IR取得ボタン・進捗表示の `{#if irFetching}...{/if}` ブロック（302-311行目）

追加するもの:
- import: `import BulkFetchButton from '../components/BulkFetchButton.svelte'`
- import: `StartBulkFetch`, `StopBulkFetch` は残す（BulkFetchButtonのpropsに渡す）
- テンプレート: 上記ブロックの代わりに以下を配置:

```svelte
<BulkFetchButton
  startFn={StartBulkFetch}
  stopFn={StopBulkFetch}
  on:done={() => ListCharts().then(c => { charts = c || [] }).catch(console.error)}
/>
```

**Step 2: ビルド確認**

Run: `cd frontend && npm run check`
Expected: エラーなし

**Step 3: 動作確認**

Run: `wails dev`
確認項目:
- 譜面一覧タブでIR取得ボタンが表示される
- ボタンクリックで取得が開始され、進捗が表示される
- 停止ボタンで中断できる
- 完了後にメッセージが表示され5秒後に消える

**Step 4: コミット**

```bash
git add frontend/src/views/ChartListView.svelte
git commit -m "refactor: ChartListViewのIR取得ロジックをBulkFetchButtonに置換"
```

---

### Task 5: DifficultyTableView に BulkFetchButton を適用

**Files:**
- Modify: `frontend/src/views/DifficultyTableView.svelte`

**Step 1: IR関連コードを BulkFetchButton に置換**

削除するもの:
- 状態変数: `irFetching`, `irProgress`, `irDoneMessage`, `irDoneTimer`（39-43行目）
- 関数: `startBulkFetch`, `stopBulkFetch`（45-59行目）
- EventsOnリスナー: `offProgress`, `offDone` の宣言（131-132行目）
- onMount内: `ir:progress` と `ir:done` のEventsOn登録（135-151行目）
- onDestroy内: `offProgress?.()`, `offDone?.()`, `irDoneTimer` クリーンアップ（167-170行目）
- テンプレート: IR取得ボタン・進捗表示の `{#if irFetching}...{/if}` ブロック（260-269行目）
- import: `EventsOn`（EventsOnの他の使用箇所がなければ削除）

追加するもの:
- import: `import BulkFetchButton from '../components/BulkFetchButton.svelte'`
- `selectedTableId` をstartFnのクロージャで利用するため、テンプレートでラムダを使う
- テンプレート: 上記ブロックの代わりに以下を配置:

```svelte
<BulkFetchButton
  startFn={() => selectedTableId ? StartDifficultyTableBulkFetch(selectedTableId) : Promise.resolve()}
  stopFn={StopBulkFetch}
  on:done={() => selectedTableId && loadEntries(selectedTableId)}
/>
```

注意: `StartDifficultyTableBulkFetch` は引数が必要なため、`startFn` はラムダで包む。

**Step 2: ビルド確認**

Run: `cd frontend && npm run check`
Expected: エラーなし

**Step 3: 動作確認**

Run: `wails dev`
確認項目:
- 難易度表タブでIR取得ボタンが表示される
- ボタンクリックで取得が開始され、進捗が表示される
- 停止ボタンで中断できる
- 完了後にメッセージが表示され5秒後に消える
- 難易度表を切り替えても正常に動作する

**Step 4: コミット**

```bash
git add frontend/src/views/DifficultyTableView.svelte
git commit -m "refactor: DifficultyTableViewのIR取得ロジックをBulkFetchButtonに置換"
```

---

### Task 6: TODO.md更新

**Files:**
- Modify: `docs/TODO.md`

**Step 1: リファクタリング項目を実装済みに移動**

フロントエンドセクションの2項目にチェックを付ける:
```
- [x] IR一括取得イベント処理パターンの共通化（ChartListView / DifficultyTableView で重複）
- [x] Wails生成型の活用（DuplicateView でローカル型を再定義している箇所を解消）
```

**Step 2: コミット**

```bash
git add docs/TODO.md
git commit -m "docs: フロントエンドリファクタリング2項目を実装済みに更新"
```
