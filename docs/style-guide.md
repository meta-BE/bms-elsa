# フロントエンド スタイルガイド

## テキストサイズ階層

| サイズ | 用途 | 例 |
|--------|------|-----|
| `text-lg font-bold` | 詳細ビューのメインタイトル | 楽曲名、譜面名 |
| `text-sm font-semibold` | セクション見出し、件数表示 | 「譜面一覧」、「1,234 songs」 |
| `text-sm` | テーブルセル、アーティスト名等 | 行データ、補足テキスト |
| `text-xs font-bold uppercase` | テーブルカラムヘッダー | SortableHeader |
| `text-xs` | メタデータ、補助情報 | ジャンルラベル、IR情報、パス |

## テキスト色・透明度

| クラス | 用途 |
|--------|------|
| `text-base-content` | デフォルト |
| `text-base-content/70` | 準主要（アーティスト、subtitle） |
| `text-base-content/50` | 補助（ジャンル、パス、メタデータ） |

## リストビュー（タブの上ペイン）

- 外枠: `h-full flex flex-col bg-base-100 rounded-lg border border-base-300`
- ヘッダーバー: `px-4 py-2 bg-base-200 rounded-t-lg flex items-center justify-between gap-2`
- 件数: `text-sm font-semibold shrink-0`
- カラムヘッダー: `SortableHeader` コンポーネント
- 行高: 32px（1行）/ 52px（2行: title+subtitle）
- セル: `px-2 text-sm truncate`
- 選択行: `bg-primary/20`
- ホバー: `hover:bg-base-200`

## 詳細ビュー（タブの下ペイン）

- コンテナ: `flex flex-col gap-3`
- セクションカード: `bg-base-200 rounded-lg p-3`
- タイトル: `text-lg font-bold truncate`
- アーティスト: `text-sm text-base-content/70`
- ジャンル（タイトル上）: `text-xs text-base-content/50`
- セクション見出し: `text-sm font-semibold mb-2`
- セクション内容: `text-xs space-y-1`
- 閉じるボタン: `btn btn-ghost btn-xs`

## ボタン

| クラス | 用途 |
|--------|------|
| `btn btn-primary` | 主要アクション |
| `btn btn-xs btn-outline` | ヘッダー内の操作ボタン |
| `btn btn-ghost btn-xs` | 閉じる、軽量な操作 |

## 共通コンポーネント

- **SplitPane**: 上下分割レイアウト（上=リスト、下=詳細）
- **SortableHeader**: ソート可能なカラムヘッダー
- **SearchInput**: インクリメンタル検索（`input-xs input-bordered w-48`）

## テーマ

- daisyUI テーマ: `emerald`
- App 全体: `data-theme="emerald"`
