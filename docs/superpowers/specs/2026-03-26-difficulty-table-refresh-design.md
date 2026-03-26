# 難易度表更新の並列化・進捗表示・個別更新

## 概要

難易度表の読み込みが遅い問題を改善する。バックエンドがGoogle Spreadsheetスクリプトであることが多く、逐次フェッチでは待ち時間が長い。並列リクエスト、個別更新ボタン、進捗表示を導入する。

## 設計方針

IR一括取得で確立済みのWails Eventsパターン（`ir:progress` / `ir:done`）を難易度表更新にも適用する。コードベース全体の一貫性を保ちつつ、キャンセル対応もcontextで自然に実現する。

## バックエンド

### 新規API

#### `RefreshAllDifficultyTablesAsync()`

- goroutineを起動して即座にreturnする非同期メソッド
- 内部でsemaphoreチャネル（cap=5）を使い、最大5テーブルを並列フェッチ
- 1テーブル完了ごとにWails Eventを発火:
  - `dt:refresh-progress` — `{ current: int, total: int, tableName: string, success: bool, error: string }`
  - `dt:refresh-done` — `{ results: []DifficultyTableRefreshResult }`
- `context.Context` + `context.CancelFunc` でキャンセル対応
- 既に実行中の場合はエラーを返す（二重実行防止）

#### `StopDifficultyTableRefresh()`

- 実行中の一括更新をキャンセルする
- 内部で `cancelFunc()` を呼び出し、goroutineが `ctx.Done()` で検知して停止

#### `IsRefreshing() bool`

- 一括更新が実行中かどうかを返す
- 設定ダイアログの再オープン時に進捗表示を復元するために使用

#### `RefreshProgress() (current int, total int)`

- 実行中の一括更新の現在の進捗を返す
- `IsRefreshing()` が true の場合のみ有効な値を返す

### 既存APIの変更なし

- `RefreshDifficultyTable(id)` — メインUIの個別更新ボタンでそのまま使用（同期RPC）
- `RefreshAllDifficultyTables()` — 同期版は残す（互換性）

### 実装パターン

IR一括取得（`ir_handler.go` の `StartDifficultyTableBulkFetch` + `bulk_fetch_ir.go`）と同じ構造を踏襲:

```
RefreshAllDifficultyTablesAsync()
  ├─ 二重実行チェック（mu.Lock）
  ├─ ctx, cancel = context.WithCancel()
  └─ go func() {
       sem := make(chan struct{}, 5)
       var wg sync.WaitGroup
       var mu sync.Mutex
       var completed int
       for _, t := range tables {
           sem <- struct{}{}
           wg.Add(1)
           go func(t) {
               defer func() { <-sem; wg.Done() }()
               result := refreshTable(ctx, t)
               mu.Lock()
               completed++
               mu.Unlock()
               EventsEmit(ctx, "dt:refresh-progress", progress)
           }(t)
       }
       wg.Wait()
       EventsEmit(ctx, "dt:refresh-done", allResults)
     }()
```

## フロントエンド

### メインUI（DifficultyTableView.svelte）

#### リフレッシュボタン

- テーブルセレクタ（`<select>`）の直後に配置
- 既存の `Icon` コンポーネントで `name="arrowPath"` を使用（Heroicons arrow-path、回転矢印）
- 新規アイコン追加不要
- サイズは `w-4 h-4` 程度（セレクタと調和する小さめサイズ）

#### 動作

- クリックで `RefreshDifficultyTable(selectedTableId)` を呼び出し（同期RPC）
- 更新中はアイコンに `animate-spin` クラスを付与して回転、ボタンはdisabled
- 完了後に「○件更新」のような一時メッセージを3秒間表示（IR取得ボタンの `doneMessage` パターンと同じ）
- 完了後にエントリを自動リロード（`loadEntries(selectedTableId)` を呼び出し）
- エラー時はエラーメッセージを一時表示

### 設定ダイアログ（DifficultyTableSettings.svelte）

#### 一括更新の変更

- 「全て更新」ボタンの動作を `RefreshAllDifficultyTablesAsync()` に変更
- ボタン押下後、進捗テキストを表示: 「更新中: 3/8 テーブル完了」
- `dt:refresh-progress` イベントをリッスンして current/total をリアルタイム更新
- `dt:refresh-done` イベントで最終結果一覧を表示（既存のチェックマーク/エラー表示パターンを流用）
- 更新中は「全て更新」ボタンを「停止」ボタンに切り替え（`StopDifficultyTableRefresh()` を呼ぶ）
- ダイアログを閉じても裏で処理は継続する。再度開いた際はバックエンドの `IsRefreshing() bool` と `RefreshProgress() (current, total int)` で現在の進捗を取得し、表示を復元する

#### イベントリスナーのライフサイクル

- `onMount` で `dt:refresh-progress` と `dt:refresh-done` をリッスン開始
- `onDestroy` でリスナーを解除

## Wails Events仕様

### `dt:refresh-progress`

テーブル1つの更新完了時に発火。

```typescript
{
  current: number    // 完了したテーブル数
  total: number      // 全テーブル数
  tableName: string  // 完了したテーブル名
  success: boolean   // 成功/失敗
  error: string      // 失敗時のエラーメッセージ（成功時は空文字）
}
```

### `dt:refresh-done`

全テーブルの更新完了（またはキャンセル）時に発火。

```typescript
{
  results: Array<{
    tableName: string
    success: boolean
    entryCount: number
    error: string
  }>
}
```

## スコープ外

- 起動時の自動更新
- 定期更新（バックグラウンドポーリング）
- 個別テーブル更新の非同期化（同期RPCで十分）
