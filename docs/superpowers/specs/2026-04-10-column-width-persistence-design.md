# カラム幅リサイズの全テーブル展開と永続化

## 概要

DiffImportViewで実装済みのTanStack Tableカラム幅リサイズ機能を、ChartListView・SongListView・DifficultyTableViewに展開する。
加えて、全テーブルのカラム幅をconfig.jsonに永続化し、Settingsモーダルにテーブルごとのリセット機能を追加する。

## 対象テーブル

| ビュー | リサイズ不可カラム |
|---|---|
| DiffImportView（既存） | `score`, `matchMethod`, `actions` |
| ChartListView | `ir` |
| SongListView | `ir` |
| DifficultyTableView | `level`, `statusLabel` |

DuplicateViewは素のHTML tableであり、対象外。

## config.json構造

```json
{
  "songdataDBPath": "...",
  "fileLog": false,
  "columnWidths": {
    "chartList": { "title": 0.30, "artist": 0.20, "genre": 0.14, "bpm": 0.10, "releaseYear": 0.06, "eventName": 0.12, "notes": 0.08 },
    "songList": { "title": 0.30, "artist": 0.20, "genre": 0.14, "bpm": 0.10, "releaseYear": 0.06, "eventName": 0.12, "chartCount": 0.08 },
    "difficultyTable": { "title": 0.55, "artist": 0.30, "hasUrl": 0.15 },
    "diffImport": { "fileName": 0.25, "title": 0.25, "artist": 0.25, "destFolder": 0.25 }
  }
}
```

- リサイズ可能カラムのみ記録する。リサイズ不可カラムは記録しない
- 値は**利用可能幅**（コンテナ幅 - 固定カラム合計px）に対する割合（0〜1の小数）
- テーブルの識別キーはビュー単位: `chartList`, `songList`, `difficultyTable`, `diffImport`

### Go側 Config struct

```go
type Config struct {
    SongdataDBPath string                        `json:"songdataDBPath"`
    FileLog        bool                          `json:"fileLog"`
    ColumnWidths   map[string]map[string]float64 `json:"columnWidths,omitempty"`
}
```

## カラム幅の復元フロー

1. ビュー表示時に `GetConfig()` でカラム幅設定を取得
2. configに該当ビューのキーがある場合:
   - 保存済みキー集合と現在のリサイズ可能カラムのキー集合を比較
   - **一致する場合** → 利用可能幅（コンテナ幅 - 固定カラム合計px）を算出 → 割合からpx変換 → `columnSizing` にセット
   - **不一致の場合** → configから該当ビューの設定を削除 → `SaveConfig()` → デフォルトのflex初期化にフォールバック
3. configにキーがない場合 → 現行通りflex → 計測 → 固定幅に切り替え

## カラム幅の保存フロー

1. リサイズ操作完了（mouseup）
2. 各リサイズ可能カラムの `現在のpx / 利用可能幅px` → 割合を算出
3. `SaveConfig()` で即時保存

## ウィンドウリサイズ対応

- ウィンドウの`resize`イベントを検知
- 新しい利用可能幅を算出
- 保存済み割合からpx再計算 → `setColumnSizing()` で反映
- 現在のDiffImportViewでは右側に空白ができるバグがあり、これも解消される

## Settingsモーダル

テーブルごとに「カラム幅をリセット」ボタンを4つ配置する:
- 楽曲一覧（chartList）
- 譜面一覧（songList）
- 難易度表（difficultyTable）
- 差分導入（diffImport）

リセット動作:
1. configから該当キーを削除
2. `SaveConfig()` で保存
3. 次回ビュー表示時にflex初期化にフォールバック

編集機能は不要。リセットのみ。

## リサイズの挙動

既存のDiffImportView実装と同じ:
- `columnResizeMode: 'onChange'`（リアルタイム更新）
- 隣接カラム間の双方向リサイズ（合計幅不変）
- 最小幅40px（`MIN_COL_WIDTH`）
- マウス・タッチ対応
- SortableHeaderコンポーネントのリサイズハンドルをそのまま利用
