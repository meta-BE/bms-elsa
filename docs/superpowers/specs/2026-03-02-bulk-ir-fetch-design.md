# IR一括取得機能 設計

## 概要

未取得譜面のLR2IR情報をバックグラウンドで逐次取得する単機能。
メタ推測機能の前提条件として、IR URL取得済みの曲だけがマッチング対象になる。

## アーキテクチャ

既存の`LookupIRUseCase`（1MD5単位）をループで呼び出す**`BulkFetchIRUseCase`**を新設。
Wailsイベント（`runtime.EventsEmit`）で進捗をフロントに通知し、`context.Cancel`で中断対応。

## バックエンド変更

### 1. LookupIRUseCase修正

未登録（LR2IRに存在しない）譜面でも`lr2ir_fetched_at`をセットしてDB保存する。
再取得対象から外すことで、一括取得の再実行時に無駄なリクエストを防ぐ。

### 2. 新リポジトリメソッド: ListUnfetchedChartKeys

songdata.db（sdスキーマ）とelsa.dbのchart_metaをLEFT JOINし、
`chart_metaにレコードなし OR lr2ir_fetched_at IS NULL`のmd5/sha256ペアを返す。

### 3. 新ユースケース: BulkFetchIRUseCase

```
BulkFetchIRUseCase
  - irClient: port.IRClient
  - metaRepo: model.MetaRepository

Execute(ctx, progressFn func(current, total int))
  1. ListUnfetchedChartKeys() → 対象リスト
  2. for each (md5, sha256):
     - ctx.Err() チェック（中断対応）
     - LookupIRUseCase.Execute(md5, sha256)
     - progressFn(i+1, total)
  3. 結果集計を返却
```

レートリミットはLR2IRClient内の1秒/リクエスト制限で既に制御済み。

### 4. IRHandler拡張

- `StartBulkFetch()` — goroutineで起動、`runtime.EventsEmit("ir:progress", ...)` で進捗通知
- `StopBulkFetch()` — context cancelで中断
- 二重起動防止（実行中フラグ）

## フロントエンド変更

### UIイメージ

SongTableヘッダーバーに「IR取得」ボタンを追加（「メタ推測」ボタンの隣）。
モーダルは使わず、ヘッダーバーのインライン表示で完結。

- **待機中**: `IR取得` ボタン（btn-xs btn-outline）
- **実行中**: `取得中: 1234 / 5000` テキスト + `停止` ボタン
- **完了**: 結果サマリーを数秒表示 → ボタンに戻る

### Wailsイベント

```
ir:progress  → { current: number, total: number, lastTitle: string }
ir:done      → { fetched: number, notFound: number, failed: number, cancelled: boolean }
```

## データフロー

```
ユーザー「IR取得」クリック
  → StartBulkFetch() (Wails binding)
    → goroutine起動
      → ListUnfetchedChartKeys() → [(md5,sha256), ...]
      → for each: LookupIRUseCase.Execute(md5, sha256)
        → LR2IRClient.LookupByMD5 (1秒/req制限)
        → UpsertChartMeta (未登録でもfetched_atセット)
        → EventsEmit("ir:progress", {current, total, lastTitle})
      → EventsEmit("ir:done", {結果サマリー})

フロント: EventsOn("ir:progress") → プログレス更新
フロント: EventsOn("ir:done") → 完了表示、楽曲リスト再読み込み

ユーザー「停止」クリック
  → StopBulkFetch() → context.Cancel → ループ中断 → ir:done送信
```

## 決定事項

- 未登録譜面でもfetched_atを記録し、再取得対象から外す
- 再取得したい場合は別途「再取得」操作を用意する（スコープ外）
