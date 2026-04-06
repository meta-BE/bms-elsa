# 重複検知スキャンのバックグラウンドタスク化

## 概要

重複検知タブのスキャン動作を、他のバックグラウンドタスク（MinHash・難易度表更新・動作URL推定）と同様に起動時自動実行へ移行する。

## 要件

- 起動時に自動実行（MinHashスキャン完了後に開始）
- スキャン結果はバックエンドメモリにキャッシュ
- ProgressBarは0%/100%の2状態（進捗コールバック不要）
- DuplicateViewのスキャンボタンは完全削除
- Settings.svelteに4つ目のバックグラウンドタスクとして表示

## アプローチ: コールバック注入

`ScanHandler.StartMinHashScan` に完了コールバック `onDone func()` を追加し、`startup()` で重複検知スキャンの起動を渡す。

## バックエンド変更

### DuplicateHandler（`internal/app/duplicate_handler.go`）

既存の `ScanHandler` と同じバックグラウンドタスクパターンを適用:

- `mu sync.Mutex` / `running bool` で二重実行防止
- `results []similarity.DuplicateGroup` フィールドでスキャン結果をキャッシュ

新規メソッド:
- `StartScanDuplicates()` — goroutineでスキャン実行。`dup:progress`（0/1→1/1）と`dup:done`（グループ数）イベントを発火
- `GetDuplicateGroups() []similarity.DuplicateGroup` — キャッシュ済み結果を返す
- `IsDuplicateScanRunning() bool` — 状態確認用

### ScanHandler（`internal/app/scan_handler.go`）

`StartMinHashScan` のシグネチャ変更:

```go
func (h *ScanHandler) StartMinHashScan(onDone func()) error
```

goroutine末尾で `scan:done` イベント発火後に `onDone()` を呼ぶ。

### app.go の startup()

```go
a.ScanHandler.StartMinHashScan(func() {
    a.DuplicateHandler.StartScanDuplicates()
})
```

## フロントエンド変更

### DuplicateView.svelte

- `handleScan()` / スキャンボタンを削除
- `onMount` で `EventsOn("dup:done", ...)` をリッスンし、完了時に `GetDuplicateGroups()` で結果取得
- 初回表示時に `IsDuplicateScanRunning()` で状態確認:
  - 実行中 → スピナー表示
  - 完了済み（結果あり） → `GetDuplicateGroups()` で即表示
  - 未開始（結果なし） → 待機状態表示

### Settings.svelte

4つ目のセクション「重複検知スキャン」を追加:
- `dup:progress` / `dup:done` をリッスン
- ProgressBar（current=0, total=1 → current=1, total=1）
- 完了サマリー: 「Nグループ検出」

## 依存関係

```
startup()
  ├── MinHashスキャン（即時開始）
  │     └── onDone → 重複検知スキャン開始
  ├── 難易度表更新（即時開始）
  └── 動作URL推定（即時開始）
```

## イベント一覧

| イベント名 | ペイロード | タイミング |
|-----------|-----------|-----------|
| `dup:progress` | `{current: 0\|1, total: 1}` | 開始時(0/1)、完了時(1/1) |
| `dup:done` | `{groups: int}` | スキャン完了時 |
