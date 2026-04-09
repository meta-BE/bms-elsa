# カラム幅リサイズの全テーブル展開と永続化 実装プラン

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 全TanStack Tableビューにカラムリサイズを展開し、カラム幅をconfig.jsonに永続化する。ウィンドウリサイズ時の比例再計算と、Settingsモーダルからのテーブル別リセットも実装する。

**Architecture:** Go側Config structに`ColumnWidths map[string]map[string]float64`を追加し、既存のGetConfig/SaveConfig経由で読み書き。フロントエンドでは共通ユーティリティ`columnResize.ts`にリサイズ状態管理・永続化・復元・ウィンドウリサイズ対応のロジックを集約し、各ビューから利用する。SortableHeaderのリサイズ完了時にコールバック経由で保存をトリガーする。

**Tech Stack:** Go, Svelte (v4), TanStack Table (v8), Wails v2

**設計書:** `docs/superpowers/specs/2026-04-10-column-width-persistence-design.md`

---

## ファイル構成

| ファイル | 操作 | 役割 |
|---|---|---|
| `app.go:222-225` | 修正 | Config structにColumnWidthsフィールド追加 |
| `frontend/wailsjs/go/models.ts:504-517` | 自動生成 | `wails dev`起動時に自動更新 |
| `frontend/src/utils/columnResize.ts` | 新規 | カラム幅の永続化・復元・ウィンドウリサイズ対応ユーティリティ |
| `frontend/src/components/SortableHeader.svelte` | 修正 | リサイズ完了時のコールバック追加 |
| `frontend/src/views/DiffImportView.svelte` | 修正 | 共通ユーティリティ利用に移行 |
| `frontend/src/views/ChartListView.svelte` | 修正 | カラムリサイズ有効化 |
| `frontend/src/views/SongListView.svelte` | 修正 | カラムリサイズ有効化 |
| `frontend/src/views/DifficultyTableView.svelte` | 修正 | カラムリサイズ有効化 |
| `frontend/src/settings/Settings.svelte` | 修正 | テーブル別リセットボタン追加 |

---

## Task 1: Go側Config structの拡張

**Files:**
- Modify: `app.go:222-225`

- [ ] **Step 1: Config structにColumnWidthsフィールドを追加**

`app.go` の Config struct を以下のように変更:

```go
type Config struct {
	SongdataDBPath string                        `json:"songdataDBPath"`
	FileLog        bool                          `json:"fileLog"`
	ColumnWidths   map[string]map[string]float64 `json:"columnWidths,omitempty"`
}
```

`omitempty`により、カラム幅が未設定の場合はconfig.jsonに出力されない。

- [ ] **Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: エラーなし

- [ ] **Step 3: コミット**

```bash
git add app.go
git commit -m "feat: Config structにColumnWidthsフィールドを追加"
```

---

## Task 2: カラム幅ユーティリティの作成

**Files:**
- Create: `frontend/src/utils/columnResize.ts`

- [ ] **Step 1: columnResize.tsを作成**

```typescript
import type { ColumnSizingState } from '@tanstack/svelte-table'
import { GetConfig, SaveConfig } from '../../wailsjs/go/main/App'

/** ビュー識別キー */
export type ViewId = 'chartList' | 'songList' | 'difficultyTable' | 'diffImport'

interface ColumnResizeConfig {
  /** ビュー識別キー */
  viewId: ViewId
  /** リサイズ可能カラムのIDリスト（順不同、キー集合の整合チェックに使用） */
  resizableColumnIds: string[]
  /** リサイズ不可カラムの固定幅合計（px） */
  fixedColumnsWidth: number
  /** カラム定義のデフォルトサイズ比率（flex比）。ID → 定義上のsize値 */
  defaultSizes: Record<string, number>
}

/**
 * config.jsonからカラム幅の割合を読み込み、ColumnSizingStateに変換する。
 * - configにキーがない場合: null（flex初期化にフォールバック）
 * - キー集合が不一致の場合: configから削除してnull
 */
export async function loadColumnWidths(
  config: ColumnResizeConfig,
  containerWidth: number,
): Promise<ColumnSizingState | null> {
  const cfg = await GetConfig()
  const saved = cfg.columnWidths?.[config.viewId]
  if (!saved) return null

  // キー集合の整合チェック
  const savedKeys = Object.keys(saved).sort()
  const expectedKeys = [...config.resizableColumnIds].sort()
  if (savedKeys.length !== expectedKeys.length || savedKeys.some((k, i) => k !== expectedKeys[i])) {
    // 不一致: 該当ビューの設定を削除
    const columnWidths = { ...cfg.columnWidths }
    delete columnWidths[config.viewId]
    await SaveConfig({ ...cfg, columnWidths })
    return null
  }

  // 割合 → px変換
  const available = Math.max(0, containerWidth - config.fixedColumnsWidth)
  const sizing: ColumnSizingState = {}
  for (const [id, ratio] of Object.entries(saved)) {
    sizing[id] = Math.round(ratio * available)
  }
  return sizing
}

/**
 * 現在のColumnSizingStateを割合に変換してconfig.jsonに保存する。
 */
export async function saveColumnWidths(
  viewId: ViewId,
  columnSizing: ColumnSizingState,
  resizableColumnIds: string[],
  fixedColumnsWidth: number,
  containerWidth: number,
): Promise<void> {
  const available = Math.max(1, containerWidth - fixedColumnsWidth)
  const ratios: Record<string, number> = {}
  for (const id of resizableColumnIds) {
    const px = columnSizing[id]
    if (px != null) {
      ratios[id] = Math.round((px / available) * 10000) / 10000
    }
  }

  const cfg = await GetConfig()
  const columnWidths = { ...cfg.columnWidths, [viewId]: ratios }
  await SaveConfig({ ...cfg, columnWidths })
}

/**
 * 保存済み割合とコンテナ幅からColumnSizingStateを再計算する。
 * ウィンドウリサイズ時に使用。
 */
export function recalcFromRatios(
  ratios: Record<string, number>,
  fixedColumnsWidth: number,
  containerWidth: number,
): ColumnSizingState {
  const available = Math.max(0, containerWidth - fixedColumnsWidth)
  const sizing: ColumnSizingState = {}
  for (const [id, ratio] of Object.entries(ratios)) {
    sizing[id] = Math.round(ratio * available)
  }
  return sizing
}

/**
 * ColumnSizingStateから割合マップを算出する（メモリ上の保持用）。
 */
export function toRatios(
  columnSizing: ColumnSizingState,
  resizableColumnIds: string[],
  fixedColumnsWidth: number,
  containerWidth: number,
): Record<string, number> {
  const available = Math.max(1, containerWidth - fixedColumnsWidth)
  const ratios: Record<string, number> = {}
  for (const id of resizableColumnIds) {
    const px = columnSizing[id]
    if (px != null) {
      ratios[id] = Math.round((px / available) * 10000) / 10000
    }
  }
  return ratios
}
```

- [ ] **Step 2: コミット**

```bash
git add frontend/src/utils/columnResize.ts
git commit -m "feat: カラム幅の永続化・復元ユーティリティを追加"
```

---

## Task 3: SortableHeaderにリサイズ完了コールバックを追加

**Files:**
- Modify: `frontend/src/components/SortableHeader.svelte`

- [ ] **Step 1: onResizeEndコールバックpropsを追加**

`SortableHeader.svelte` の `<script>` セクション冒頭（`export let table` の後）に追加:

```typescript
export let onResizeEnd: (() => void) | undefined = undefined
```

- [ ] **Step 2: onEnd関数内でコールバックを呼び出す**

`SortableHeader.svelte` の `onEnd` 関数（94行目付近）を修正。`resizingColumnId = null` の後にコールバック呼び出しを追加:

```typescript
function onEnd() {
  resizingColumnId = null
  document.removeEventListener('mousemove', onMove as EventListener)
  document.removeEventListener('mouseup', onEnd)
  document.removeEventListener('touchmove', onMove as EventListener)
  document.removeEventListener('touchend', onEnd)
  onResizeEnd?.()
}
```

- [ ] **Step 3: コミット**

```bash
git add frontend/src/components/SortableHeader.svelte
git commit -m "feat: SortableHeaderにリサイズ完了コールバックを追加"
```

---

## Task 4: DiffImportViewを共通ユーティリティに移行

**Files:**
- Modify: `frontend/src/views/DiffImportView.svelte`

- [ ] **Step 1: import追加とリサイズ設定定義**

`DiffImportView.svelte` の import セクション（1-18行目）の末尾に追加:

```typescript
import {
  loadColumnWidths,
  saveColumnWidths,
  recalcFromRatios,
  toRatios,
  type ViewId,
} from '../utils/columnResize'
```

columns定義（119-172行目）の後、`$: importableCount` の前に以下を追加:

```typescript
const VIEW_ID: ViewId = 'diffImport'
const RESIZABLE_IDS = columns
  .filter(c => c.enableResizing !== false && (c.meta as { flex?: boolean })?.flex)
  .map(c => c.id || (c as { accessorKey?: string }).accessorKey || '')
const FIXED_WIDTH = columns
  .filter(c => !(c.meta as { flex?: boolean })?.flex)
  .reduce((sum, c) => sum + (c.size || 150), 0)
const CONTAINER_PADDING = 16
```

- [ ] **Step 2: 割合のメモリ保持用変数を追加**

`columnSizing` 定義（176行目付近）の近くに追加:

```typescript
let currentRatios: Record<string, number> = {}
```

- [ ] **Step 3: afterUpdate内のflex初期化ロジックを修正して、config読み込みを優先する**

`afterUpdate` ブロック（212-231行目）を以下に置き換え:

```typescript
afterUpdate(() => {
  if (candidates.length > 0 && scrollElement && !widthsLocked) {
    widthsLocked = true
    requestAnimationFrame(async () => {
      const containerWidth = scrollElement.clientWidth - CONTAINER_PADDING
      // configから復元を試みる
      const restored = await loadColumnWidths(
        { viewId: VIEW_ID, resizableColumnIds: RESIZABLE_IDS, fixedColumnsWidth: FIXED_WIDTH, defaultSizes: {} },
        containerWidth,
      )
      if (restored) {
        columnSizing = restored
        currentRatios = toRatios(restored, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
        return
      }
      // configになければflex比から初期化
      const flexCols = columns.filter(c => (c.meta as { flex?: boolean })?.flex)
      const flexDefined = flexCols.reduce((sum, c) => sum + (c.size || 150), 0)
      const available = Math.max(0, containerWidth - FIXED_WIDTH)
      const newSizing: Record<string, number> = {}
      for (const col of flexCols) {
        const id = col.id || (col as { accessorKey?: string }).accessorKey || ''
        newSizing[id] = Math.round(((col.size || 150) / flexDefined) * available)
      }
      columnSizing = newSizing
      currentRatios = toRatios(newSizing, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
    })
  }
})
```

- [ ] **Step 4: リサイズ完了時の保存関数を追加**

`afterUpdate` ブロックの後に追加:

```typescript
function handleResizeEnd() {
  if (!scrollElement) return
  const containerWidth = scrollElement.clientWidth - CONTAINER_PADDING
  currentRatios = toRatios(columnSizing, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
  saveColumnWidths(VIEW_ID, columnSizing, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
}
```

- [ ] **Step 5: ウィンドウリサイズ対応を追加**

`handleResizeEnd`の後に追加:

```typescript
function handleWindowResize() {
  if (!scrollElement || !widthsLocked || Object.keys(currentRatios).length === 0) return
  const containerWidth = scrollElement.clientWidth - CONTAINER_PADDING
  columnSizing = recalcFromRatios(currentRatios, FIXED_WIDTH, containerWidth)
}
```

- [ ] **Step 6: clearAll時にcurrentRatiosもリセット**

`clearAll` 関数（81-86行目）の中に `currentRatios = {}` を追加:

```typescript
function clearAll() {
  candidates = []
  importResult = null
  columnSizing = {}
  currentRatios = {}
  widthsLocked = false
}
```

- [ ] **Step 7: リセットイベントリスナーを追加**

`onMount` 内（または直後）にWailsイベントリスナーを追加。`onDestroy` でクリーンアップ。

importに `EventsOn` を追加（既にimportされている）:

```typescript
// DiffImportView.svelteでは既にEventsOnがimportされている
```

`onMount` ブロック内の末尾に追加:

```typescript
const offResetWidths = EventsOn('column-width-reset', (id: string) => {
  if (id !== VIEW_ID) return
  columnSizing = {}
  currentRatios = {}
  widthsLocked = false
})
```

`onDestroy` ブロック内に追加:

```typescript
offResetWidths?.()
```

- [ ] **Step 8: テンプレートにイベントバインドを追加**

テンプレート内の `<SortableHeader table={$table} />`（278行目）を修正:

```svelte
<SortableHeader table={$table} onResizeEnd={handleResizeEnd} />
```

`<svelte:window>` タグをテンプレートの先頭に追加（234行目付近、既存のdiv開始前）:

```svelte
<svelte:window on:resize={handleWindowResize} />
```

- [ ] **Step 9: コミット**

```bash
git add frontend/src/views/DiffImportView.svelte
git commit -m "feat: DiffImportViewのカラム幅を永続化・ウィンドウリサイズ対応"
```

---

## Task 5: ChartListViewにカラムリサイズを追加

**Files:**
- Modify: `frontend/src/views/ChartListView.svelte`

- [ ] **Step 1: import追加**

`ChartListView.svelte` の `@tanstack/svelte-table` importに `ColumnSizingState`, `ColumnSizingInfoState` を追加:

```typescript
import {
  createSvelteTable,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  getFacetedRowModel,
  getFacetedUniqueValues,
  flexRender,
  type ColumnDef,
  type SortingState,
  type FilterFn,
  type ColumnSizingState,
  type ColumnSizingInfoState,
} from '@tanstack/svelte-table'
```

`SortableHeader` importの後に追加:

```typescript
import {
  loadColumnWidths,
  saveColumnWidths,
  recalcFromRatios,
  toRatios,
  type ViewId,
} from '../utils/columnResize'
```

- [ ] **Step 2: `ir`カラムにenableResizing: falseを設定**

`columns` 定義（56-103行目）の `ir` カラム（96-102行目）を修正:

```typescript
{
  id: 'ir',
  header: 'IR',
  size: 40,
  meta: { align: 'center' },
  enableResizing: false,
  accessorFn: (row) => row.hasIrMeta ? '●' : '',
},
```

- [ ] **Step 3: リサイズ状態管理の変数を追加**

`$: table = createSvelteTable({...})` の前（104行目付近）に追加:

```typescript
const VIEW_ID: ViewId = 'chartList'
const RESIZABLE_IDS = columns
  .filter(c => c.enableResizing !== false)
  .filter(c => c.id !== 'ir')
  .map(c => c.id || (c as { accessorKey?: string }).accessorKey || '')
const FIXED_WIDTH = columns
  .filter(c => c.enableResizing === false)
  .reduce((sum, c) => sum + (c.size || 150), 0)
const CONTAINER_PADDING = 16

let columnSizing: ColumnSizingState = {}
let columnSizingInfo: ColumnSizingInfoState = {} as ColumnSizingInfoState
let widthsLocked = false
let currentRatios: Record<string, number> = {}
```

- [ ] **Step 4: createSvelteTableにリサイズオプションを追加**

`$: table = createSvelteTable({...})` （105-118行目）を修正:

```typescript
$: table = createSvelteTable({
  data: charts,
  columns,
  enableColumnResizing: true,
  columnResizeMode: 'onChange',
  state: { sorting, globalFilter, columnSizing, columnSizingInfo },
  onSortingChange: (updater) => {
    sorting = typeof updater === 'function' ? updater(sorting) : updater
  },
  onColumnSizingChange: (updater) => {
    columnSizing = typeof updater === 'function' ? updater(columnSizing) : updater
  },
  onColumnSizingInfoChange: (updater) => {
    columnSizingInfo = typeof updater === 'function' ? updater(columnSizingInfo) : updater
  },
  globalFilterFn: searchFilter,
  getCoreRowModel: getCoreRowModel(),
  getFilteredRowModel: getFilteredRowModel(),
  getSortedRowModel: getSortedRowModel(),
  getFacetedRowModel: getFacetedRowModel(),
  getFacetedUniqueValues: getFacetedUniqueValues(),
})
```

- [ ] **Step 5: 初期化・永続化・ウィンドウリサイズのロジックを追加**

`onMount` importに `afterUpdate` を追加:

```typescript
import { onMount, afterUpdate, createEventDispatcher } from 'svelte'
```

`onMount` ブロック（141-147行目）の後に追加:

```typescript
afterUpdate(() => {
  if (charts.length > 0 && scrollElement && !widthsLocked) {
    widthsLocked = true
    requestAnimationFrame(async () => {
      const containerWidth = scrollElement.clientWidth - CONTAINER_PADDING
      const restored = await loadColumnWidths(
        { viewId: VIEW_ID, resizableColumnIds: RESIZABLE_IDS, fixedColumnsWidth: FIXED_WIDTH, defaultSizes: {} },
        containerWidth,
      )
      if (restored) {
        columnSizing = restored
        currentRatios = toRatios(restored, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
        return
      }
      const flexCols = columns.filter(c => (c.meta as { flex?: boolean })?.flex)
      const flexDefined = flexCols.reduce((sum, c) => sum + (c.size || 150), 0)
      const available = Math.max(0, containerWidth - FIXED_WIDTH)
      const newSizing: Record<string, number> = {}
      for (const col of flexCols) {
        const id = col.id || (col as { accessorKey?: string }).accessorKey || ''
        newSizing[id] = Math.round(((col.size || 150) / flexDefined) * available)
      }
      columnSizing = newSizing
      currentRatios = toRatios(newSizing, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
    })
  }
})

function handleResizeEnd() {
  if (!scrollElement) return
  const containerWidth = scrollElement.clientWidth - CONTAINER_PADDING
  currentRatios = toRatios(columnSizing, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
  saveColumnWidths(VIEW_ID, columnSizing, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
}

function handleWindowResize() {
  if (!scrollElement || !widthsLocked || Object.keys(currentRatios).length === 0) return
  const containerWidth = scrollElement.clientWidth - CONTAINER_PADDING
  columnSizing = recalcFromRatios(currentRatios, FIXED_WIDTH, containerWidth)
}
```

- [ ] **Step 6: リセットイベントリスナーを追加**

`onMount` importに `onDestroy` を追加:

```typescript
import { onMount, afterUpdate, onDestroy, createEventDispatcher } from 'svelte'
```

`EventsOn` をimport:

```typescript
import { EventsOn } from '../../wailsjs/runtime/runtime'
```

`onMount` ブロック内の末尾に追加:

```typescript
const offResetWidths = EventsOn('column-width-reset', (id: string) => {
  if (id !== VIEW_ID) return
  columnSizing = {}
  currentRatios = {}
  widthsLocked = false
})
```

`onMount` の後に `onDestroy` を追加:

```typescript
onDestroy(() => {
  offResetWidths?.()
})
```

- [ ] **Step 7: テンプレートを修正**

`<svelte:window on:keydown={handleKeyNav} />`（170行目）を修正:

```svelte
<svelte:window on:keydown={handleKeyNav} on:resize={handleWindowResize} />
```

`<SortableHeader table={$table} />`（208行目）を修正:

```svelte
<SortableHeader table={$table} onResizeEnd={handleResizeEnd} />
```

セルのstyle属性（227行目）を修正。`resizeLocked`相当の判定を追加:

```svelte
style={widthsLocked || !cell.column.columnDef.meta?.flex ? `flex: 0 0 ${cell.column.getSize()}px` : `flex: 1 1 ${cell.column.getSize()}px; min-width: ${cell.column.getSize()}px`}
```

（SortableHeader内部の`resizeLocked`はヘッダー側で自動的に効くため、ボディ側のstyleのみ修正が必要）

- [ ] **Step 8: コミット**

```bash
git add frontend/src/views/ChartListView.svelte
git commit -m "feat: ChartListViewにカラムリサイズ・永続化を追加"
```

---

## Task 6: SongListViewにカラムリサイズを追加

**Files:**
- Modify: `frontend/src/views/SongListView.svelte`

- [ ] **Step 1: import追加**

`SongListView.svelte` の `@tanstack/svelte-table` importに `ColumnSizingState`, `ColumnSizingInfoState` を追加:

```typescript
import {
  createSvelteTable,
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  getFacetedRowModel,
  getFacetedUniqueValues,
  type ColumnDef,
  type SortingState,
  type FilterFn,
  type ColumnSizingState,
  type ColumnSizingInfoState,
} from '@tanstack/svelte-table'
```

`SortableHeader` importの後に追加:

```typescript
import {
  loadColumnWidths,
  saveColumnWidths,
  recalcFromRatios,
  toRatios,
  type ViewId,
} from '../utils/columnResize'
```

- [ ] **Step 2: `ir`カラムにenableResizing: falseを設定**

`columns` 定義（60-99行目）の `ir` カラム（92-98行目）を修正:

```typescript
{
  id: 'ir',
  header: 'IR',
  size: 40,
  meta: { align: 'center' },
  enableResizing: false,
  accessorFn: (row) => row.hasIrMeta ? '●' : '',
},
```

- [ ] **Step 3: リサイズ状態管理の変数を追加**

`let sorting: SortingState = []`（101行目）の後に追加:

```typescript
const VIEW_ID: ViewId = 'songList'
const RESIZABLE_IDS = columns
  .filter(c => c.enableResizing !== false)
  .filter(c => c.id !== 'ir')
  .map(c => c.id || (c as { accessorKey?: string }).accessorKey || '')
const FIXED_WIDTH = columns
  .filter(c => c.enableResizing === false)
  .reduce((sum, c) => sum + (c.size || 150), 0)
const CONTAINER_PADDING = 16

let columnSizing: ColumnSizingState = {}
let columnSizingInfo: ColumnSizingInfoState = {} as ColumnSizingInfoState
let widthsLocked = false
let currentRatios: Record<string, number> = {}
```

- [ ] **Step 4: createSvelteTableにリサイズオプションを追加**

`$: table = createSvelteTable({...})` （112-125行目）を修正:

```typescript
$: table = createSvelteTable({
  data: songs,
  columns,
  enableColumnResizing: true,
  columnResizeMode: 'onChange',
  state: { sorting, globalFilter, columnSizing, columnSizingInfo },
  onSortingChange: (updater) => {
    sorting = typeof updater === 'function' ? updater(sorting) : updater
  },
  onColumnSizingChange: (updater) => {
    columnSizing = typeof updater === 'function' ? updater(columnSizing) : updater
  },
  onColumnSizingInfoChange: (updater) => {
    columnSizingInfo = typeof updater === 'function' ? updater(columnSizingInfo) : updater
  },
  globalFilterFn: searchFilter,
  getCoreRowModel: getCoreRowModel(),
  getFilteredRowModel: getFilteredRowModel(),
  getSortedRowModel: getSortedRowModel(),
  getFacetedRowModel: getFacetedRowModel(),
  getFacetedUniqueValues: getFacetedUniqueValues(),
})
```

- [ ] **Step 5: 初期化・永続化・ウィンドウリサイズのロジックを追加**

`onMount, onDestroy` importに `afterUpdate` を追加:

```typescript
import { onMount, onDestroy, afterUpdate, createEventDispatcher } from 'svelte'
```

`onDestroy` ブロック（209-213行目）の後に追加:

```typescript
afterUpdate(() => {
  if (songs.length > 0 && scrollElement && !widthsLocked) {
    widthsLocked = true
    requestAnimationFrame(async () => {
      const containerWidth = scrollElement.clientWidth - CONTAINER_PADDING
      const restored = await loadColumnWidths(
        { viewId: VIEW_ID, resizableColumnIds: RESIZABLE_IDS, fixedColumnsWidth: FIXED_WIDTH, defaultSizes: {} },
        containerWidth,
      )
      if (restored) {
        columnSizing = restored
        currentRatios = toRatios(restored, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
        return
      }
      const flexCols = columns.filter(c => (c.meta as { flex?: boolean })?.flex)
      const flexDefined = flexCols.reduce((sum, c) => sum + (c.size || 150), 0)
      const available = Math.max(0, containerWidth - FIXED_WIDTH)
      const newSizing: Record<string, number> = {}
      for (const col of flexCols) {
        const id = col.id || (col as { accessorKey?: string }).accessorKey || ''
        newSizing[id] = Math.round(((col.size || 150) / flexDefined) * available)
      }
      columnSizing = newSizing
      currentRatios = toRatios(newSizing, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
    })
  }
})

function handleResizeEnd() {
  if (!scrollElement) return
  const containerWidth = scrollElement.clientWidth - CONTAINER_PADDING
  currentRatios = toRatios(columnSizing, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
  saveColumnWidths(VIEW_ID, columnSizing, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
}

function handleWindowResize() {
  if (!scrollElement || !widthsLocked || Object.keys(currentRatios).length === 0) return
  const containerWidth = scrollElement.clientWidth - CONTAINER_PADDING
  columnSizing = recalcFromRatios(currentRatios, FIXED_WIDTH, containerWidth)
}
```

- [ ] **Step 6: リセットイベントリスナーを追加**

`onMount` ブロック内（`offSyncDone` 登録の後）に追加:

```typescript
const offResetWidths = EventsOn('column-width-reset', (id: string) => {
  if (id !== VIEW_ID) return
  columnSizing = {}
  currentRatios = {}
  widthsLocked = false
})
```

`onDestroy` ブロック内に追加:

```typescript
offResetWidths?.()
```

- [ ] **Step 7: テンプレートを修正**

`<svelte:window on:keydown={handleKeyNav} />`（216行目）を修正:

```svelte
<svelte:window on:keydown={handleKeyNav} on:resize={handleWindowResize} />
```

`<SortableHeader table={$table} />`（258行目）を修正:

```svelte
<SortableHeader table={$table} onResizeEnd={handleResizeEnd} />
```

セルのstyle属性（283行目）を修正:

```svelte
style={widthsLocked || !cell.column.columnDef.meta?.flex ? `flex: 0 0 ${cell.column.getSize()}px` : `flex: 1 1 ${cell.column.getSize()}px; min-width: ${cell.column.getSize()}px`}
```

- [ ] **Step 8: コミット**

```bash
git add frontend/src/views/SongListView.svelte
git commit -m "feat: SongListViewにカラムリサイズ・永続化を追加"
```

---

## Task 7: DifficultyTableViewにカラムリサイズを追加

**Files:**
- Modify: `frontend/src/views/DifficultyTableView.svelte`

DifficultyTableViewは他のビューと異なり、`writable` storeベースの`options`パターンでテーブルを管理している。この既存パターンに合わせてリサイズを組み込む。

- [ ] **Step 1: import追加**

`@tanstack/svelte-table` importに `ColumnSizingState`, `ColumnSizingInfoState` を追加:

```typescript
import {
  createSvelteTable,
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  getFilteredRowModel,
  type ColumnDef,
  type SortingState,
  type TableOptions,
  type ColumnSizingState,
  type ColumnSizingInfoState,
} from '@tanstack/svelte-table'
```

`SortableHeader` importの後に追加:

```typescript
import {
  loadColumnWidths,
  saveColumnWidths,
  recalcFromRatios,
  toRatios,
  type ViewId,
} from '../utils/columnResize'
```

- [ ] **Step 2: `level`と`statusLabel`カラムにenableResizing: falseを設定**

`columns` 定義（46-81行目）の `level` カラム（47-57行目）を修正:

```typescript
{
  accessorKey: 'level',
  header: 'Level',
  size: 80,
  meta: { align: 'right' },
  enableResizing: false,
  sortingFn: (rowA, rowB, columnId) => {
    const a = Number(rowA.getValue(columnId)) || 0
    const b = Number(rowB.getValue(columnId)) || 0
    return a - b
  },
},
```

`statusLabel` カラム（68-80行目）を修正:

```typescript
{
  id: 'statusLabel',
  header: 'Status',
  size: 100,
  enableResizing: false,
  accessorFn: (row) => {
    if (row.status === 'installed') return '導入済'
    if (row.status === 'not_installed') return '未導入'
    if (row.status === 'duplicate') return '重複'
    return row.status
  },
  enableSorting: false,
  filterFn: 'equalsString',
  meta: { filterType: 'select', filterOptions: ['導入済', '未導入', '重複'] },
},
```

- [ ] **Step 3: リサイズ状態管理の変数を追加**

`let sorting: SortingState = []`（83行目）の後に追加:

```typescript
const VIEW_ID: ViewId = 'difficultyTable'
const RESIZABLE_IDS = columns
  .filter(c => c.enableResizing !== false)
  .filter(c => {
    const key = (c as { accessorKey?: string }).accessorKey || c.id || ''
    return key !== 'level' && key !== 'statusLabel'
  })
  .map(c => c.id || (c as { accessorKey?: string }).accessorKey || '')
const FIXED_WIDTH = columns
  .filter(c => c.enableResizing === false)
  .reduce((sum, c) => sum + (c.size || 150), 0)
const CONTAINER_PADDING = 16

let columnSizing: ColumnSizingState = {}
let columnSizingInfo: ColumnSizingInfoState = {} as ColumnSizingInfoState
let widthsLocked = false
let currentRatios: Record<string, number> = {}
```

- [ ] **Step 4: options storeにリサイズオプションを追加**

`const options = writable<TableOptions<...>>({...})` （85-100行目）を修正:

```typescript
const options = writable<TableOptions<dto.DifficultyTableEntryDTO>>({
  data: entries,
  columns,
  enableColumnResizing: true,
  columnResizeMode: 'onChange',
  state: { sorting, columnSizing, columnSizingInfo },
  onSortingChange: (updater) => {
    if (typeof updater === 'function') {
      sorting = updater(sorting)
    } else {
      sorting = updater
    }
    options.update((o) => ({ ...o, state: { ...o.state, sorting } }))
  },
  onColumnSizingChange: (updater) => {
    columnSizing = typeof updater === 'function' ? updater(columnSizing) : updater
    options.update((o) => ({ ...o, state: { ...o.state, columnSizing } }))
  },
  onColumnSizingInfoChange: (updater) => {
    columnSizingInfo = typeof updater === 'function' ? updater(columnSizingInfo) : updater
    options.update((o) => ({ ...o, state: { ...o.state, columnSizingInfo } }))
  },
  getCoreRowModel: getCoreRowModel(),
  getSortedRowModel: getSortedRowModel(),
  getFilteredRowModel: getFilteredRowModel(),
})
```

- [ ] **Step 5: applyFilter内でcolumnSizing stateも維持する**

`applyFilter` 関数（146-154行目）を修正:

```typescript
function applyFilter() {
  const filtered = searchText
    ? entries.filter(e => {
        const s = searchText.toLowerCase()
        return e.title.toLowerCase().includes(s) || e.artist.toLowerCase().includes(s)
      })
    : entries
  options.update((o) => ({ ...o, data: filtered, state: { ...o.state, columnSizing, columnSizingInfo } }))
}
```

- [ ] **Step 6: 初期化・永続化・ウィンドウリサイズのロジックを追加**

`onMount` importに `afterUpdate` を追加:

```typescript
import { onMount, afterUpdate, createEventDispatcher } from 'svelte'
```

`handleRowClick` 関数の後（188行目付近）に追加:

```typescript
afterUpdate(() => {
  if (entries.length > 0 && scrollElement && !widthsLocked) {
    widthsLocked = true
    requestAnimationFrame(async () => {
      const containerWidth = scrollElement.clientWidth - CONTAINER_PADDING
      const restored = await loadColumnWidths(
        { viewId: VIEW_ID, resizableColumnIds: RESIZABLE_IDS, fixedColumnsWidth: FIXED_WIDTH, defaultSizes: {} },
        containerWidth,
      )
      if (restored) {
        columnSizing = restored
        currentRatios = toRatios(restored, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
        options.update((o) => ({ ...o, state: { ...o.state, columnSizing } }))
        return
      }
      const flexCols = columns.filter(c => (c.meta as { flex?: boolean })?.flex)
      const flexDefined = flexCols.reduce((sum, c) => sum + (c.size || 150), 0)
      const available = Math.max(0, containerWidth - FIXED_WIDTH)
      const newSizing: Record<string, number> = {}
      for (const col of flexCols) {
        const id = col.id || (col as { accessorKey?: string }).accessorKey || ''
        newSizing[id] = Math.round(((col.size || 150) / flexDefined) * available)
      }
      columnSizing = newSizing
      currentRatios = toRatios(newSizing, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
      options.update((o) => ({ ...o, state: { ...o.state, columnSizing } }))
    })
  }
})

function handleResizeEnd() {
  if (!scrollElement) return
  const containerWidth = scrollElement.clientWidth - CONTAINER_PADDING
  currentRatios = toRatios(columnSizing, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
  saveColumnWidths(VIEW_ID, columnSizing, RESIZABLE_IDS, FIXED_WIDTH, containerWidth)
}

function handleWindowResize() {
  if (!scrollElement || !widthsLocked || Object.keys(currentRatios).length === 0) return
  const containerWidth = scrollElement.clientWidth - CONTAINER_PADDING
  columnSizing = recalcFromRatios(currentRatios, FIXED_WIDTH, containerWidth)
  options.update((o) => ({ ...o, state: { ...o.state, columnSizing } }))
}
```

- [ ] **Step 7: リセットイベントリスナーを追加**

`EventsOn` をimport（DifficultyTableViewには未import）:

```typescript
import { EventsOn } from '../../wailsjs/runtime/runtime'
```

`onMount` importに `onDestroy` を追加:

```typescript
import { onMount, afterUpdate, onDestroy, createEventDispatcher } from 'svelte'
```

`onMount` ブロック内の末尾に追加:

```typescript
const offResetWidths = EventsOn('column-width-reset', (id: string) => {
  if (id !== VIEW_ID) return
  columnSizing = {}
  currentRatios = {}
  widthsLocked = false
  options.update((o) => ({ ...o, state: { ...o.state, columnSizing: {} } }))
})
```

`onMount` の後に `onDestroy` を追加:

```typescript
onDestroy(() => {
  offResetWidths?.()
})
```

- [ ] **Step 8: テンプレートを修正**

`<svelte:window on:keydown={handleKeyNav} />`（233行目）を修正:

```svelte
<svelte:window on:keydown={handleKeyNav} on:resize={handleWindowResize} />
```

`<SortableHeader table={$table} />`（289行目）を修正:

```svelte
<SortableHeader table={$table} onResizeEnd={handleResizeEnd} />
```

セルのstyle属性（322行目）を修正:

```svelte
style={widthsLocked || !cell.column.columnDef.meta?.flex ? `flex: 0 0 ${cell.column.getSize()}px` : `flex: 1 1 ${cell.column.getSize()}px; min-width: ${cell.column.getSize()}px`}
```

- [ ] **Step 9: コミット**

```bash
git add frontend/src/views/DifficultyTableView.svelte
git commit -m "feat: DifficultyTableViewにカラムリサイズ・永続化を追加"
```

---

## Task 8: Settingsモーダルにカラム幅リセットを追加

**Files:**
- Modify: `frontend/src/settings/Settings.svelte`

- [ ] **Step 1: リセット関数を追加**

`Settings.svelte` のimportセクションに `EventsEmit` を追加:

```typescript
import { EventsOn, EventsEmit } from '../../wailsjs/runtime/runtime'
```

`handleClose` 関数（97-99行目）の後に追加:

```typescript
async function resetColumnWidths(viewId: string) {
  try {
    const cfg = await GetConfig()
    if (cfg.columnWidths) {
      const columnWidths = { ...cfg.columnWidths }
      delete columnWidths[viewId]
      await SaveConfig({ ...cfg, columnWidths })
    }
    // ビュー側のメモリ状態をクリアさせるイベントを発火
    EventsEmit('column-width-reset', viewId)
  } catch (e: any) {
    error = e?.message || 'リセットに失敗しました'
  }
}
```

- [ ] **Step 2: UIを追加**

「ファイル別ログを出力」チェックボックス（213-223行目）と「バックグラウンドタスク」divider（226行目）の間に、カラム幅リセットセクションを追加:

```svelte
<div class="divider text-xs text-base-content/50">カラム幅</div>

<div class="space-y-2">
  <p class="text-sm text-base-content/70">各テーブルのカラム幅をデフォルトに戻します</p>
  <div class="flex flex-wrap gap-2">
    <button class="btn btn-xs btn-outline" on:click={() => resetColumnWidths('chartList')}>楽曲一覧</button>
    <button class="btn btn-xs btn-outline" on:click={() => resetColumnWidths('songList')}>譜面一覧</button>
    <button class="btn btn-xs btn-outline" on:click={() => resetColumnWidths('difficultyTable')}>難易度表</button>
    <button class="btn btn-xs btn-outline" on:click={() => resetColumnWidths('diffImport')}>差分導入</button>
  </div>
</div>
```

- [ ] **Step 3: コミット**

```bash
git add frontend/src/settings/Settings.svelte
git commit -m "feat: Settingsモーダルにカラム幅リセットボタンを追加"
```

---

## Task 9: Wailsバインディング再生成とビルド確認

**Files:**
- 自動生成: `frontend/wailsjs/go/models.ts`

- [ ] **Step 1: Wailsバインディングを再生成**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails generate module`
Expected: `frontend/wailsjs/go/models.ts` の `Config` クラスに `columnWidths` フィールドが追加される

もし `wails generate module` が使えない場合、`wails build` でもバインディングは再生成される。

- [ ] **Step 2: 生成されたmodels.tsを確認**

`frontend/wailsjs/go/models.ts` の `Config` クラスに `columnWidths` フィールドがあることを確認。

- [ ] **Step 3: フロントエンドビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run build`
Expected: エラーなし

- [ ] **Step 4: Goビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: エラーなし

- [ ] **Step 5: 生成ファイルをコミット**

```bash
git add frontend/wailsjs/go/models.ts
git commit -m "chore: Wailsバインディング再生成（Config.columnWidths追加）"
```

---

## Task 10: 動作確認

- [ ] **Step 1: `wails dev` で起動**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails dev`

- [ ] **Step 2: 各ビューの確認項目**

以下を各ビュー（楽曲一覧、譜面一覧、難易度表、差分導入）で確認:

1. カラムヘッダー間のリサイズハンドルが表示される
2. ドラッグでカラム幅を変更できる
3. リサイズ不可カラム（ir, level, statusLabel, score, matchMethod, actions）のハンドルは非表示
4. リサイズ後にアプリを再起動しても幅が維持されている
5. ウィンドウサイズを変更するとカラム幅が比例して再計算される
6. Settingsモーダルの「カラム幅をリセット」ボタンが機能する

- [ ] **Step 3: config.jsonを直接確認**

アプリディレクトリの`config.json`を開き、`columnWidths`フィールドに各ビューの割合値が保存されていることを確認。
