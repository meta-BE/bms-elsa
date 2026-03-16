# subtitle/subartist 表示追加 デザインドキュメント

## 概要

songdata.dbに存在するsubtitle/subartistフィールドを各画面に表示する。
現状はバックエンドの一部メソッド（GetSongByFolder, GetChartByMD5）でのみ取得しており、DTOやフロントエンドには未反映。

## 対象画面

| 画面 | 一覧テーブル | 詳細ビュー |
|------|-------------|-----------|
| 楽曲タブ | SongTable: 変更なし | SongDetail: 譜面リスト各行にsubtitle表示 |
| 譜面タブ | ChartListView: Title/Artistの下に小さく表示 | ChartDetail: タイトル下にsubtitle、アーティスト下にsubartist |
| 難易度表タブ | — | EntryDetail: 導入済み時のみ表示 |

SongTableはSong（フォルダ）単位の集約なので、譜面ごとに異なるsubtitle/subartistは表示しない。

## バックエンド

### songdata_reader.go

- `ListAllCharts` SQLに`s.subtitle`, `COALESCE(s.subartist, '')`を追加
- `ChartListItem`構造体に`Subtitle`, `SubArtist`フィールド追加
- `GetSongByFolder`のSQLに`s.subtitle`を追加（subartistは取得済み）

### dto.go

- `ChartListItemDTO`に`Subtitle string`, `SubArtist string`追加
- `ChartDTO`に`Subtitle string`, `SubArtist string`追加

### app.go

- `ListCharts`のDTO変換でsubtitle/subartistをマッピング

## フロントエンド

### ChartListView.svelte（譜面一覧テーブル）

- `ROW_HEIGHT`を32→48に変更（固定2行高さ）
- Titleカラム: セル内を2行構成（タイトル + subtitle小さく表示）
- Artistカラム: セル内を2行構成（アーティスト + subartist小さく表示）
- subtitle/subartistが空の場合も高さは固定（空行のまま）

### ChartDetail.svelte（譜面詳細）

- タイトル行の下にsubtitle表示
- アーティスト行の下にsubartist表示

### SongDetail.svelte（楽曲詳細）

- 譜面リスト各行にsubtitle表示（mode/diff/levelの横に追加）

### EntryDetail.svelte（難易度表エントリ詳細）

- 導入済み時の譜面情報セクションにsubtitle/subartist表示

## データフロー

1. songdata.song テーブルからsubtitle/subartistを取得
2. ドメインモデル（Chart）経由でDTOに変換
3. Wailsバインディング経由でフロントエンドに渡す
4. 各Svelteコンポーネントで表示
