# 難易度表ビュー Level カラム フィルタ化 設計

- 作成日: 2026-05-05
- ブランチ: `feat/difficulty-table-level-filter`

## 1. 目的とスコープ

難易度表ビューの Level カラムのヘッダークリック動作を **ソート → フィルタ** に変更する。Status カラムと同様のセレクト型フィルタ（プルダウン）を適用し、選択した level に該当するエントリのみ表示できるようにする。

### スコープ内

- `frontend/src/views/DifficultyTableView.svelte` の Level カラム定義変更
- `frontend/src/components/SortableHeader.svelte` の `filterSort` モード拡張
- `frontend/src/vite-env.d.ts` の型定義追加
- 表切替時の Level フィルタ値リセット

### スコープ外（変更しない）

- バックエンド（Go コード一切）
- DB マイグレーション、スキーマ変更
- フェッチャ（`difficulty_table_fetcher.go`）の変更
- DTO、Wails 自動生成バインディング
- 他のビュー（SongListView、ChartListView 等）の挙動
- 検索テキスト（`searchText`）の挙動
- 行のデフォルト並び順そのもの（既存の SQL ORDER BY を維持）

## 2. 要件まとめ

| 項目 | 決定 |
|---|---|
| Level ヘッダーのクリック動作 | フィルタプルダウンを開く（ソートは無効化） |
| フィルタプルダウンの並び順 | 数値昇順 → 非数値（辞書順） |
| プルダウン項目の供給元 | `getFacetedUniqueValues()`（テーブルに渡された data の level 値の集合。検索ボックスのフィルタ後に再計算される） |
| 表切替時の Level フィルタ値 | クリアする |
| Level カラムでのリサイズ | 既存通り無効（`enableResizing: false`） |
| ソートインジケータ（▲▼）の消失 | 許容（Status カラムと同等の見た目になる） |

## 3. 並び順の根拠

### 行のデフォルト並び順

`internal/adapter/persistence/difficulty_table_repository.go:167-191` の `ListEntries` が以下の SQL ORDER BY で行を返す：

```sql
ORDER BY
    CASE WHEN CAST(level AS INTEGER) = 0 AND level != '0' THEN 1 ELSE 0 END,
    CAST(level AS INTEGER),
    level,
    title, artist
```

これにより「数値 level 昇順 → 非数値 level（`???` 等）」の順で行が並ぶ。Level カラムを `enableSorting: false` にすることでフロント側ソートが発生せず、SQL の並び順がそのまま表示される。

### フィルタプルダウンの並び順

新規導入する `filterSort: 'numericFirst'` モードで以下のロジックを適用する：

- 両方が数値解釈可能 → 数値昇順
- 片方のみ数値 → 数値が先
- 両方が非数値 → `localeCompare` の昇順

これは行の並び順と一致する。

## 4. 実装変更

### 4-1. `frontend/src/views/DifficultyTableView.svelte`

**インポート追加**

```ts
import {
  // ...
  getFacetedUniqueValues,  // 追加
} from '@tanstack/svelte-table'
```

**Level カラム定義の変更**

```ts
// 変更前
{
  accessorKey: 'level',
  header: 'Level',
  size: 70,
  meta: { align: 'right' },
  enableResizing: false,
  sortingFn: (rowA, rowB, columnId) => {
    const a = Number(rowA.getValue(columnId)) || 0
    const b = Number(rowB.getValue(columnId)) || 0
    return a - b
  },
},

// 変更後
{
  accessorKey: 'level',
  header: 'Level',
  size: 70,
  meta: { align: 'right', filterType: 'select', filterSort: 'numericFirst' },
  enableResizing: false,
  enableSorting: false,
  filterFn: 'equalsString',
},
```

**table options に `getFacetedUniqueValues` を追加**

```ts
const options = writable<TableOptions<dto.DifficultyTableEntryDTO>>({
  // 既存設定
  getCoreRowModel: getCoreRowModel(),
  getSortedRowModel: getSortedRowModel(),
  getFilteredRowModel: getFilteredRowModel(),
  getFacetedUniqueValues: getFacetedUniqueValues(),  // 追加
})
```

**`handleTableChange` で Level フィルタをクリア**

```ts
async function handleTableChange(e: Event) {
  const target = e.target as HTMLSelectElement
  const id = Number(target.value)
  selectedTableId = id
  $table.getColumn('level')?.setFilterValue(undefined)  // 追加
  dispatch('deselect')
  await loadEntries(id)
}
```

### 4-2. `frontend/src/components/SortableHeader.svelte`

`getFilterOptions` の `filterSort` 分岐に `'numericFirst'` を追加。

```ts
function getFilterOptions(column: Column<any, unknown>): string[] {
  const meta = column.columnDef.meta as { filterOptions?: string[]; filterSort?: 'asc' | 'desc' | 'numericFirst' } | undefined
  if (meta?.filterOptions) return meta.filterOptions
  try {
    const values = column.getFacetedUniqueValues()
    const opts = Array.from(values.keys())
      .filter((v) => v != null && v !== '')
      .map(String)
    if (meta?.filterSort === 'numericFirst') {
      return opts.sort((a, b) => {
        const an = Number(a)
        const bn = Number(b)
        const aIsNum = !isNaN(an) && a.trim() !== ''
        const bIsNum = !isNaN(bn) && b.trim() !== ''
        if (aIsNum && bIsNum) return an - bn
        if (aIsNum) return -1
        if (bIsNum) return 1
        return a.localeCompare(b)
      })
    }
    return meta?.filterSort === 'desc' ? opts.sort().reverse() : opts.sort()
  } catch {
    return []
  }
}
```

### 4-3. `frontend/src/vite-env.d.ts`

`filterSort` の型を拡張：

```ts
// 変更前
filterSort?: 'asc' | 'desc'

// 変更後
filterSort?: 'asc' | 'desc' | 'numericFirst'
```

## 5. 動作確認項目

1. Level ヘッダーをクリックするとプルダウンが開く（▲▼ は表示されない）
2. プルダウン項目が「1, 2, 3, ..., 25, ???」順に並ぶ（発狂BMS難易度表で確認）
3. プルダウンから「12」等を選択すると、その level のエントリのみ表示される
4. プルダウンの「すべて」をクリックするとフィルタ解除される
5. フィルタ適用中は Level ヘッダー右側にフィルタアイコンが表示される
6. 表セレクタで別の難易度表に切り替えると、Level フィルタの選択値がクリアされる
7. Level カラムの幅が変化しない（`enableResizing: false` 維持）
8. 他カラム（Title, Artist, URL）のソートやリサイズは従来通り動作する

## 6. 想定リスクと対応

| リスク | 対応 |
|---|---|
| `getFacetedUniqueValues()` 未追加によりプルダウンが空になる | options に明示的に追加（4-1） |
| 表切替時に前の表のフィルタ値が残り全件除外される | `handleTableChange` でクリア（4-1） |
| `filterSort: 'numericFirst'` が他の場所で誤使用される | 当面 Level カラムのみで使用。型定義で許容値を制限 |
| `localeCompare` の locale 依存で並びがブラウザ環境で揺れる | level の非数値値は実用上 `???` 程度。将来必要なら `'en'` 固定化検討 |

## 7. テスト

- 既存テスト（`difficulty_table_repository_test.go` 等）への影響なし
- TypeScript 型チェックでカラム定義の整合性が検証される
- 手動確認は §5 のチェックリストで実施
