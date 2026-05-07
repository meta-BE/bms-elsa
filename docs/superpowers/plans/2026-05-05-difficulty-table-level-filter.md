# 難易度表 Level カラムフィルタ化 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 難易度表ビューの Level カラムのヘッダークリック動作をソートからフィルタに変更する。

**Architecture:** フロントエンドの 4 ファイル（`SortableHeader.svelte`、`DifficultyTableView.svelte`、`vite-env.d.ts`、`manual.md`）のみを変更。バックエンド・DB・フェッチャ・DTO・テストには一切手を入れない。フィルタプルダウンの並び順は新規導入する `'numericFirst'` モード（数値昇順 → 非数値辞書順）で実現し、行のデフォルト並び順は既存の SQL ORDER BY をそのまま流用する。

**Tech Stack:** Svelte 4 + TypeScript + `@tanstack/svelte-table` 8.x + Vite + svelte-check（Wails アプリのフロントエンド）

**Spec:** `docs/superpowers/specs/2026-05-05-difficulty-table-level-filter-design.md`

**Branch:** `feat/difficulty-table-level-filter`（既に作成・チェックアウト済み）

**注意事項:**
- このプロジェクトのフロントエンドには **ユニットテストの基盤が存在しない**。検証は `npm run check`（svelte-check）でのコンパイル・型チェックと、`wails dev` での手動動作確認で行う。
- 各タスクは小さく分け、タスクごとにコミットする。

---

## Task 1: SortableHeader に `'numericFirst'` filterSort モードを追加

**Files:**
- Modify: `frontend/src/vite-env.d.ts`
- Modify: `frontend/src/components/SortableHeader.svelte:23-41`

**目的:** プルダウン項目を「数値昇順 → 非数値辞書順」で並べる新モードを追加。Level カラム以外でも将来再利用可能な汎用拡張として実装する。

- [ ] **Step 1: `vite-env.d.ts` の `filterSort` 型に `'numericFirst'` を追加**

`frontend/src/vite-env.d.ts` を以下の通り変更：

```ts
/// <reference types="svelte" />
/// <reference types="vite/client" />

import type { RowData } from '@tanstack/table-core'

declare module '@tanstack/table-core' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    flex?: boolean
    align?: 'left' | 'center' | 'right'
    filterType?: string
    filterSort?: 'asc' | 'desc' | 'numericFirst'
    filterOptions?: string[]
  }
}
```

変更点は `filterSort?: 'asc' | 'desc'` を `filterSort?: 'asc' | 'desc' | 'numericFirst'` に拡張する1行のみ。

- [ ] **Step 2: `SortableHeader.svelte` の `getFilterOptions` に `numericFirst` 分岐を追加**

`frontend/src/components/SortableHeader.svelte:28-41` の `getFilterOptions` 関数を以下に置き換える：

```ts
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
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

変更点は型注釈に `'numericFirst'` を含める1点と、`if (meta?.filterSort === 'numericFirst') { ... }` の分岐を `desc/asc` 分岐の前に挟む点のみ。既存の `'asc'` / `'desc'` の挙動は変更しない。

- [ ] **Step 3: 型チェックを実行**

```bash
cd frontend && npm run check
```

期待: エラー無しで完了。新規 `'numericFirst'` がどこからも使われていない時点でも、型は `'asc' | 'desc' | 'numericFirst'` のユニオンなのでエラーは出ない。

- [ ] **Step 4: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add frontend/src/vite-env.d.ts frontend/src/components/SortableHeader.svelte
git commit -m "feat: SortableHeader に numericFirst filterSort モードを追加"
```

---

## Task 2: DifficultyTableView の Level カラムをフィルタ化

**Files:**
- Modify: `frontend/src/views/DifficultyTableView.svelte:2-13` (import追加)
- Modify: `frontend/src/views/DifficultyTableView.svelte:56-68` (Level カラム定義)
- Modify: `frontend/src/views/DifficultyTableView.svelte:113-138` (table options に getFacetedUniqueValues 追加)

**目的:** Level カラムのヘッダークリックでフィルタプルダウンを開くようにする。プルダウン項目はテーブルに渡された data の level 値の集合から自動生成し、`'numericFirst'` で並べる。

- [ ] **Step 1: `getFacetedUniqueValues` のインポートを追加**

`frontend/src/views/DifficultyTableView.svelte:2-13` のインポート文を以下に変更：

```ts
  import {
    createSvelteTable,
    flexRender,
    getCoreRowModel,
    getSortedRowModel,
    getFilteredRowModel,
    getFacetedUniqueValues,
    type ColumnDef,
    type SortingState,
    type TableOptions,
    type ColumnSizingState,
    type ColumnSizingInfoState,
  } from '@tanstack/svelte-table'
```

変更点は `getFacetedUniqueValues,` の追加 1 行のみ。

- [ ] **Step 2: Level カラム定義を変更**

`frontend/src/views/DifficultyTableView.svelte:57-68` の Level カラム定義を以下に置き換える：

```ts
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

変更点：
- 旧 `meta: { align: 'right' }` を `meta: { align: 'right', filterType: 'select', filterSort: 'numericFirst' }` に拡張
- `enableSorting: false` を追加
- `filterFn: 'equalsString'` を追加
- `sortingFn: (rowA, rowB, columnId) => { ... }` のブロックを削除

- [ ] **Step 3: table options に `getFacetedUniqueValues` を追加**

`frontend/src/views/DifficultyTableView.svelte:113-138` の `options = writable<...>({...})` ブロックの末尾、`getFilteredRowModel: getFilteredRowModel(),` の直後に1行追加：

```ts
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
  })
```

- [ ] **Step 4: 型チェックを実行**

```bash
cd frontend && npm run check
```

期待: エラー無しで完了。

- [ ] **Step 5: ビルド確認**

```bash
cd frontend && npm run build
```

期待: ビルド成功。

- [ ] **Step 6: 開発サーバーを起動して動作確認**

別ターミナルで：

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
wails dev
```

Wails ウィンドウが開いたら：

1. 難易度表タブを開く
2. 「発狂BMS難易度表」を選択
3. Level ヘッダーをクリック
4. プルダウンが開き、項目が「すべて、1, 2, 3, ..., 25, ???」の順に並んでいることを確認
5. 「12」を選択 → level=12 のエントリのみ表示されることを確認
6. 「すべて」をクリック → 全エントリが復帰することを確認
7. Level ヘッダーに ▲▼（ソート矢印）が出ていないことを確認
8. フィルタ適用中は Level ヘッダー右にフィルタアイコン（漏斗）が表示されることを確認

問題があれば該当ステップに戻って修正。確認OKなら次へ。

- [ ] **Step 7: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add frontend/src/views/DifficultyTableView.svelte
git commit -m "feat: 難易度表 Level カラムをセレクトフィルタに変更"
```

---

## Task 3: 表切替時に Level フィルタをクリア

**Files:**
- Modify: `frontend/src/views/DifficultyTableView.svelte:206-212` (`handleTableChange` 関数)

**目的:** 表Aで「12」を選択した状態で表Bに切り替えたとき、表Bに「12」が無くて全件除外されてしまう問題を防ぐ。

- [ ] **Step 1: `handleTableChange` に Level フィルタクリアを追加**

`frontend/src/views/DifficultyTableView.svelte:206-212` の関数を以下に置き換える：

```ts
  async function handleTableChange(e: Event) {
    const target = e.target as HTMLSelectElement
    const id = Number(target.value)
    selectedTableId = id
    $table.getColumn('level')?.setFilterValue(undefined)
    dispatch('deselect')
    await loadEntries(id)
  }
```

変更点は `$table.getColumn('level')?.setFilterValue(undefined)` の 1 行追加のみ。`searchText` は仕様により据え置き（`docs/manual.md:65` の既存挙動「難易度表を切り替えても検索テキストは保持されます」を維持）。

- [ ] **Step 2: 型チェック**

```bash
cd frontend && npm run check
```

期待: エラー無しで完了。

- [ ] **Step 3: 開発サーバーで動作確認**

`wails dev` で起動済みなら即時反映される（HMR）。

1. 難易度表タブで「発狂BMS難易度表」を選択
2. Level ヘッダーから「12」を選択（フィルタ適用される）
3. テーブルセレクタで別の難易度表（例: Satellite）に切り替え
4. Level ヘッダーのフィルタアイコンが消えていること、全件表示になっていることを確認
5. 検索ボックスに何か入れた状態で表を切り替え → 検索テキストは保持されることを確認（既存挙動維持）

確認OKなら次へ。

- [ ] **Step 4: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add frontend/src/views/DifficultyTableView.svelte
git commit -m "feat: 難易度表切替時に Level フィルタをクリア"
```

---

## Task 4: マニュアル更新

**Files:**
- Modify: `docs/manual.md:59-65` (難易度表セクション)

**目的:** プロジェクト規約「機能追加・変更時は、該当するマニュアルのセクションも更新すること」に従い、Level カラムのフィルタ化をユーザー向けに記載する。

- [ ] **Step 1: 難易度表セクションに Level フィルタの説明を追加**

`docs/manual.md:59-65` のセクションを以下に置き換える：

```markdown
### 難易度表

BMS難易度表（Stella、発狂BMS、Solomon等）を取り込んで表示します。
難易度表の追加・削除・更新・並び替えは、難易度表タブの「難易度表設定」ボタンから行えます。
並び替えは各行左端のグリップハンドル（⠿）をドラッグして行えます。並び替え結果はセレクターの表示順に反映されます。
未導入の譜面についてもIR情報を表示できます。
LEVEL・STATUSカラムにはフィルタ機能があります。LEVELヘッダーをクリックするとプルダウンが開き、難易度を選んで絞り込めます。
難易度表を切り替えても検索テキストは保持されます（LEVELフィルタは切替時にクリアされます）。
```

変更点：
- 「LEVEL・STATUSカラムにはフィルタ機能があります。LEVELヘッダーをクリックするとプルダウンが開き、難易度を選んで絞り込めます。」の1行を追加
- 「難易度表を切り替えても検索テキストは保持されます。」の末尾に「（LEVELフィルタは切替時にクリアされます）」を追加

- [ ] **Step 2: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add docs/manual.md
git commit -m "docs: マニュアルに難易度表 LEVEL フィルタを記載"
```

---

## Task 5: 最終手動確認

**Files:** （変更なし。`docs/superpowers/specs/2026-05-05-difficulty-table-level-filter-design.md` §5 のチェックリストに沿って確認）

**目的:** 仕様書の動作確認項目を一気通貫で確認し、リグレッションが無いことを検証する。

- [ ] **Step 1: 開発サーバー起動**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
wails dev
```

- [ ] **Step 2: 全項目確認**

仕様書 §5 の8項目を順に確認：

1. ☐ Level ヘッダーをクリックするとプルダウンが開く（▲▼ は表示されない）
2. ☐ プルダウン項目が「1, 2, 3, ..., 25, ???」順に並ぶ（発狂BMS難易度表で確認）
3. ☐ プルダウンから「12」等を選択すると、その level のエントリのみ表示される
4. ☐ プルダウンの「すべて」をクリックするとフィルタ解除される
5. ☐ フィルタ適用中は Level ヘッダー右側にフィルタアイコンが表示される
6. ☐ 表セレクタで別の難易度表に切り替えると、Level フィルタの選択値がクリアされる
7. ☐ Level カラムの幅が変化しない（`enableResizing: false` 維持）
8. ☐ 他カラム（Title, Artist, URL）のソートやリサイズは従来通り動作する

加えて以下のリグレッション項目：

9. ☐ Status カラムのフィルタが従来通り動作する（フィルタ機能の汎用拡張で壊していないか確認）
10. ☐ 検索ボックスでテキスト絞り込みが従来通り動作する
11. ☐ Level フィルタ＋検索テキストの併用が動作する（AND 条件で絞り込まれる）
12. ☐ 譜面の選択・選択解除（行クリック）、矢印キーナビゲーションが従来通り動作する

- [ ] **Step 3: 全 OK ならブランチをプッシュして PR の準備（PR 作成は別途ユーザー判断）**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git log --oneline main..HEAD
```

期待: Task 1〜4 の 4 コミットが並んでいる（設計ドキュメントコミットを含めれば 5 コミット）。

問題があれば該当タスクに戻って修正。
