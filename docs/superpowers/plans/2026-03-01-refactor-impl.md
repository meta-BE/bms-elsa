# フロントエンドコンポーネントリファクタ 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** App.svelteと各テーブル/詳細コンポーネントの重複コードを共通コンポーネントに抽出し保守性を向上させる

**Architecture:** ボトムアップで小さい共通部品から抽出。ユーティリティ関数→SearchInput→SortableHeader→SplitPaneの順に進める。各ステップでビルド確認し、壊れたらすぐ修正。

**Tech Stack:** Svelte 3, TypeScript, Tanstack Table, DaisyUI/Tailwind CSS

**設計ドキュメント:** `docs/plans/2026-03-01-refactor-design.md`

---

### Task 1: ユーティリティ関数の抽出

**Files:**
- Create: `frontend/src/utils/chartLabels.ts`
- Modify: `frontend/src/SongDetail.svelte`
- Modify: `frontend/src/ChartDetail.svelte`
- Modify: `frontend/src/EntryDetail.svelte`

**概要:** 3つの詳細コンポーネントに完全コピーされている`modeLabel()`と`diffLabel()`を共通モジュールに抽出する。

**Step 1: 共通モジュールを作成**

`frontend/src/utils/chartLabels.ts` を新規作成:

```typescript
export function modeLabel(mode: number): string {
  const labels: Record<number, string> = { 5: '5K', 7: '7K', 9: 'PMS', 10: '10K', 14: '14K', 25: '24K' }
  return labels[mode] || `${mode}K`
}

export function diffLabel(diff: number): string {
  const labels = ['', 'BEG', 'NOR', 'HYP', 'ANO', 'INS']
  return labels[diff] || ''
}
```

**Step 2: SongDetail.svelteを修正**

scriptセクション先頭のimport群に追加:
```typescript
import { modeLabel, diffLabel } from './utils/chartLabels'
```

65-73行目のローカル`modeLabel`関数と`diffLabel`関数の定義を削除。

**Step 3: ChartDetail.svelteを修正**

scriptセクション先頭のimport群に追加:
```typescript
import { modeLabel, diffLabel } from './utils/chartLabels'
```

47-55行目のローカル`modeLabel`関数と`diffLabel`関数の定義を削除。

**Step 4: EntryDetail.svelteを修正**

scriptセクション先頭のimport群に追加:
```typescript
import { modeLabel, diffLabel } from './utils/chartLabels'
```

48-56行目のローカル`modeLabel`関数と`diffLabel`関数の定義を削除。

**Step 5: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: BUILD SUCCESS

**Step 6: コミット**

```bash
git add frontend/src/utils/chartLabels.ts frontend/src/SongDetail.svelte frontend/src/ChartDetail.svelte frontend/src/EntryDetail.svelte
git commit -m "refactor: modeLabel/diffLabelをutils/chartLabels.tsに抽出"
```

---

### Task 2: SearchInputコンポーネントの作成

**Files:**
- Create: `frontend/src/SearchInput.svelte`
- Modify: `frontend/src/SongTable.svelte`
- Modify: `frontend/src/ChartListView.svelte`
- Modify: `frontend/src/DifficultyTableView.svelte`

**概要:** 3つのテーブルで重複している「検索窓 + オーバーレイクリアボタン」を共通コンポーネントに抽出する。

**Step 1: SearchInput.svelteを作成**

`frontend/src/SearchInput.svelte` を新規作成:

```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte'

  const dispatch = createEventDispatcher<{ input: void; clear: void }>()

  export let value = ''
  export let placeholder = '検索...'

  function handleClear() {
    value = ''
    dispatch('clear')
  }
</script>

<div class="relative">
  <input
    type="text"
    {placeholder}
    class="input input-xs input-bordered w-48 pr-6"
    bind:value
    on:input={() => dispatch('input')}
  />
  {#if value}
    <button
      class="absolute right-1 top-1/2 -translate-y-1/2 btn btn-ghost btn-xs btn-circle h-4 w-4 min-h-0 p-0"
      on:click={handleClear}
    >✕</button>
  {/if}
</div>
```

**Step 2: SongTable.svelteを修正**

importに追加:
```typescript
import SearchInput from './SearchInput.svelte'
```

ヘッダー行内の検索UI（`<div class="relative">` ～ `</div>`）を以下に置換:
```svelte
<SearchInput bind:value={searchText} on:input={handleSearchInput} on:clear={doSearch} />
```

**Step 3: ChartListView.svelteを修正**

importに追加:
```typescript
import SearchInput from './SearchInput.svelte'
```

ヘッダー行内の検索UI（`<div class="relative">` ～ `</div>`）を以下に置換:
```svelte
<SearchInput bind:value={globalFilter} />
```

注: ChartListViewはTanstack Tableのリアクティブフィルタを使うため、`on:input`や`on:clear`は不要。`globalFilter`の変更でテーブルが自動更新される。

**Step 4: DifficultyTableView.svelteを修正**

importに追加:
```typescript
import SearchInput from './SearchInput.svelte'
```

ヘッダー行内の検索UI（`<div class="relative">` ～ `</div>`）を以下に置換:
```svelte
<SearchInput bind:value={searchText} on:input={applyFilter} on:clear={applyFilter} />
```

**Step 5: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: BUILD SUCCESS

**Step 6: コミット**

```bash
git add frontend/src/SearchInput.svelte frontend/src/SongTable.svelte frontend/src/ChartListView.svelte frontend/src/DifficultyTableView.svelte
git commit -m "refactor: 検索UIをSearchInputコンポーネントに抽出"
```

---

### Task 3: SortableHeaderコンポーネントの作成

**Files:**
- Create: `frontend/src/SortableHeader.svelte`
- Modify: `frontend/src/SongTable.svelte`
- Modify: `frontend/src/ChartListView.svelte`
- Modify: `frontend/src/DifficultyTableView.svelte`

**概要:** 3つのテーブルで完全に同一のソート可能ヘッダー行を共通コンポーネントに抽出する。

**Step 1: SortableHeader.svelteを作成**

`frontend/src/SortableHeader.svelte` を新規作成。Tanstack Tableの`Table`インスタンスを受け取り、ヘッダーグループを描画する:

```svelte
<script lang="ts">
  import { flexRender, type Table } from '@tanstack/svelte-table'

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  export let table: Table<any>
</script>

<div class="bg-base-200 border-b border-base-300 px-2 shrink-0">
  {#each table.getHeaderGroups() as headerGroup}
    <div class="flex">
      {#each headerGroup.headers as header}
        <div
          role="columnheader"
          tabindex="0"
          class="px-2 py-1.5 text-xs font-bold uppercase cursor-pointer select-none hover:bg-base-300 transition-colors truncate"
          style="width: {header.getSize()}px; min-width: {header.getSize()}px"
          on:click|stopPropagation={header.column.getToggleSortingHandler()}
          on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') header.column.getToggleSortingHandler()?.(e) }}
        >
          <span class="flex items-center gap-1">
            {#if !header.isPlaceholder}
              <svelte:component
                this={flexRender(header.column.columnDef.header, header.getContext())}
              />
            {/if}
            {#if header.column.getIsSorted() === 'asc'}
              <span>▲</span>
            {:else if header.column.getIsSorted() === 'desc'}
              <span>▼</span>
            {/if}
          </span>
        </div>
      {/each}
    </div>
  {/each}
</div>
```

**Step 2: SongTable.svelteを修正**

importに追加:
```typescript
import SortableHeader from './SortableHeader.svelte'
```

`<!-- ヘッダー（スクロールしない） -->`のコメントから`</div>`までのヘッダーHTMLブロック全体を以下に置換:
```svelte
<SortableHeader table={$table} />
```

不要になったimportを削除: `flexRender`（セル描画でまだ使用している場合は残す）

**Step 3: ChartListView.svelteを修正**

importに追加:
```typescript
import SortableHeader from './SortableHeader.svelte'
```

`<!-- テーブルヘッダー -->`のコメントから対応する`</div>`までのヘッダーHTMLブロック全体を以下に置換:
```svelte
<SortableHeader table={$table} />
```

**Step 4: DifficultyTableView.svelteを修正**

importに追加:
```typescript
import SortableHeader from './SortableHeader.svelte'
```

`<!-- ヘッダー -->`のコメントから対応する`</div>`までのヘッダーHTMLブロック全体を以下に置換:
```svelte
<SortableHeader table={$table} />
```

**Step 5: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: BUILD SUCCESS

**Step 6: コミット**

```bash
git add frontend/src/SortableHeader.svelte frontend/src/SongTable.svelte frontend/src/ChartListView.svelte frontend/src/DifficultyTableView.svelte
git commit -m "refactor: ソートヘッダーをSortableHeaderコンポーネントに抽出"
```

---

### Task 4: SplitPaneコンポーネントの作成とApp.svelteリファクタ

**Files:**
- Create: `frontend/src/SplitPane.svelte`
- Modify: `frontend/src/App.svelte`

**概要:** App.svelteで3回繰り返されている「上部リスト + ドラッグセパレーター + 下部詳細」パターンをSplitPaneに抽出。ドラッグリサイズのロジックをSplitPane内に閉じ込める。

**Step 1: SplitPane.svelteを作成**

`frontend/src/SplitPane.svelte` を新規作成:

```svelte
<script lang="ts">
  export let showDetail = false
  export let splitRatio = 0.6

  let containerEl: HTMLDivElement
  let dragging = false

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

<div bind:this={containerEl} class="h-full flex flex-col">
  <div class="overflow-hidden" style="flex: {showDetail ? splitRatio : 1}">
    <slot name="list" />
  </div>
  {#if showDetail}
    <!-- svelte-ignore a11y-no-noninteractive-tabindex a11y-no-noninteractive-element-interactions -->
    <div
      class="h-1 shrink-0 cursor-row-resize bg-base-300 hover:bg-primary/30 transition-colors my-1 rounded"
      on:mousedown={onDragStart}
      role="separator"
      tabindex="0"
    ></div>
    <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
    <div class="overflow-y-auto" style="flex: {1 - splitRatio}" on:click|stopPropagation>
      <slot name="detail" />
    </div>
  {/if}
</div>
```

**Step 2: App.svelteを修正**

importに追加:
```typescript
import SplitPane from './SplitPane.svelte'
```

script部から以下を削除:
- `let containerEl: HTMLDivElement`
- `let dragging = false`
- `let splitRatio = 0.6`
- `onDragStart`, `onDragMove`, `onDragEnd` 関数（3つとも）

`let splitRatio = 0.6` はSplitPaneの内部状態になるが、3つのタブで共有する場合はApp.svelteに残してbindする。ただし各タブで独立した比率にしたい場合は個別に管理してもよい。ここでは共有（1つのsplitRatio）とする:

App.svelte scriptに残す:
```typescript
let splitRatio = 0.6
```

template部を以下のように変更。現在の`<div bind:this={containerEl} ...>` コンテナを `<div class="flex-1 overflow-hidden p-4">` に変更し、内部を3つのSplitPaneに:

```svelte
<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div class="flex-1 overflow-hidden p-4" on:click={handleDeselect}>
  <div class="h-full" class:hidden={activeTab !== 'songs'}>
    <SplitPane showDetail={!!selectedFolderHash} bind:splitRatio>
      <SongTable slot="list" selected={selectedFolderHash} on:select={handleSelect} on:deselect={handleDeselect} />
      <SongDetail slot="detail" folderHash={selectedFolderHash} on:close={handleClose} />
    </SplitPane>
  </div>

  <div class="h-full" class:hidden={activeTab !== 'charts'}>
    <SplitPane showDetail={!!selectedChartMD5} bind:splitRatio>
      <ChartListView slot="list" selected={selectedChartMD5} on:select={handleChartSelect} on:deselect={handleChartDeselect} />
      <ChartDetail slot="detail" md5={selectedChartMD5} on:close={() => { selectedChartMD5 = null }} />
    </SplitPane>
  </div>

  <div class="h-full" class:hidden={activeTab !== 'difficulty'}>
    <SplitPane showDetail={!!(selectedEntryMD5 && selectedEntryData)} bind:splitRatio>
      <DifficultyTableView slot="list" selected={selectedEntryMD5} on:select={handleEntrySelect} on:deselect={handleEntryDeselect} />
      <EntryDetail slot="detail" md5={selectedEntryMD5} entryData={selectedEntryData} on:close={handleClose} />
    </SplitPane>
  </div>
</div>
```

注意: `selectedFolderHash`がnullの場合でも`<SongDetail slot="detail">`は常にDOMに存在するが、SplitPaneの`showDetail=false`で非表示になる。SongDetailが`folderHash`がnullの場合に適切にハンドリングしているか確認し、必要に応じて`{#if}`で囲む。

**Step 3: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: BUILD SUCCESS

**Step 4: コミット**

```bash
git add frontend/src/SplitPane.svelte frontend/src/App.svelte
git commit -m "refactor: SplitPaneコンポーネントに抽出しApp.svelteを簡素化"
```

---

### Task 5: 全体ビルド・動作確認

**Step 1: Goテスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./...`
Expected: 全テスト PASS

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: BUILD SUCCESS
