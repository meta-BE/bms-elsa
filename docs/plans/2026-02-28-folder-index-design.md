# songdata.song folderインデックス追加による初回表示高速化

## 概要

初回表示時間の改善のため、songdata.dbのsongテーブルにfolderカラムのインデックスを追加する。

## 問題

ListSongsクエリにEXISTS相関サブクエリがあり、各フォルダグループ（2,666回）ごとにsongテーブル（15,437行）のフルテーブルスキャンが発生していた。folderカラムにインデックスがないことが根本原因。

### 計測結果（testdata/songdata.db、2,666フォルダ・15,437譜面）

| クエリ | 実行時間 |
|--------|---------|
| 現在のクエリ（インデックスなし） | 2.389秒 |
| 現在のクエリ + folderインデックス | 0.023秒 |
| EXISTS除外 + folderインデックス | 0.014秒 |

## 設計

### 変更内容

`AttachSongdata`関数でsongdata.dbをATTACH後、`CREATE INDEX IF NOT EXISTS`でfolderインデックスを作成する。

- 冪等: 既にインデックスが存在する場合は何も行わない
- 非破壊: テーブルのデータには一切変更を加えない
- SQLクエリ自体の変更は不要

### 変更ファイル

- `internal/adapter/persistence/songdata_reader.go`: AttachSongdata関数にインデックス作成を追加
- `README.md`: songdata.dbへの書き込みに関する注意事項を追記

### 採用しなかったアプローチ

前回のセッションでサーバーサイドページネーション（PAGE_SIZE=100）を試みたが、同じ重いSQLクエリが最大27回実行され逆に遅くなった。詳細は `2026-02-28-server-side-pagination-retrospective.md` を参照。ボトルネックの正確な特定（インデックス欠如）が先に必要だった。
