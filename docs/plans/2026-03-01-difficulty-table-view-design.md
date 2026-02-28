# 難易度表譜面一覧ビュー設計

## 概要

楽曲一覧とタブで切り替え可能な難易度表の譜面一覧画面を追加する。
登録済み難易度表をドロップダウンで切り替え、選択した難易度表の全譜面をテーブル表示する。
songdata.dbとmd5で照合し、導入済み/未導入/重複の3状態を行の背景色で区別する。

## アプローチ

**独立コンポーネント方式**を採用。SongTable/SongDetailとは独立した`DifficultyTableView`/`ChartDetail`を新規作成する。

理由: 楽曲一覧は楽曲単位グルーピング+ページネーション、難易度表は譜面単位フラットリストと性質が異なるため、独立させた方が素直。tanstack-tableのボイラープレートの重複はあるが、カラム定義・行クリック・背景色ロジックなど差異が大きく共通化の恩恵は薄い。

## バックエンドAPI

### 新規API

#### `ListDifficultyTableEntries(tableID int64) []DifficultyTableEntryWithStatus`

- 指定テーブルの全エントリを取得
- 各エントリのmd5でsongdata.dbを照合し、ステータスを付与
  - `installed`: md5が1件一致
  - `not_installed`: md5が0件
  - `duplicate`: md5が2件以上一致
- レスポンスDTO: `{ md5, level, title, artist, url, urlDiff, status, installedCount }`

#### `GetChartDetailByMD5(md5 string) ChartDetailDTO`

- songdata.dbからmd5で譜面情報を取得（メタ情報 + IR情報 + 難易度ラベル）
- 未導入の場合はエラーまたは空レスポンスを返し、フロントエンドが難易度表エントリ情報でフォールバック

### 既存APIの変更

なし。`ListDifficultyTables()`はドロップダウン用にそのまま使用。

### md5照合の実装

- Repository層に `CountChartsByMD5s(md5s []string) map[string]int` を追加
- App層の `ListDifficultyTableEntries` 内でエントリ全md5を一括照合（N+1回避）

## フロントエンド構成

### タブ切り替え（App.svelte）

- 上ペイン上部にタブバーを追加: 「楽曲一覧」「難易度表」
- タブ切り替えで上ペイン・下ペインの表示コンポーネントを一括切り替え
  - 楽曲一覧タブ → SongTable + SongDetail
  - 難易度表タブ → DifficultyTableView + ChartDetail

### DifficultyTableView.svelte（上ペイン）

- 上部: ドロップダウンで登録済み難易度表を選択
- 本体: tanstack-table + 仮想スクロールで譜面一覧表示
- カラム: Level, Title, Artist, URL有無, 導入ステータス
- 行の背景色:
  - 導入済み: デフォルト
  - 未導入: グレー系
  - 重複: 黄色系
- 行クリックで選択 → 下ペインに詳細表示

### ChartDetail.svelte（下ペイン）

- 導入済み/重複の場合: `GetChartDetailByMD5()`で取得した譜面メタ情報 + IR情報 + 難易度ラベル + url/url_diff
- 未導入の場合: 「未導入です」メッセージ + 難易度表エントリ情報（title, artist, level, url, url_diff）

### 状態管理

- `selectedTable`: writable store（選択中の難易度表ID）
- `selectedEntryMD5`: writable store（選択中のエントリのmd5）
- タブ切り替え時に各ビューの選択状態はリセット

## データフロー

```
難易度表タブ選択
  → ListDifficultyTables() でドロップダウン候補を取得
  → ユーザーが難易度表を選択
  → ListDifficultyTableEntries(tableID) で譜面一覧+ステータスを取得
  → テーブル描画（行の背景色をステータスで制御）
  → ユーザーが行をクリック
  → status が installed/duplicate → GetChartDetailByMD5(md5) で詳細取得
  → status が not_installed → 難易度表エントリ情報のみ表示
```

## エラーハンドリング

- 難易度表が未登録 → 「Settings画面から難易度表を追加してください」メッセージ
- 難易度表のエントリが0件 → 「エントリがありません。更新してください」メッセージ
- GetChartDetailByMD5で取得失敗 → 難易度表エントリ情報でフォールバック表示

## スコープ外

- 難易度表の譜面一覧からの直接ダウンロード機能
- 譜面一覧のフィルタリング・検索
- 難易度表エントリのソート以外の操作
