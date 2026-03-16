# MinHashスキャン（フォルダ走査）設計

## Goal

songdata.dbに登録済みの譜面に対してBMSファイルをパースし、WAV定義からMinHash署名を計算して`chart_meta.wav_minhash`に保存する機能を実装する。

## 前提

- BMSパーサー（`ParseWAVFiles`）とMinHash計算（`ComputeMinHash`）は実装済み
- `chart_meta`テーブルに`wav_minhash BLOB`カラムは追加済み
- IR一括取得と同じWailsイベントパターン（進捗表示・中断対応）を踏襲する

## データフロー

```
[フロントエンド] ボタンクリック
    ↓
[ScanHandler.StartMinHashScan()] Wailsバインディング
    ↓
songdata.db + chart_meta JOIN → wav_minhashがNULLの譜面リスト取得
    ↓ (md5, path) のペアのリスト
1件ずつループ:
    1. ParseWAVFiles(path) → WAVファイル名集合
    2. ComputeMinHash(wavFiles) → MinHashSignature
    3. UPDATE chart_meta SET wav_minhash = ? WHERE md5 = ?
    4. EventsEmit("scan:progress", {current, total})
    ↓
全件完了 or 中断
    ↓
EventsEmit("scan:done", {total, computed, skipped, failed, cancelled})
```

## バックエンド設計

### 新規: `internal/app/scan_handler.go`

| メソッド | 説明 |
|---------|------|
| `StartMinHashScan()` | 走査開始（goroutineで非同期実行） |
| `StopMinHashScan()` | 走査中断（context.Cancel） |
| `IsMinHashScanRunning() bool` | 走査中か判定 |

IR一括取得（`IRHandler`）と同じパターン: `context.WithCancel`で中断制御。

### 既存追加: `internal/adapter/persistence/elsa_repository.go`

| メソッド | 説明 |
|---------|------|
| `ListChartsWithoutMinhash() ([]ChartScanTarget, error)` | wav_minhashがNULLの譜面のmd5+pathリストを返す |
| `UpdateWavMinhash(md5 string, minhash []byte) error` | chart_metaのwav_minhash更新 |

```go
type ChartScanTarget struct {
    MD5  string
    Path string
}
```

### 既存修正: `app.go`

`App`構造体に`ScanHandler`を追加し、Wailsバインディングを登録。

## フロントエンド設計

### `ChartListView.svelte`

- IR一括取得ボタンの横に「MinHash計算」ボタンを配置
- 走査中: 進捗表示（`計算中: 123 / 4567`）＋「停止」ボタン
- 完了時: 結果メッセージ（`計算完了: 計算 4500 / スキップ 50 / 失敗 17`）を5秒間表示

### Wailsイベント

| イベント | ペイロード |
|---------|----------|
| `scan:progress` | `{current: number, total: number}` |
| `scan:done` | `{total, computed, skipped, failed, cancelled}` |

## エラー処理

- ファイル未発見（削除済み等）: スキップして継続、`skipped`カウントに加算
- パースエラー: スキップして継続、`failed`カウントに加算
- ユーザー中断: ループ終了、処理済み分は保存済み、`cancelled: true`

## テスト戦略

- `ListChartsWithoutMinhash`: songdata.db + chart_metaの結合クエリが正しく動作するか
- `UpdateWavMinhash`: BLOBの保存・読み取りの往復確認
- 既存テスト（BMSパーサー8件、persistence層20件）が引き続きPASS
