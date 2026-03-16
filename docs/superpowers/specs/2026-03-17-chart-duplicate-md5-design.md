# 同一MD5譜面の重複表示バグ修正 設計書

## 問題

songdata.dbに同一MD5の譜面が異なるフォルダに存在する場合（同じBMSファイルが複数のイベントパックに含まれるケース）、譜面一覧で複数行表示されるが、どちらをクリックしても同じ詳細（最初にヒットした方）が表示される。

### 根本原因

- `ListAllCharts` は `songdata.song` を `song_meta` に `folder_hash` でLEFT JOINするため、異なるフォルダの同一MD5は異なる `event_name`/`release_year` を持つ別々の行として返される
- `GetChartByMD5` は `WHERE md5 = ? LIMIT 1` で検索するため、常に最初の1件が返る
- フロントエンドは `md5` のみを選択キーとして渡すため、どちらの行をクリックしても同じAPIリクエストになる

### 例

MD5 `9188a4c9876386173ba35158edf23a15` (#B2FFFF [SP Celeste Colored Strawberry]):
- folder `8a50833a` → BOFTT2024 (song_meta: BOF:TT/2024)
- folder `32d035d6` → Astronomical Twilight (song_meta: BOFU2015/2015)

## 方針

folderHashを選択キーに追加し、詳細取得時にMD5+folderHashで特定する。一覧の表示は変更しない（EVENT/YEARカラムの差異で区別可能）。

## 変更箇所

### バックエンド

#### 1. `internal/adapter/persistence/songdata_reader.go`

**`ChartListItem` 構造体**（同ファイル395行付近に定義）に `FolderHash string` フィールドを追加。

**`ListAllCharts`**: SELECTに `s.folder` を追加し、Scan引数に `&c.FolderHash` を追加。

**`GetChartByMD5(ctx, md5)` → `GetChartByMD5(ctx, md5, folderHash)`**:
- `folderHash` が空でない場合: `WHERE md5 = ? AND folder = ?`（LIMIT不要、一意に特定される）
- `folderHash` が空の場合: 既存通り `WHERE md5 = ? LIMIT 1`（難易度表タブ等の互換性維持）

注: `model.Chart` にはFolderHashフィールドを追加しない。folderHashは選択キーとしてのみ使用し、詳細表示には不要。

#### 2. `internal/app/dto/dto.go`

`ChartListItemDTO` に `FolderHash string` フィールドを追加（json tag: `"folderHash"`）。

#### 3. `internal/app/chart_handler.go`

**`ListCharts`**: `ChartListItem.FolderHash` → `ChartListItemDTO.FolderHash` のマッピングを追加。

**`GetChartDetailByMD5(md5)` → `GetChartDetailByMD5(md5, folderHash string)`**: `folderHash` をそのまま `songReader.GetChartByMD5` に渡す。Goシグネチャ変更により、Wailsが `frontend/wailsjs/go/app/ChartHandler.{js,d.ts}` と `models.ts` を自動再生成する。

### フロントエンド

#### 4. `frontend/src/views/ChartListView.svelte`

- `selected` propの型は `string | null` のまま維持。App.svelte側で `md5 + ':' + folderHash` の結合文字列を渡す
- `dispatch` の型を `{ md5: string; folderHash: string }` に変更
- `handleRowClick` で `{ md5: chart.md5, folderHash: chart.folderHash }` をdispatch
- `handleArrowNav` の `onSelect` コールバックも `{ md5: o.md5, folderHash: o.folderHash }` をdispatchするよう変更
- `getKey` を `(o) => o.md5 + ':' + o.folderHash` に変更（同一MD5の行を区別）
- `selected` の行ハイライト比較を `selected === o.md5 + ':' + o.folderHash` で行う

#### 5. `frontend/src/views/ChartDetail.svelte`

- `folderHash` propを追加
- `GetChartDetailByMD5(md5)` → `GetChartDetailByMD5(md5, folderHash)` に変更
- リアクティブ文を `$: if (md5) loadChart(md5)` から `$: chartKey = md5 + ':' + folderHash; $: if (chartKey) loadChart(md5, folderHash)` に変更。同一MD5で異なるfolderHashの行を切り替えた場合にmd5だけでは変化を検知できないため、結合キーをリアクティブの依存にする

#### 6. `frontend/src/views/EntryDetail.svelte`

- `GetChartDetailByMD5(hash)` → `GetChartDetailByMD5(hash, '')` に変更（難易度表タブからの呼び出しはfolderHashを持たないため空文字を渡す）

#### 7. `frontend/src/App.svelte`

- チャート選択状態を `selectedChartMd5: string | null` から `selectedChart: { md5: string; folderHash: string } | null` に変更
- `handleChartSelect` のトグル比較を `selectedChart?.md5 === e.detail.md5 && selectedChart?.folderHash === e.detail.folderHash` で行う
- ChartListViewへの `selected` propは `selectedChart ? selectedChart.md5 + ':' + selectedChart.folderHash : null` の結合文字列を渡す
- ChartDetailへは `md5={selectedChart?.md5}` と `folderHash={selectedChart?.folderHash ?? ''}` を渡す
- `SplitPane` の `showDetail` 条件を `!!selectedChart` に変更

## 影響範囲

- **譜面一覧タブ**: 選択キーの変更（md5 → md5+folderHash）
- **難易度表タブ**: `EntryDetail.svelte` が `GetChartDetailByMD5(hash, '')` に変更（空文字フォールバックで既存動作維持）
- **その他のタブ**: 影響なし

## テスト

- 同一MD5が2フォルダに存在するデータで、一覧の各行クリック時にそれぞれ正しいフォルダの詳細（パスが異なる）が表示されること
- `EntryDetail.svelte`（難易度表タブ）からfolderHash空文字で呼んだ場合に既存通り `LIMIT 1` で動作すること
- 通常の譜面（重複なし）の選択・詳細表示に影響がないこと
- 同一MD5の2行それぞれの選択・選択解除（トグル）が独立して動作すること
