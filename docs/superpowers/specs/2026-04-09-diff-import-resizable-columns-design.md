# 差分導入画面 カラムリサイズ対応設計

## 概要

DiffImportView のテーブルを @tanstack/svelte-table + @tanstack/svelte-virtual に移行し、ヘッダードラッグによるカラム幅リサイズ機能を追加する。

## 背景

- DiffImportView は素の HTML `<table>` で実装されている
- プロジェクト内の他ビュー（SongTable, ChartListView, DifficultyTableView）は @tanstack/svelte-table を使用済み
- テーブル実装を統一しつつ、カラムリサイズ機能を導入する

## スコープ

- DiffImportView のテーブルを tanstack table + 仮想スクロールに移行
- SortableHeader にリサイズハンドルを追加（全テーブル共通）
- カラム幅の永続化は行わない（毎回デフォルト幅）
- DiffImportView にソート・フィルタは追加しない

## 設計

### カラム定義

| カラム | accessor | size | meta.flex | 備考 |
|--------|----------|------|-----------|------|
| ファイル名 | `fileName` | 200 | true | OpenFolderButton + truncate、cellでfilePath参照 |
| TITLE | accessorFn (title+subtitle) | 200 | true | truncate |
| ARTIST | accessorFn (artist+subartist) | 200 | true | truncate |
| 推定先 | `destFolder` | 250 | true | OpenFolderButton + truncate、空なら「-」 |
| スコア | accessorFn (score→整数) | 64 | false | font-mono |
| 推定方法 | accessorFn (matchMethod→ラベル) | 80 | false | ラベルマッピング |
| 操作 | id: 'actions' | 64 | false | クリア/削除ボタン、リサイズ無効 |

### 仮想スクロール

- ROW_HEIGHT: 32px
- overscan: 20
- 既存のSongTableと同じパターンを踏襲

### SortableHeader リサイズハンドル

- 各ヘッダーセルの右端に幅4pxの透明なドラッグハンドルを配置
- ホバー時: `bg-primary/50` で可視化
- ドラッグ中: `bg-primary` で強調、`cursor-col-resize`
- @tanstack/table の `enableColumnResizing` + `columnResizeMode: 'onChange'` を使用
- `header.getResizeHandler()` でマウスイベントハンドラを取得
- `header.column.getIsResizing()` でドラッグ中状態を判定
- ソートクリックとリサイズドラッグが競合しないよう `stopPropagation`
- ソートヘッダー・フィルタヘッダー両方に共通で付与

### 既存テーブルへの影響

- `enableColumnResizing: true` を渡したビューのみリサイズが有効
- 既存ビュー（SongTable等）は変更不要、リサイズ無効のまま

### DiffImportView テンプレート構造

変更範囲: テーブル部分（150-218行）のみ書き換え。ヘッダーバー、空状態、フッターはそのまま。

```
<div class="flex-1 overflow-hidden flex flex-col">
  <SortableHeader table={$table} />
  <div bind:this={scrollElement} class="flex-1 overflow-y-scroll">
    <div style="height: {totalSize}px; position: relative;">
      {#each virtualItems as virtualRow}
        <div style="height: 32px; transform: translateY(...);">
          {#each row.getVisibleCells() as cell}
            <!-- セルレンダリング -->
          {/each}
        </div>
      {/each}
    </div>
  </div>
</div>
```

### セルのカスタムレンダリング

- ファイル名・推定先: `cell` プロパティにカスタムレンダリング関数を定義
- 操作カラム: クリア/削除ボタンのカスタムレンダリング
- ボタンのクリックハンドラは `filePath` で行を特定（仮想スクロール対応）

### データ更新

- `candidates` 配列のリアクティブ更新パターンはそのまま維持
- tanstack table は `data` プロパティの変更で自動再描画
