# サーバーサイドページネーション設計

## 概要

初回表示時間の改善と大規模データ対応のため、現在の5,000件一括取得をサーバーサイドページネーションに変更する。

## 要件

- **初回描画の速さ**: アプリ起動後、最初の行が見えるまでの時間を短縮
- **大規模対応**: 20,000曲でも破綻しない設計
- **ソートのバックエンド一本化**: フロント側のJSソートを廃止し、バックエンドの `ORDER BY` に統一

## 現状の問題

- `onMount` で `ListSongs(1, 5000, 'title', false, '')` を呼び、全楽曲を一括取得
- フロントの TanStack Table `getSortedRowModel()` とバックエンドの `ORDER BY` でソートが二重構造
- 楽曲数が増えるとレスポンスサイズ・メモリ消費・初回待ち時間が線形に増加

## 設計

### アプローチ: サーバーサイドページネーション

フロントは表示に必要な分だけバックエンドに要求する。TanStack Virtual のスクロール位置に応じてページを動的に取得。

### データフロー

```
ユーザー操作（スクロール / ソート変更）
  ↓
SongTable.svelte
  ↓ 表示範囲から必要なページ番号を算出
  ↓ キャッシュにあればスキップ、なければ↓
ListSongs(page, PAGE_SIZE, sortBy, sortDesc, search)
  ↓
Go バックエンド（SQL ORDER BY + LIMIT/OFFSET）
  ↓
レスポンス → ページキャッシュに格納
  ↓
TanStack Virtual が該当行を描画
```

### パラメータ

| 項目 | 値 | 根拠 |
|------|-----|------|
| PAGE_SIZE | 100 | 1ページ約23KB。20,000曲でも200ページ |
| overscan | 20 | 上下各20行の先読み（現行と同じ） |

### フロントエンド変更（SongTable.svelte）

**データ管理の変更:**
- `data: dto.SongRowDTO[]` → `pageCache: Map<number, dto.SongRowDTO[]>`
- `totalCount` はバックエンドから取得した全件数（Virtual の `count` に使用）

**TanStack Table の変更:**
- `getSortedRowModel()` を削除（バックエンドソートのみ）
- `onSortingChange` でキャッシュクリア → ページ1を再取得 → スクロール先頭にリセット

**TanStack Virtual の変更:**
- `count` を `totalCount` に設定（全件数分のスクロール領域を確保）
- スクロールイベントで表示範囲のページを算出し、未取得なら非同期取得

**行データアクセス:**
```
virtualRow.index → pageIndex = Math.floor(index / PAGE_SIZE)
                 → rowIndex  = index % PAGE_SIZE
                 → pageCache.get(pageIndex)?.[rowIndex]
```

未取得の行はプレースホルダー（薄いローディング表示）を出す。

### バックエンド変更

変更なし。現在の `ListSongs(page, pageSize, sortBy, sortDesc, search)` がそのまま使える。

### ソート切替の動作

1. ユーザーがカラムヘッダーをクリック
2. `sorting` ステートを更新
3. `pageCache` を全クリア
4. `totalCount` は維持（件数は変わらない）
5. ページ1をバックエンドから取得
6. スクロール位置を先頭にリセット
7. Virtual が表示範囲のページを要求 → 取得

### エッジケース

- **高速スクロール**: 表示範囲のページのみ取得し、飛ばしたページは必要になったら取得
- **リクエスト中のソート切替**: 進行中リクエストの結果はバージョン番号で無視
- **空データ**: totalCount=0 なら「楽曲がありません」表示

## 採用しなかったアプローチ

### B. 段階的読み込み
初回500件→残りバックグラウンド取得。結局全件メモリに載る点、バックエンドソート一本化と矛盾する点で不採用。

### C. 無限スクロール
末尾到達で次ページ追加。ソート切替時の全データ破棄→再取得の体験が悪い点で不採用。
