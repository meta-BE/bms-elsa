# 導入先推定機能 設計

## 概要

難易度表の未導入譜面について、同じ楽曲の導入済み譜面を検索し、インストール先フォルダを推定する機能。

## 目的

未導入譜面がどこにインストールされるべきかを、既存の導入済み譜面の情報から推定する。
同じ楽曲名やLR2IR本体URLが一致する譜面のフォルダパスを表示し、そのフォルダを開けるようにする。

## マッチング基準

2つの基準を組み合わせて検索し、結果を統合表示する：

1. **タイトル完全一致**（大文字小文字無視）: `LOWER(songdata.song.title) = LOWER(エントリtitle)`
2. **LR2IR本体URL一致**: `chart_meta.lr2ir_body_url` が同じ譜面をsongdata.songと突合

部分一致は誤マッチが多いため採用しない。

## 重複排除

`folder`（楽曲フォルダ）単位で重複排除する。同じフォルダに複数譜面がある場合は1候補にまとめる。

## バックエンド

### ドメインモデル（model/song.go に追加）

```go
type InstallCandidate struct {
    FolderPath string
    Title      string
    Artist     string
    MatchTypes []string // "title", "body_url"
}
```

### リポジトリメソッド（SongRepository に追加）

```go
FindChartsByTitle(ctx context.Context, title string) ([]Chart, error)
FindChartsByBodyURL(ctx context.Context, bodyURL string) ([]Chart, error)
```

- `FindChartsByTitle`: `LOWER(title) = LOWER(?)` で完全一致検索、`GROUP BY folder` でフォルダ単位に集約
- `FindChartsByBodyURL`: `chart_meta.lr2ir_body_url = ?` で検索し、songdata.songと突合。インデックスなし（レコード数が少ないためフルスキャンで十分）

### ユースケース（usecase/estimate_install_location.go）

```go
type EstimateInstallLocationUsecase struct {
    songRepo model.SongRepository
    metaRepo model.MetaRepository
}

func (u *EstimateInstallLocationUsecase) Execute(ctx context.Context, title string, md5 string) ([]model.InstallCandidate, error)
```

処理フロー:
1. titleでsongdata.songを完全一致検索（大文字小文字無視）→ folder単位で集約
2. md5でchart_metaからbody_urlを取得
3. body_urlが空でなければ、同じbody_urlを持つ導入済み譜面を検索 → folder単位で集約
4. 両方の結果をfolder単位でマージ（matchTypesを統合）
5. 入力md5自身は除外

### ハンドラーメソッド（DifficultyTableHandler に追加）

```go
func (h *DifficultyTableHandler) EstimateInstallLocation(md5 string, tableID int) ([]dto.InstallCandidateDTO, error)
```

### DTO（dto/dto.go に追加）

```go
type InstallCandidateDTO struct {
    FolderPath string   `json:"folderPath"`
    Title      string   `json:"title"`
    Artist     string   `json:"artist"`
    MatchTypes []string `json:"matchTypes"`
}
```

## フロントエンド

### 新規コンポーネント: InstallCandidateCard.svelte

- `components/InstallCandidateCard.svelte` として新設
- props: `md5: string`, `tableID: number`
- マウント時に `EstimateInstallLocation(md5, tableID)` を呼び出し
- 候補ごとにタイトル、アーティスト、フォルダパス、マッチ理由バッジを表示
- フォルダを開くボタン付き

### EntryDetail.svelte への組み込み

未導入時（`!chart`）にChartInfoCardの代わりに表示:

```
エントリ基本情報カード
↓
InstallCandidateCard（未導入時のみ）
↓
IRInfoCard
```
