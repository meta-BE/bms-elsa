# DuplicateView キーボードナビゲーション 実装計画

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** arrowNav.ts を汎用化し、DuplicateView に上下キーによるグループ選択移動を追加する。

**Architecture:** 既存の `handleArrowNav` から tanstack-table 依存を除去し、素の配列でも動く汎用インターフェースに変更。既存3ビュー（SongTable, ChartListView, DifficultyTableView）の呼び出しを新インターフェースに合わせて調整。DuplicateView に `svelte:window on:keydown` でキーナビを追加。

**Tech Stack:** Svelte 4, TypeScript

---

## Chunk 1: arrowNav 汎用化 + 既存ビュー調整 + DuplicateView 実装

### Task 1: arrowNav.ts を汎用化

**Files:**
- Modify: `frontend/src/utils/arrowNav.ts`

- [ ] **Step 1: インターフェースを変更**

tanstack の `Row<any>[]` 依存を除去し、素の配列で動くようにする。`scrollToIndex` はオプショナルに変更:

```typescript
export function handleArrowNav<T>(e: KeyboardEvent, opts: {
  selected: string | null,
  items: T[],
  getKey: (item: T) => string,
  onSelect: (item: T, index: number) => void,
  scrollToIndex?: (index: number) => void,
}): void {
  if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return
  if (!opts.selected) return

  const el = document.activeElement
  if (el) {
    const tag = el.tagName.toLowerCase()
    if (tag === 'input' || tag === 'textarea' || tag === 'select' || el.hasAttribute('contenteditable')) return
  }

  const currentIndex = opts.items.findIndex(item => opts.getKey(item) === opts.selected)
  if (currentIndex === -1) return

  const nextIndex = e.key === 'ArrowUp'
    ? Math.max(0, currentIndex - 1)
    : Math.min(opts.items.length - 1, currentIndex + 1)

  if (nextIndex === currentIndex) return

  e.preventDefault()
  if (el instanceof HTMLElement) el.blur()
  opts.onSelect(opts.items[nextIndex], nextIndex)
  opts.scrollToIndex?.(nextIndex)
}
```

変更ポイント:
- `rows: Row<any>[]` → `items: T[]`（ジェネリクス化）
- `r.original` 経由のアクセスを廃止、直接 `item` を参照
- `scrollToIndex` をオプショナル（`?`）に
- `import type { Row }` を削除

- [ ] **Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npx svelte-check`
Expected: arrowNav.ts 自体はエラーなし（呼び出し側で型エラーが出る想定）

- [ ] **Step 3: コミット**

```bash
git add frontend/src/utils/arrowNav.ts
git commit -m "refactor: arrowNav を汎用化し tanstack-table 依存を除去"
```

---

### Task 2: 既存3ビューの呼び出しを新インターフェースに合わせる

**Files:**
- Modify: `frontend/src/views/SongTable.svelte:134-140`
- Modify: `frontend/src/views/ChartListView.svelte:206-212`
- Modify: `frontend/src/views/DifficultyTableView.svelte:168-174`

- [ ] **Step 1: SongTable.svelte を修正**

134-140行目の `handleArrowNav` 呼び出しを変更:

```typescript
    handleArrowNav(e, {
      selected,
      items: rows.map(r => r.original),
      getKey: (o: dto.SongRowDTO) => o.folderHash,
      onSelect: (o: dto.SongRowDTO) => dispatch('select', o.folderHash),
      scrollToIndex: (i: number) => $virtualizer.scrollToIndex(i, { align: 'auto' }),
    })
```

変更ポイント: `rows` → `items: rows.map(r => r.original)`

- [ ] **Step 2: ChartListView.svelte を修正**

206-212行目の `handleArrowNav` 呼び出しを変更:

```typescript
    handleArrowNav(e, {
      selected,
      items: rows.map(r => r.original),
      getKey: (o: dto.ChartListItemDTO) => o.md5,
      onSelect: (o: dto.ChartListItemDTO) => dispatch('select', { md5: o.md5 }),
      scrollToIndex: (i: number) => $virtualizer.scrollToIndex(i, { align: 'auto' }),
    })
```

- [ ] **Step 3: DifficultyTableView.svelte を修正**

168-174行目の `handleArrowNav` 呼び出しを変更:

```typescript
    handleArrowNav(e, {
      selected,
      items: rows.map(r => r.original),
      getKey: (o: dto.DifficultyTableEntryDTO) => o.md5,
      onSelect: (o: dto.DifficultyTableEntryDTO) => dispatch('select', { md5: o.md5, tableID: selectedTableId! }),
      scrollToIndex: (i: number) => $virtualizer.scrollToIndex(i, { align: 'auto' }),
    })
```

- [ ] **Step 4: Svelte チェック**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npx svelte-check`
Expected: エラーなし

- [ ] **Step 5: コミット**

```bash
git add frontend/src/views/SongTable.svelte frontend/src/views/ChartListView.svelte frontend/src/views/DifficultyTableView.svelte
git commit -m "refactor: 既存ビューを arrowNav 新インターフェースに合わせて調整"
```

---

### Task 3: DuplicateView にキーボードナビゲーションを追加

**Files:**
- Modify: `frontend/src/views/DuplicateView.svelte`

- [ ] **Step 1: import を追加**

script 冒頭に追加:

```typescript
import { handleArrowNav } from '../utils/arrowNav'
```

- [ ] **Step 2: handleKeyNav 関数を追加**

`handleSelect` 関数の後に追加:

```typescript
  function handleKeyNav(e: KeyboardEvent) {
    if (!active || !scanned) return
    handleArrowNav(e, {
      selected: selectedGroupID !== null ? String(selectedGroupID) : null,
      items: groups,
      getKey: (g: similarity.DuplicateGroup) => String(g.ID),
      onSelect: (g: similarity.DuplicateGroup) => handleSelect(g),
    })
  }
```

ポイント:
- `active` でタブ非アクティブ時はスキップ
- `scanned` でスキャン前はスキップ
- `selectedGroupID` は `number | null` なので `String()` で変換して `getKey` と合わせる
- `scrollToIndex` は省略（仮想スクロールなし、行数が少ないため）

- [ ] **Step 3: テンプレートに svelte:window イベントを追加**

`{#if !scanned}` の直前に追加:

```svelte
<svelte:window on:keydown={handleKeyNav} />
```

- [ ] **Step 4: Svelte チェック**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npx svelte-check`
Expected: エラーなし（warnings のみ許容）

- [ ] **Step 5: コミット**

```bash
git add frontend/src/views/DuplicateView.svelte
git commit -m "feat: DuplicateView に上下キーによるキーボードナビゲーションを追加"
```
