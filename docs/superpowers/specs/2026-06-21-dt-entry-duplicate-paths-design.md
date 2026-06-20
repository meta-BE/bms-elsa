# 難易度表エントリ詳細: 重複時の全ファイルパス列挙

## 背景・課題

難易度表画面の詳細パネル（`EntryDetail.svelte`）は、`status === 'duplicate'`（同一 md5 が songdata.db に複数行）でも、表示する詳細情報・「フォルダを開く」ボタンが1つしかない。

原因は `ChartHandler.GetChartDetailByMD5(md5, '')` が内部で `SongdataReader.GetChartByMD5` を呼び、`WHERE md5 = ? LIMIT 1` で1件だけ返すため。重複している場合でもユーザーは1つのフォルダしか開けない。

重複検知の判定自体は `DifficultyTableHandler` の `CountChartsByMD5s` による件数（`count > 1` で `duplicate`）で正しく行われている。

## 目的

重複時に、同一 md5 を持つ全ての譜面ファイルのフルパスを列挙し、それぞれにフォルダを開く UI を付ける。

## スコープ

### 対象
- バックエンド: 指定 md5 の全パスを取得する Reader メソッドとハンドラ公開
- フロントエンド: `EntryDetail.svelte` の重複時表示

### 対象外
- `status` 判定ロジック（`CountChartsByMD5s`）は変更しない
- IR / BMS Search / 譜面メタ表示は md5 単位で同一のため1セットのまま変更しない
- 重複検知タブ（`DuplicateView` / `DuplicateDetail`）は対象外

## 設計

### バックエンド

#### `SongdataReader.ListChartPathsByMD5`
`internal/adapter/persistence/songdata_reader.go` に追加。

```go
// ListChartPathsByMD5 は指定md5を持つ全譜面のパスを返す（重複時の全パス列挙用）
func (r *SongdataReader) ListChartPathsByMD5(ctx context.Context, md5 string) ([]string, error)
```

- クエリ: `SELECT path FROM songdata.song WHERE md5 = ? ORDER BY path`
- `ORDER BY path` で表示順を安定させる
- 行がなければ空スライスを返す

#### `ChartHandler.ListChartPathsByMD5`
`internal/app/chart_handler.go` に追加。`EntryDetail.svelte` はすでに `ChartHandler` の関数を import しているため、ここに公開するのが自然。

```go
func (h *ChartHandler) ListChartPathsByMD5(md5 string) ([]string, error)
```

Reader をそのまま呼び、`[]string` を返す。Wails バインディングが再生成される。

### フロントエンド（`EntryDetail.svelte`）

- `entryData.status === 'duplicate'` のときだけ、新バインディング `ListChartPathsByMD5(md5)` で全パスを取得し `dupPaths: string[]` に保持する。`loadEntry` 内で取得し、非 duplicate 時は空配列にリセット。
- 「ファイルパス一覧」セクションを追加（`ChartInfoCard` の前）。各行 = フルパス文字列（`break-all`）＋ `OpenFolderButton`（その行のフルパスを `path` に渡す）。`OpenFolder` はファイルパスから親フォルダを開く実装のためパス変換は不要。
- ヘッダー右の単一 `OpenFolderButton` は、重複時は非表示にする（一覧と重複するため）。単一導入時は従来どおり1つ表示。閉じるボタンは常に表示。

#### 表示イメージ（重複時）

```
[タイトル / アーティスト]
Lv. X  [重複]                    [×]
─────────────────────────────────
ファイルパス一覧 (3件)
📁 /path/to/folderA/song.bms
📁 /path/to/folderB/song.bme
📁 /path/to/folderC/song.bms
```

各行先頭のフォルダアイコンが `OpenFolderButton`。

## テスト

- `SongdataReader.ListChartPathsByMD5`: 既存の persistence テストにならい、重複あり/単一/該当なしの3ケースを確認するテストを追加（テスト用 songdata.db を利用）。
- フロントは手動確認（`wails dev`）で重複エントリ選択時に全パスが列挙され、各ボタンで対応フォルダが開くこと、単一導入エントリでは従来通り1ボタンであることを確認。

## マニュアル

`docs/manual.md` の難易度表セクションに、重複時に全インストール先フォルダを開ける旨を反映する（該当箇所があれば更新）。
