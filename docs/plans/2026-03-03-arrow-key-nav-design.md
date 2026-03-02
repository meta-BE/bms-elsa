# テーブル上下キーナビゲーション + EntryDetail自己完結化

## ゴール

3つのテーブル（楽曲一覧・譜面一覧・難易度表）で、詳細表示中に上下矢印キーで隣のレコードに移動できるようにする。あわせて、DifficultyTableViewのEntryDetailをSongDetail/ChartDetailと同じ「IDだけ受け取って自分でデータ取得」パターンに統一する。

## 設計

### 1. バックエンド: GetDifficultyTableEntry API追加

`app.go` に `GetDifficultyTableEntry(tableID int, md5 string) (*DifficultyTableEntryDTO, error)` を追加。既存の `ListDifficultyTableEntries` と同じロジック（songdataからinstalledCount取得→status計算）を単一エントリ版として実装。

### 2. EntryDetail自己完結化

- props: `md5: string`, `tableID: number`（`entryData` を廃止）
- `$: if (md5 && tableID) loadEntry(md5, tableID)` で自動取得
- 既存のchart/irMetaフェッチロジックはそのまま

### 3. DifficultyTableView dispatch簡素化

- Before: `dispatch('select', { md5: entry.md5, entry })`
- After: `dispatch('select', { md5: entry.md5, tableID: selectedTableId })`

### 4. App.svelte

- `selectedEntryData` → `selectedTableID: number | null` に変更
- `EntryDetail md5={selectedEntryMD5} tableID={selectedTableID}`

### 5. 上下キーナビゲーション

**共通ヘルパー** `frontend/src/utils/arrowNav.ts`:

```typescript
export function handleArrowNav(e: KeyboardEvent, opts: {
  selected: string | null,
  rows: Row<any>[],
  getKey: (original: any) => string,
  onSelect: (original: any, index: number) => void,
  scrollToIndex: (index: number) => void,
}): void
```

- ArrowUp/ArrowDown以外 → 無視
- `selected` が null → 無視
- `document.activeElement` が input/textarea/select/[contenteditable] → 無視
- rows内で現在のselectedのindexを特定、上下に移動（範囲クランプ）
- `onSelect` で新しい行を選択、`scrollToIndex` で画面内にスクロール
- `e.preventDefault()` でページスクロール防止

**各テーブルでの使用:**
- `svelte:window on:keydown` でハンドラを呼び出し
- `getKey` と `onSelect` だけテーブルごとにカスタマイズ
