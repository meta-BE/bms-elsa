# BMS Search連携によるイベント情報管理の刷新

## 概要

イベント情報の管理を、現在のURLパターンマッチング方式（event_mapping）からBMS Search API連携に切り替える。イベントマスターテーブルを導入し、楽曲とイベントの紐付けをBMS Searchから自動取得する。

## 現状の課題

- event_mappingテーブル（232件のURLパターン）を手動メンテナンスする必要がある
- LR2IRのBody URLに依存しており、IR未取得の楽曲にはイベント情報を設定できない
- URLパターンマッチングは最初にマッチしたものを採用するため、パターンの順序に依存する

## 設計方針

- BMS Search APIのexhibition情報をイベントマスターとして活用する
- 楽曲→イベントの紐付けはBMS Search APIから自動取得する（MD5 → Pattern → BMS → exhibition）
- 短縮名（BOF:TT等）はeventマスターのshort_nameカラムで管理する
- event_mappingテーブルは廃止する

## DBスキーマ

### eventテーブル（新規）

```sql
CREATE TABLE IF NOT EXISTS event (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    bms_search_id   TEXT UNIQUE,
    name  TEXT NOT NULL,
    short_name     TEXT NOT NULL,
    release_year   INTEGER NOT NULL,
    created_at     TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at     TEXT NOT NULL DEFAULT (datetime('now'))
);
```

- `bms_search_id`: BMS Search exhibition ID（例: `"0FMNS6rxB5CIA8"`）。BMS Search由来でないレコードはNULL
- `name`: BMS Search上の正式名称（例: `"THE BMS OF FIGHTERS 2015 -Time Travelers-"`）
- `short_name`: 表示用の短縮名（例: `"BOF:TT"`）。初期値はnameのコピー、後から編集可能
- `release_year`: イベントの開催年

初期データはevent.csv（既存event_mappings.csv + BMS Search APIイベント一覧のマージ）から投入する。

### song_metaテーブルの変更

```sql
ALTER TABLE song_meta ADD COLUMN event_id INTEGER REFERENCES event(id);
ALTER TABLE song_meta ADD COLUMN bms_search_id TEXT;
ALTER TABLE song_meta DROP COLUMN event_name;
-- release_year は維持（イベントに属さない楽曲用）
```

変更後のスキーマ:
```sql
CREATE TABLE IF NOT EXISTS song_meta (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    folder_hash     TEXT NOT NULL UNIQUE,
    release_year    INTEGER,
    event_id        INTEGER REFERENCES event(id),
    bms_search_id   TEXT,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
);
```

- `bms_search_id`: BMS SearchのBMS ID（例: `"Y--E3erQ36Ap-F"`）。BMS Search同期時に設定。将来的にダウンロードURL等の取得に活用

### event_mappingテーブル

マイグレーションで削除する。既存データの移行は行わない（BMS Search同期で再取得する）。

## 表示ロジック

### イベント名

`event_id` が設定されていれば `event.short_name` をJOINで表示。未設定なら空。

```sql
SELECT sm.*, e.short_name AS event_name, e.name
FROM song_meta sm
LEFT JOIN event e ON sm.event_id = e.id
```

### release_year

`event.release_year` → `song_meta.release_year` の順にフォールバック。

```sql
COALESCE(e.release_year, sm.release_year) AS release_year
```

## BMS Search同期フロー（楽曲→イベント紐付け）

バックグラウンド一括取得。IR一括取得と同じWails Eventsパターン。

### 処理フロー

1. songdata.dbの全MD5一覧を取得
2. song_metaにevent_idが未設定の楽曲を対象とする
3. 各MD5に対して `GET /patterns/{md5}` を呼ぶ
4. レスポンスから `bms.id` を取得
5. `GET /bmses/{bms.id}` を呼ぶ
6. `exhibition.id` があれば、eventテーブルの `bms_search_id` と照合してevent_idを取得
7. song_metaの `event_id` と `bms_search_id` を更新
8. 進捗をWails Eventsで通知

### レート制限

BMS Search APIにはレートリミットの明示がないが、1リクエストあたり最低でもLR2IR同様の間隔（500ms程度）を設ける。1楽曲あたり最大2リクエスト（Pattern + BMS）が必要。

### 最適化

- 同一 `bms.id` のBMS詳細はキャッシュする（同じBMS内の複数譜面で重複リクエストを防ぐ）
- フォルダ単位で処理し、同一フォルダ内の最初のMD5でヒットすればそのフォルダは完了とする

## eventマスターの更新機能

### BMS Searchからのイベント取得

1. `GET /exhibitions/search` で全イベントを取得（ページネーションで全件走査）
2. `bms_search_id` がeventテーブルに存在しないレコードをINSERT
3. `short_name` の初期値は `name` のコピー
4. 既存レコードは `name` のみ更新（short_nameはユーザー編集を尊重して上書きしない）

### short_nameの編集UI

設定画面にイベントマスター管理UIを追加。イベント一覧テーブルで `short_name` をインライン編集可能にする。

## フロントエンド

### SongDetail — イベント設定UI

- 現在のevent_name自由テキスト入力 → eventマスターからのオートコンプリート付きドロップダウンに変更
- eventマスターのshort_nameで検索・選択
- release_yearは従来通り数値入力。event_id設定時はevent.release_yearが表示される（読み取り専用）。event_id未設定時のみsong_meta.release_yearを手動編集可能

### SongTable — 一覧表示

- Eventカラム: `COALESCE(e.short_name, '')` を表示
- Yearカラム: `COALESCE(e.release_year, sm.release_year)` を表示
- フィルタ・検索は従来通り

### InferenceModal — 廃止

現在のURLパターンマッチ推測UI（3フェーズ: 自動実行→結果→手動確認）はevent_mapping廃止に伴い削除。BMS Search同期に置き換える。

### EventMappingManager — 廃止→EventManager に変更

URLパターンマッピングの管理UIを廃止し、eventマスターの管理UIに変更:
- イベント一覧表示（name, short_name, release_year）
- short_nameのインライン編集
- 「BMS Searchから更新」ボタンで新規イベントを取得

## event.csv の生成

初回マイグレーション用のevent.csvは以下のソースをマージして生成する:

1. **既存のevent_mappings.csv**: url_pattern, event_name, release_year → event_nameをshort_name、nameの初期値として使用。bms_search_idは後でBMS Searchと突合して紐付け
2. **BMS Search `/exhibitions/search`**: 全イベントを取得し、bms_search_id, name, release_yearを取得

マージルール:
- BMS SearchのイベントをベースとしてINSERT
- event_mappings.csvのevent_nameをshort_nameとして、BMS Searchのnameと手動で対応付け（スクリプトで生成後、手動確認）

## スコープ外

- LR2IR取得との統合（将来的には起動時バックグラウンド処理に統合予定だが、今回は独立した一括取得として実装）
- chart_metaへのBMS Search情報保存（ダウンロードURL等の活用は別タスク）
- イベントページのWebスクレイピング
