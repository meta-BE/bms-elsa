# 導入先推定ロジック改善 設計

## 概要

現行の導入先推定は「タイトル完全一致」と「LR2IR本体URL一致」の2基準だが、
難易度表エントリのtitleにはsubtitle相当の接尾辞（`[闇]`, `(集0)`, `-Despair-`等）が含まれるため、
完全一致では導入済み楽曲を見つけられないケースが多い。

例: エントリ `影縫 [闇]` → DB title `影縫` → 完全一致しない

## 改善方針

複数のマッチング手法を組み合わせ、スコアリングで結果をランク付けする。

## マッチング手法

| # | 手法 | スコア | 説明 |
|---|------|--------|------|
| 1 | タイトル完全一致 | 3 | 現行ロジック。`LOWER(entry.title) = LOWER(song.title)` |
| 2 | ベースタイトル一致 | 2 | エントリtitleから末尾の接尾辞を除去し、DB titleと完全一致 |
| 3 | body_url一致 | 3 | 現行ロジック。chart_meta.lr2ir_body_urlが同じ譜面を突合 |
| 4 | アーティスト一致 | 1 | `LOWER(entry.artist) = LOWER(song.artist)`。単独では弱いが加算要員 |

### ベースタイトル抽出

エントリtitleの末尾から以下のパターンを繰り返し除去してtrimする：
- `[...]` — 差分名（例: `[闇]`, `[INSANE]`）
- `(...)` — 補足情報（例: `(集0)`, `(・ω・)`）
- `-...-` — サブタイトル（例: `-Despair-`, `-Eclipse-`）

例: `影縫 [闇] (集0)` → `影縫`

元titleと同じ場合は検索をスキップ（タイトル完全一致と重複するため）。

## スコアリング

- 同じfolderに対して複数手法がマッチした場合、スコアを合算
- matchTypesにマッチした手法をすべて含める（`"title"`, `"base_title"`, `"body_url"`, `"artist"`）
- 結果はスコア降順でソート

## バックエンド変更

### ドメインモデル（model/song.go）

InstallCandidateにScoreフィールドを追加：

```go
type InstallCandidate struct {
    FolderPath string
    Title      string
    Artist     string
    MatchTypes []string // "title", "base_title", "body_url", "artist"
    Score      int
}
```

### リポジトリ（SongRepository）

1メソッドを追加：

```go
FindChartFoldersByArtist(ctx context.Context, artist string) ([]InstallCandidate, error)
```

ベースタイトル検索にはFindChartFoldersByTitleを再利用（ユースケース側で接尾辞除去した値を渡す）。

### ユースケース（estimate_install_location.go）

処理フロー：
1. エントリtitleでタイトル完全一致検索 → スコア3
2. エントリtitleからベースタイトルを抽出、元と異なる場合のみ検索 → スコア2
3. md5からbody_urlを取得、body_url一致検索 → スコア3
4. エントリartistでアーティスト一致検索 → スコア1
5. folder単位でマージ（スコア合算、matchTypes統合）
6. スコア降順でソート

ベースタイトル抽出関数をユースケース内に追加。

### DTO（dto/dto.go）

InstallCandidateDTOにScoreフィールドを追加。

### ハンドラー（difficulty_table_handler.go）

EstimateInstallLocationメソッドにartist引数の追加が必要。
エントリからtitleとartistの両方を取得してユースケースに渡す。

## フロントエンド変更

InstallCandidateCard.svelteのmatchLabel関数に新しいmatchType対応を追加：
- `"base_title"` → `"タイトル類似"`
- `"artist"` → `"アーティスト一致"`
