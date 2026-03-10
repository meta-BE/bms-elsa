# 難易度表の並び替え機能 設計書

## 概要

難易度表設定モーダルで難易度表の表示順をドラッグ&ドロップで並び替えられるようにする。セレクターの順番もこれに追従する。

## 背景

現在 `difficulty_table` テーブルには並び順のカラムがなく、`ORDER BY name`（名前アルファベット順）で固定されている。ユーザーがよく使う難易度表を上に持ってきたいというニーズに対応する。

## 設計

### DB変更

`difficulty_table` に `sort_order INTEGER NOT NULL DEFAULT 0` カラムを追加。

```sql
ALTER TABLE difficulty_table ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;
UPDATE difficulty_table SET sort_order = id;
```

`ListTables` の ORDER BY を `sort_order, name` に変更。

### バックエンド API

`DifficultyTableHandler` に並び替えAPIを追加:

```go
func (h *DifficultyTableHandler) ReorderDifficultyTables(ids []int) error
```

`DifficultyTableRepository` に:

```go
func (r *DifficultyTableRepository) ReorderTables(ctx context.Context, ids []int) error
```

`InsertTable` も修正: 新規追加時は `MAX(sort_order) + 1` をセット。

### フロントエンド

**ライブラリ**: `svelte-dnd-action` を採用。
- Svelte 4 互換（peerDeps: `>=3.23.0`）
- 週間46,500 DL、2,079 star。Svelte DnD の定番
- ネイティブ Drag API 非依存のため Wails WebView でも動作する

**DifficultyTableSettings.svelte**:
- `<tbody>` に `use:dndzone` を適用
- 各行の左端にグリップハンドル（`⠿`）を表示
- `on:finalize` で `ReorderDifficultyTables` を呼び出し
- Svelte 4 での TypeScript 型定義を `app.d.ts` に追加

### セレクターへの反映

`DifficultyTableView` のセレクターは `ListDifficultyTables()` の返り値順で表示しているため、バックエンドの `ORDER BY sort_order` 変更で自動的に追従する。モーダル close 時の既存リフレッシュ処理でそのまま反映。

## 変更ファイル

| ファイル | 変更 |
|---------|------|
| `internal/adapter/persistence/migrations.go` | `sort_order` カラム追加マイグレーション |
| `internal/adapter/persistence/difficulty_table_repository.go` | ORDER BY変更、ReorderTables追加、InsertTable修正 |
| `internal/app/difficulty_table_handler.go` | `ReorderDifficultyTables` メソッド追加 |
| `frontend/src/settings/DifficultyTableSettings.svelte` | DnD並び替え、グリップハンドル |
| `frontend/src/app.d.ts` | svelte-dnd-action 型定義 |
| `frontend/package.json` | svelte-dnd-action 依存追加 |

## 選定理由

### sort_order カラム方式を採用した理由

- 隣接リスト（prev/next ポインタ）は実装が複雑でオーバーエンジニアリング
- フロントエンドのみ（localStorage等）はID不整合リスクがあり脆弱
- sort_order は最もシンプルで堅実。難易度表は10件程度なので全行UPDATEでも問題なし

### svelte-dnd-action を採用した理由

- ↑↓ボタン方式はシンプルだがDnDの方が直感的
- HTML5 Drag API 直接利用は実装複雑でタッチデバイス非対応
- svelte-dnd-action は Svelte エコシステムの定番で、Svelte 4 互換性・メンテナンス状況ともに良好
