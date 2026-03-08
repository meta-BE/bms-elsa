# OpenFolderButton コンポーネント化 設計

## 目的
「フォルダを開く」ボタンを共通コンポーネントに抽出し、DiffImportViewに追加する。

## コンポーネント仕様

**ファイル**: `frontend/src/components/OpenFolderButton.svelte`

**Props**:
- `path: string` — 開くフォルダのパス
- `size: "xs" | "sm"` — デフォルト `"sm"`

**サイズマッピング**:
| size | ボタン | アイコン |
|------|--------|---------|
| `"xs"` | `btn-xs` | `h-3 w-3` |
| `"sm"` | `btn-xs` | `h-4 w-4` |

**動作**: `path` が空/未定義の場合はボタンを非表示

## 変更箇所

| ファイル | 変更内容 |
|---------|---------|
| `components/OpenFolderButton.svelte` | 新規作成 |
| `views/DiffImportView.svelte` | ファイル名セルと推定先セルの左にアイコン追加（size="xs"） |
| `views/SongDetail.svelte` | 既存のフォルダボタンをコンポーネントに置換 |
| `views/ChartDetail.svelte` | 同上 |
| `views/EntryDetail.svelte` | 同上 |
| `components/InstallCandidateCard.svelte` | 同上 |
