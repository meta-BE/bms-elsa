# カラムフィルタ設計

## 目的

楽曲一覧・譜面一覧のEVENT/YEARカラム、難易度表のSTATUSカラムを、ソートからドロップダウンフィルタに変更する。

## アプローチ

TanStack Tableのカラムフィルタ機能（`column.setFilterValue()`）を使用。

## 変更対象

### 新規: FilterHeader.svelte

ドロップダウン `<select>` 付きヘッダーコンポーネント。

- Props: `header`（TanStack Tableのヘッダー）、`options`（選択肢の配列）
- 初期値「すべて」でフィルタ解除
- 選択時に `column.setFilterValue(value)` で即時フィルタ

### SongTable.svelte

- EVENT/YEARカラムに `enableSorting: false`、`filterFn` 設定
- ヘッダーをSortableHeaderからFilterHeaderに変更
- 選択肢はデータから動的抽出（重複排除・ソート済み）

### ChartListView.svelte

- SongTableと同様の変更

### DifficultyTableView.svelte

- STATUSカラムに `enableSorting: false`、`filterFn` 設定
- 選択肢は固定3値: 導入済 / 未導入 / 重複

## データフロー

```
ドロップダウン選択 → column.setFilterValue(value)
  → TanStack Table getFilteredRowModel() 再計算
  → テーブル表示更新
```

グローバル検索フィルタとの併用可（TanStack Tableが両方を合成）。
