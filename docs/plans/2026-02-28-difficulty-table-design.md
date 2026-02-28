# BMS難易度表の取り込み・表示

## 概要

BMS難易度表のデータをelsa.dbに保存し、譜面詳細ビューに難易度情報を表示する。

## 要件

- 難易度表のURL（table.html）を登録・削除できる
- 登録時にHTML→header.json→body JSONの3段階で取得しDBに保存
- 手動で「全て更新」でき、結果を成功/失敗の内訳で表示する
- 1譜面が複数テーブルに属するケースは全て保持する
- 譜面詳細ビューに難易度ラベル（`[st0]` `[★18]` 等）をバッジ表示する

## DBスキーマ

```sql
CREATE TABLE difficulty_table (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    url         TEXT NOT NULL UNIQUE,  -- ユーザーが入力したHTML URL (ex. https://stellabms.xyz/st/table.html)
    header_url  TEXT NOT NULL,         -- metaタグから取得したheader.json URL
    data_url    TEXT NOT NULL,         -- header.jsonから取得したbody JSON URL（絶対化済み）
    name        TEXT NOT NULL,         -- "Stella"
    symbol      TEXT NOT NULL,         -- "st"
    fetched_at  TEXT,                  -- 最終取得日時
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE difficulty_table_entry (
    table_id    INTEGER NOT NULL REFERENCES difficulty_table(id) ON DELETE CASCADE,
    md5         TEXT NOT NULL,
    level       TEXT NOT NULL,         -- "0", "12", "???" 等
    title       TEXT,
    artist      TEXT,
    url         TEXT,                  -- 本体URL（将来のworking_body_url推定用）
    url_diff    TEXT,                  -- 差分URL（同上）
    PRIMARY KEY (table_id, md5)
);
CREATE INDEX idx_dte_md5 ON difficulty_table_entry(md5);
```

設計判断:
- `difficulty_table_entry` はAUTOINCREMENTなし。`table_id + md5` の複合PKで、全件置換時のID爆発を防ぐ。
- `ON DELETE CASCADE` で難易度表削除時にエントリも消える。
- `md5` にインデックス → chart_meta/songdata.songとのJOINに使う。
- 難易度ラベル表示は `difficulty_table` とJOINして `symbol || level` で組み立てる。

## バックエンドAPI

Appに追加するメソッド:

- `AddDifficultyTable(url string) error` — 3段階取得してDB保存
- `RemoveDifficultyTable(id int) error` — 削除（CASCADE）
- `RefreshDifficultyTable(id int) RefreshResult` — 個別更新
- `RefreshAllDifficultyTables() []RefreshResult` — 全テーブル更新（途中で止めず全件処理）
- `ListDifficultyTables() []DifficultyTableDTO` — 一覧（エントリ数含む）

```go
type RefreshResult struct {
    TableName  string `json:"tableName"`
    Success    bool   `json:"success"`
    EntryCount int    `json:"entryCount"`
    Error      string `json:"error,omitempty"`
}
```

### データ取得フロー（AddDifficultyTable）

```
1. url のHTMLを fetch
2. <meta name="bmstable" content="..."> をパース → header_url（相対→絶対化）
3. header_url を fetch → name, symbol, data_url（相対→絶対化）
4. data_url を fetch → []entry
5. difficulty_table に INSERT
6. difficulty_table_entry に BULK INSERT
```

### エントリ更新（Refresh）

```
1. header_url を fetch → data_urlが変わっていれば更新
2. data_url を fetch → エントリ取得
3. DELETE FROM difficulty_table_entry WHERE table_id = ?
4. BULK INSERT 新エントリ
5. fetched_at を更新
```

### 譜面詳細への組み込み

`GetSongDetail` のレスポンス（ChartDTO）に `difficultyLabels` フィールドを追加。
個別にAPIを呼ぶとN+1になるため、GetSongDetail内でまとめて取得する。

```go
type DifficultyLabelDTO struct {
    TableName string `json:"tableName"`
    Symbol    string `json:"symbol"`
    Level     string `json:"level"`
}
```

## フロントエンド

### Settings.svelte の拡張

songdataDBPath設定の下に「難易度表」セクションを追加:
- 登録済みテーブル一覧（name, symbol, エントリ数, 最終取得日時）
- 各テーブルに「削除」ボタン
- URL入力欄 + 「追加」ボタン
- 「全て更新」ボタン → 結果をリスト表示（成功/失敗の内訳）

### SongDetail.svelte の拡張

譜面一覧の各譜面に難易度ラベルをバッジ表示:

```
7K / ANO / ☆12  [st0] [★18]   md5: 9188a4c9  ● IR
```

## スコープ外（将来）

- 段位認定（course）データの取り込み
- url/url_diff → working_body_url の推定・反映
- 未所持譜面の難易度表ベース表示・導入機能
- 譜面一覧ビューでのフィルタ・ソート
