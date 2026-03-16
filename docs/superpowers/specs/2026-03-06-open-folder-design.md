# インストール先フォルダを開く機能

## 概要

楽曲詳細・譜面詳細・エントリ詳細パネルのヘッダーに「フォルダを開く」ボタンを追加し、
譜面ファイルが格納されているフォルダをOSのファイルマネージャ（Finder/Explorer）で開けるようにする。

## バックエンドAPI

### `App.OpenFolder(filePath string) error`

`app.go` に追加。

- `filePath`: 譜面ファイルのフルパス（`ChartDTO.path`）
- `filepath.Dir()` で親ディレクトリを算出
- `os.Stat()` でディレクトリの存在を検証
- OS別コマンドで開く:
  - macOS: `open <dir>`
  - Windows: `explorer <dir>`
  - Linux: `xdg-open <dir>`
- 既存の `OpenURL()` と同じ `exec.Command` パターンを踏襲

## フロントエンドUI

### 共通パターン

各詳細パネルのヘッダー部分（タイトル行の右側、✕ボタンの左）にフォルダアイコンボタンを追加。

```
┌───────────────────────────────┐
│  Genre                        │
│  楽曲タイトル      [📁] [✕]  │
│  Artist                       │
└───────────────────────────────┘
```

- スタイル: `btn btn-ghost btn-xs`
- ツールチップ: `title="インストール先フォルダを開く"`
- パスが無い場合はボタンを非表示

### パネル別のパス取得元と表示条件

| パネル | パス取得元 | 表示条件 |
|--------|-----------|---------|
| SongDetail | `detail.charts[0]?.path` | `detail.charts.length > 0` |
| ChartDetail | `chart.path` | `chart != null` |
| EntryDetail | `chart.path` | `chart != null`（導入済みのみ） |

## 変更対象ファイル

- `app.go`: `OpenFolder` メソッド追加
- `frontend/src/SongDetail.svelte`: ヘッダーにボタン追加
- `frontend/src/ChartDetail.svelte`: ヘッダーにボタン追加
- `frontend/src/EntryDetail.svelte`: ヘッダーにボタン追加（導入済み条件付き）
