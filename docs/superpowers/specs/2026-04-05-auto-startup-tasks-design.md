# 起動時バックグラウンドタスク自動実行

## 概要

アプリ起動時にMinHashスキャンと難易度表一括更新を自動実行する。タスクの進捗・状態は基本設定モーダルで確認できるようにする。

## 対象タスク

| タスク | 自動実行 | UI変更 |
|---|---|---|
| MinHashスキャン | する | 譜面一覧の起動ボタンを削除 |
| 難易度表一括更新 | する | 既存UIをすべて維持（手動実行も可能） |

LR2IR一括取得・BMS Search同期は外部API負荷の観点から除外。重複検知スキャンも除外。

## バックエンド

### 変更箇所: `app.go` の `startup()` メソッド

既存の `SetContext()` 呼び出し後に、2つのバックグラウンドタスクを並列起動する。

```go
a.ScanHandler.StartMinHashScan()
a.DifficultyTableHandler.RefreshAllDifficultyTablesAsync()
```

- 両メソッドとも内部でgoroutineを起動して即座にreturnするため、startupをブロックしない
- 既存の二重起動防止（`running` フラグ）により、難易度表の手動実行との競合も安全
- エラーはWailsイベント経由でフロントに通知される
- 新規メソッド・構造体の追加は不要

## フロントエンド

### 1. MinHash起動ボタンの削除

**ファイル: `frontend/src/views/ChartListView.svelte`**

以下を削除:
- 「MinHash計算」ボタン（ヘッダー部分）
- スキャン中の進捗表示・停止ボタン
- 関連する状態変数（`scanning`, `scanProgress`, `scanResult`等）
- Wailsイベント購読（`scan:progress`, `scan:done`）
- `startMinHashScan()`, `stopMinHashScan()` 関数

### 2. 基本設定モーダルに進捗セクション追加

**ファイル: `frontend/src/settings/Settings.svelte`**

既存の設定項目（songdata.dbパス、ファイルログ）の下に「バックグラウンドタスク」セクションを追加する。

```
┌─────────────────────────────────────┐
│ songdata.db パス  [...]  [参照]     │
│ □ ファイル別ログ出力                │
│                                     │
│ ── バックグラウンドタスク ──        │
│                                     │
│ MinHashスキャン        完了          │
│ ████████████████████████ 2600/2600  │
│                                     │
│ 難易度表更新           実行中...    │
│ ████████░░░░░░░░░░░░░░  3/8        │
│                                     │
│ (エラー時のみ表示)                  │
│ ⚠ 難易度表更新でエラー: Stella...  │
└─────────────────────────────────────┘
```

#### 状態の取得方法

- モーダルopen時に `IsMinHashScanRunning()` / `IsRefreshing()` + `RefreshProgress()` でバックエンド状態を取得
- Wailsイベント（`scan:progress/done`, `dt:refresh-progress/done`）を購読してリアルタイム更新
- モーダルclose時にイベント購読を解除

#### 表示する状態

各タスクは以下の状態を表示:
- 「実行中...」+ DaisyUI `<progress>` バー + 件数テキスト
- 「完了」
- エラーがあれば赤字で表示

#### プログレスバー

DaisyUI 5の `<progress>` コンポーネントを使用:

```html
<div class="flex items-center gap-2 text-xs">
  <progress class="progress progress-primary flex-1" value={current} max={total}></progress>
  <span>{current}/{total}</span>
</div>
```

### 3. 難易度表設定タブ

変更なし。既存の「全て更新」ボタン・個別更新ボタン・進捗表示をすべて維持する。起動時の自動実行中に手動で「全て更新」を押した場合、既存の二重起動防止により無視される（既存の挙動通り）。
