# 譜面一覧ビュー デザインドキュメント

## 概要

songdata.dbの全譜面を個別表示する「譜面一覧」タブを追加する。
既存の「楽曲一覧」（フォルダグループ単位）に対し、こちらは個別譜面レベルの一覧。
DifficultyTableViewの構造をベースに、SongTableのカラム構成を適用する。

## タブ構成

「楽曲一覧」「譜面一覧」「難易度表」の3タブ。CSS hidden方式で状態保持。

## バックエンド

### songdata_reader.go

`ListAllCharts()` メソッドを追加。songdata.songから全レコードを個別取得し、chart_meta（elsa.db）をLEFT JOINしてIR情報を付与。

### app.go

`ListCharts()` Wailsバインディングを追加。songReaderのListAllChartsを呼びDTOに変換して返却。

## フロントエンド

### ChartListView.svelte（新規）

- DifficultyTableViewと同構造（Tanstack + virtualizer + ヘッダーソート）
- ドロップダウンなし（全譜面表示）
- カラム8列: Title(300px), Artist(200px), Genre(140px), BPM(100px), Difficulty(80px), Event(140px), Year(60px), IR(40px)
- BPM表示: min≠maxなら「min-max」、同じなら単一値
- 行クリックでChartDetail表示、同じ行再クリックでトグル

### App.svelte

- `activeTab: 'songs' | 'charts' | 'difficulty'`
- 譜面一覧タブもCSS hidden方式
- 下ペイン: ChartDetailコンポーネント流用（md5で取得、ステータス表示なし）

## データフロー

1. 譜面一覧タブ表示時に `ListCharts()` を呼び全譜面取得
2. フロント側でTanstackソート・仮想スクロール
3. 行クリック → md5で `GetChartDetailByMD5()` → ChartDetailに表示
