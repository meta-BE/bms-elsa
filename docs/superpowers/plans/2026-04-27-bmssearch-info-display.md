# BMS Search 情報表示機能 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** BMS Search 由来の楽曲メタデータ（イベント・公開日・DLリンク・プレビュー等）を、楽曲詳細・譜面詳細・難易度表エントリ詳細の各画面に表示し、「取得」「解除」操作を提供する。

**Architecture:** ドメイン層に `BMSSearchBMS`/`BMSSearchLink` エンティティと `BMSSearchRepository` を追加し、永続化層で `bmssearch_bms_id_md5` / `bmssearch_bms` テーブルを管理。共通の Resolve+Persist ロジックを `BMSSearchResolver` に集約し、`LookupBMSSearchUseCase`・`UnlinkBMSSearchUseCase`・改修した `SyncBMSSearchUseCase` から委譲する。フロントは `BMSSearchInfoCard.svelte` を3詳細画面に配置する。

**Tech Stack:** Go 1.x（標準 `database/sql`, `modernc.org/sqlite`）、Wails v2、Svelte + TypeScript + DaisyUI/Tailwind

**設計ドキュメント:** `docs/superpowers/specs/2026-04-27-bmssearch-info-display-design.md`

---

## ファイル構成

### 新規作成
- `cmd/probe-bmssearch/main.go` — フォールバック検索仕様確定用の調査スクリプト（Task 1）
- `internal/domain/model/bmssearch.go` — `BMSSearchBMS`/`BMSSearchLink`/`BMSSearchSource`/`BMSSearchRepository` 定義
- `internal/adapter/persistence/bmssearch_repository.go` — `bmssearch_bms_id_md5` / `bmssearch_bms` テーブルの CRUD
- `internal/adapter/persistence/bmssearch_repository_test.go`
- `internal/usecase/bmssearch_scoring.go` — title/artist 正規化・スコアリング（pure functions）
- `internal/usecase/bmssearch_scoring_test.go`
- `internal/usecase/bmssearch_resolver.go` — 公式/フォールバック解決の共通ロジック
- `internal/usecase/bmssearch_resolver_test.go`
- `internal/usecase/lookup_bmssearch.go` — 詳細画面「取得」ボタン用
- `internal/usecase/lookup_bmssearch_test.go`
- `internal/usecase/unlink_bmssearch.go` — 「解除」ボタン用
- `internal/usecase/unlink_bmssearch_test.go`
- `internal/app/bmssearch_handler.go` — Wails ハンドラー
- `frontend/src/components/BMSSearchInfoCard.svelte` — UI コンポーネント

### 変更
- `internal/adapter/persistence/migrations.go` — マイグレーション追加
- `internal/adapter/persistence/migrations_test.go` — マイグレーションテスト追加
- `internal/adapter/persistence/elsa_repository.go` — `bms_search_source` 対応
- `internal/adapter/gateway/bmssearch_client.go` — `BMSSearchBMS` 構造体拡張、`SearchBMSesByTitle` 追加
- `internal/adapter/gateway/bmssearch_client_test.go` — 新メソッドのテスト
- `internal/domain/model/song.go` — `SongMeta.BMSSearchSource` 追加
- `internal/domain/model/repository.go` — `MetaRepository` 拡張
- `internal/usecase/sync_bmssearch.go` — Resolver 委譲に改修
- `internal/usecase/usecase_test.go` — モック更新
- `internal/app/dto/dto.go` — `BMSSearchInfoDTO` ほか追加
- `app.go` — DI 組み立て更新
- `main.go` — Wails Bind に `BMSSearchHandler` 追加
- `frontend/src/components/icons.ts` — `search` アイコン追加
- `frontend/src/views/SongDetail.svelte`
- `frontend/src/views/ChartDetail.svelte`
- `frontend/src/views/EntryDetail.svelte`
- `docs/manual.md`
- `docs/superpowers/specs/2026-04-27-bmssearch-info-display-design.md` — Task 2 でフォールバック検索仕様を確定後に更新

---

## Phase 1: 事前調査スパイク

### Task 1: BMS Search フォールバック検索の調査スクリプト作成

**Files:**
- Create: `cmd/probe-bmssearch/main.go`
- Create: `docs/superpowers/specs/data/bmssearch-probe/2026-04-27/` （JSON保存先ディレクトリ。実行時に自動作成）

- [ ] **Step 1: スクリプトの土台を作成**

`cmd/probe-bmssearch/main.go` を新規作成:

```go
// Package main は BMS Search API のフォールバック検索仕様を確定するための調査スクリプト
// 使い方:
//   go run ./cmd/probe-bmssearch -songdata path/to/songdata.db -out docs/superpowers/specs/data/bmssearch-probe/YYYY-MM-DD
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const baseURL = "https://api.bmssearch.net/v1"

type sample struct {
	MD5    string
	Title  string
	Artist string
}

type queryResult struct {
	Variant   string          `json:"variant"`
	Query     string          `json:"query"`
	HTTPCode  int             `json:"httpCode"`
	Count     int             `json:"count"`
	Items     json.RawMessage `json:"items"`
	FetchedAt string          `json:"fetchedAt"`
}

func main() {
	var songdataPath, outDir string
	flag.StringVar(&songdataPath, "songdata", "testdata/songdata.db", "songdata.db path")
	flag.StringVar(&outDir, "out", "docs/superpowers/specs/data/bmssearch-probe/probe", "output dir for JSON dumps")
	flag.Parse()

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	hits, misses, err := pickSamples(songdataPath)
	if err != nil {
		log.Fatal(err)
	}
	all := append(hits, misses...)

	for _, s := range all {
		variants := buildQueryVariants(s.Title)
		for _, v := range variants {
			res := fetchSearch(v.query)
			res.Variant = v.label
			fname := filepath.Join(outDir, fmt.Sprintf("%s_%s.json", s.MD5[:8], v.label))
			writeJSON(fname, map[string]any{
				"sample": s,
				"result": res,
			})
		}
	}
	fmt.Printf("done. %d samples × variants written to %s\n", len(all), outDir)
}

type variant struct{ label, query string }

func buildQueryVariants(title string) []variant {
	return []variant{
		{label: "raw", query: title},
		{label: "normalized", query: normalizeTitle(title)},
		{label: "stripped", query: stripTrailingBrackets(title)},
	}
}

// 暫定実装: 設計ドキュメントの初期案に沿った正規化（実調査結果で更新する）
func normalizeTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	// TODO: 全半角統一・記号除去は調査結果を踏まえて拡充
	return s
}

func stripTrailingBrackets(s string) string {
	s = strings.TrimSpace(s)
	for {
		trimmed := s
		// 末尾の [...] / (...) / -...- を1段階ずつ剥離
		for _, pair := range [][2]string{{"[", "]"}, {"(", ")"}, {"-", "-"}} {
			if strings.HasSuffix(trimmed, pair[1]) {
				idx := strings.LastIndex(trimmed, pair[0])
				if idx > 0 {
					trimmed = strings.TrimSpace(trimmed[:idx])
				}
			}
		}
		if trimmed == s {
			break
		}
		s = trimmed
	}
	return s
}

func fetchSearch(title string) queryResult {
	q := url.Values{}
	q.Set("title", title)
	q.Set("limit", "20")
	q.Set("orderBy", "PUBLISHED")
	q.Set("orderDirection", "DESC")
	u := baseURL + "/bmses/search?" + q.Encode()
	time.Sleep(150 * time.Millisecond)
	resp, err := http.Get(u)
	if err != nil {
		return queryResult{Query: title, HTTPCode: -1, FetchedAt: time.Now().Format(time.RFC3339)}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var arr []json.RawMessage
	_ = json.Unmarshal(body, &arr)
	return queryResult{
		Query:     title,
		HTTPCode:  resp.StatusCode,
		Count:     len(arr),
		Items:     body,
		FetchedAt: time.Now().Format(time.RFC3339),
	}
}

// pickSamples は公式md5ヒット5件・ミス5件を選ぶ
// ヒット判定: bms_search_id が song_meta に保存されているもの
// ミス判定: song_meta なし or bms_search_id IS NULL
func pickSamples(songdataPath string) (hits, misses []sample, err error) {
	db, err := sql.Open("sqlite", songdataPath)
	if err != nil {
		return nil, nil, err
	}
	defer db.Close()
	ctx := context.Background()

	hitRows, err := db.QueryContext(ctx, `
		SELECT s.md5, s.title, s.artist
		FROM song s
		ORDER BY s.md5
		LIMIT 5`)
	if err != nil {
		return nil, nil, err
	}
	defer hitRows.Close()
	for hitRows.Next() {
		var s sample
		if err := hitRows.Scan(&s.MD5, &s.Title, &s.Artist); err != nil {
			return nil, nil, err
		}
		hits = append(hits, s)
	}

	missRows, err := db.QueryContext(ctx, `
		SELECT s.md5, s.title, s.artist
		FROM song s
		ORDER BY s.md5 DESC
		LIMIT 5`)
	if err != nil {
		return nil, nil, err
	}
	defer missRows.Close()
	for missRows.Next() {
		var s sample
		if err := missRows.Scan(&s.MD5, &s.Title, &s.Artist); err != nil {
			return nil, nil, err
		}
		misses = append(misses, s)
	}
	return hits, misses, nil
}

func writeJSON(path string, v any) {
	data, _ := json.MarshalIndent(v, "", "  ")
	_ = os.WriteFile(path, data, 0o644)
}
```

- [ ] **Step 2: ビルド確認**

実行: `go build ./cmd/probe-bmssearch`
期待: ビルド成功、ルートに `probe-bmssearch` バイナリが残らないこと（残ったら削除する）

- [ ] **Step 3: 調査スクリプトを実行**

実行: `go run ./cmd/probe-bmssearch -songdata testdata/songdata.db -out docs/superpowers/specs/data/bmssearch-probe/2026-04-27`
期待: 30件前後の JSON が `docs/superpowers/specs/data/bmssearch-probe/2026-04-27/` に出力される

- [ ] **Step 4: 出力 JSON を目視確認しながら調査レポート作成**

`docs/superpowers/specs/2026-04-27-bmssearch-info-fallback-probe.md` を新規作成。調査内容（雛形）:

```markdown
# BMS Search フォールバック検索 調査レポート（2026-04-27実施）

## サンプル選定基準
- 公式md5ヒット5件: `song_meta.bms_search_id IS NOT NULL` の楽曲を md5 順に取得
- ミス5件: `song_meta.bms_search_id IS NULL` の楽曲を md5 降順に取得

## 末尾付帯文字列パターン
（出力JSONを見て列挙: 例 "[ANBMS]", "(BMS版)", "-PMS-" 等）

## 正規化ルール最終案
- ケース折りたたみ: ASCII 大小無視
- 全半角統一: NFKC 適用
- 記号除去: ` `（空白）, `~`, `!` 等を除去
- 末尾装飾剥離: `[...]`, `(...)`, `-...-` をループで除去

## スコア配点最終案
（初期案からの変更があれば記述）

| 項目 | 配点 |
|---|---|
| title 完全一致 | +60 |
| title 正規化後完全一致 | +50 |
| title 部分一致 | +25 |
| artist 完全一致 | +20 |
| artist 正規化後完全一致 | +15 |
| artist トークン共通率 × 10 | 0〜+10 |

## 閾値最終値
- 採用閾値: 50
- 同点首位の取り扱い: 採用しない（保留）

## top1 採用率と誤紐付け率の見積もり
- 公式ヒット5件のうち X 件で正解 top1
- ミス5件のうち、誤紐付けが Y 件、unofficial 採用妥当が Z 件

## 結論
（設計ドキュメントの「フォールバック検索の正規化・スコアリング」セクションをこの内容で確定する）
```

- [ ] **Step 5: 設計ドキュメントを更新**

`docs/superpowers/specs/2026-04-27-bmssearch-info-display-design.md` の「フォールバック検索の正規化・スコアリング（暫定）」セクションを、調査レポートの結論で書き換える（タイトル末尾の「（暫定）」を削除）。

- [ ] **Step 6: コミット**

```bash
git add cmd/probe-bmssearch/ docs/superpowers/specs/data/bmssearch-probe/ docs/superpowers/specs/2026-04-27-bmssearch-info-fallback-probe.md docs/superpowers/specs/2026-04-27-bmssearch-info-display-design.md
git commit -m "spike: BMS Search フォールバック検索仕様の事前調査"
```

---

## Phase 2: マイグレーション・ドメインエンティティ・リポジトリ

### Task 2: マイグレーション追加

**Files:**
- Modify: `internal/adapter/persistence/migrations.go`
- Modify: `internal/adapter/persistence/migrations_test.go`

- [ ] **Step 1: 失敗するテストを追加**

`internal/adapter/persistence/migrations_test.go` の末尾に追加:

```go
func TestRunMigrations_BMSSearchSchema(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := persistence.RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// song_meta.bms_search_source カラムが追加されている
	var hasSource int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('song_meta') WHERE name='bms_search_source'`,
	).Scan(&hasSource); err != nil {
		t.Fatal(err)
	}
	if hasSource != 1 {
		t.Errorf("song_meta.bms_search_source should exist, got %d", hasSource)
	}

	// bmssearch_bms_id_md5 テーブルが存在する
	var c int
	if err := db.QueryRow(`SELECT COUNT(*) FROM bmssearch_bms_id_md5`).Scan(&c); err != nil {
		t.Errorf("bmssearch_bms_id_md5 table not found: %v", err)
	}

	// bmssearch_bms テーブルが存在する
	if err := db.QueryRow(`SELECT COUNT(*) FROM bmssearch_bms`).Scan(&c); err != nil {
		t.Errorf("bmssearch_bms table not found: %v", err)
	}
}

func TestRunMigrations_BMSSearchSourceBackfill(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// 旧スキーマ相当: 先にマイグレーションを実行してから bms_search_source カラムを削除
	if err := persistence.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	// シミュレーション: bms_search_source を一旦消して bms_search_id だけ入れる
	if _, err := db.Exec(`UPDATE song_meta SET bms_search_source = NULL`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO song_meta (folder_hash, bms_search_id) VALUES ('h1', 'bms-1')`); err != nil {
		t.Fatal(err)
	}

	// 再度マイグレーション → backfill ロジックで 'official' が入る
	if err := persistence.RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	var src sql.NullString
	if err := db.QueryRow(`SELECT bms_search_source FROM song_meta WHERE folder_hash='h1'`).Scan(&src); err != nil {
		t.Fatal(err)
	}
	if !src.Valid || src.String != "official" {
		t.Errorf("expected bms_search_source='official', got %v", src)
	}
}
```

- [ ] **Step 2: テストを実行して失敗を確認**

実行: `go test ./internal/adapter/persistence/ -run TestRunMigrations_BMSSearch -v`
期待: FAIL（カラム/テーブルがまだ存在しない）

- [ ] **Step 3: マイグレーション実装**

`internal/adapter/persistence/migrations.go` の `RunMigrations` 関数の `statements` スライスの末尾（`url_rewrite_rule` の後）に追加:

```go
		`CREATE TABLE IF NOT EXISTS bmssearch_bms_id_md5 (
			md5         TEXT PRIMARY KEY,
			bms_id      TEXT NOT NULL,
			source      TEXT NOT NULL,
			resolved_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_bmssearch_link_bms_id ON bmssearch_bms_id_md5(bms_id)`,
		`CREATE TABLE IF NOT EXISTS bmssearch_bms (
			bms_id              TEXT PRIMARY KEY,
			title               TEXT NOT NULL DEFAULT '',
			artist              TEXT NOT NULL DEFAULT '',
			subartist           TEXT NOT NULL DEFAULT '',
			genre               TEXT NOT NULL DEFAULT '',
			exhibition_id       TEXT,
			exhibition_name     TEXT NOT NULL DEFAULT '',
			published_at        TEXT NOT NULL DEFAULT '',
			downloads_json      TEXT NOT NULL DEFAULT '[]',
			previews_json       TEXT NOT NULL DEFAULT '[]',
			related_links_json  TEXT NOT NULL DEFAULT '[]',
			fetched_at          INTEGER NOT NULL
		)`,
```

そして `RunMigrations` の末尾（最後の `return nil` の前）に追加:

```go
	// song_meta.bms_search_source カラムの追加（冪等）+ 初回バックフィル
	var hasBMSSearchSource int
	_ = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('song_meta') WHERE name='bms_search_source'`).Scan(&hasBMSSearchSource)
	if hasBMSSearchSource == 0 {
		if _, err := db.Exec(`ALTER TABLE song_meta ADD COLUMN bms_search_source TEXT`); err != nil {
			return fmt.Errorf("add bms_search_source: %w", err)
		}
		// 既存の bms_search_id 入りレコードはすべて公式 md5 ヒット起因なので 'official' で埋める
		if _, err := db.Exec(`UPDATE song_meta SET bms_search_source = 'official' WHERE bms_search_id IS NOT NULL`); err != nil {
			return fmt.Errorf("backfill bms_search_source: %w", err)
		}
	}
```

- [ ] **Step 4: テストを実行して成功を確認**

実行: `go test ./internal/adapter/persistence/ -run TestRunMigrations -v`
期待: 全 PASS（既存 + 新規2件）

- [ ] **Step 5: コミット**

```bash
git add internal/adapter/persistence/migrations.go internal/adapter/persistence/migrations_test.go
git commit -m "migration: bmssearch_bms_id_md5/bmssearch_bms テーブルと bms_search_source カラム追加"
```

---

### Task 3: ドメインエンティティ追加

**Files:**
- Create: `internal/domain/model/bmssearch.go`
- Modify: `internal/domain/model/song.go`
- Modify: `internal/domain/model/repository.go`

- [ ] **Step 1: `bmssearch.go` を新規作成**

```go
package model

import (
	"context"
	"time"
)

// BMSSearchSource は md5 ↔ bms_id リンクの確度区分
type BMSSearchSource string

const (
	BMSSearchSourceOfficial   BMSSearchSource = "official"
	BMSSearchSourceUnofficial BMSSearchSource = "unofficial"
)

// BMSSearchLink は md5 と BMS Search の bms_id のリンク
type BMSSearchLink struct {
	MD5        string
	BMSID      string
	Source     BMSSearchSource
	ResolvedAt time.Time
}

// BMSSearchBMS は BMS API の楽曲レスポンスをキャッシュしたエンティティ
type BMSSearchBMS struct {
	BMSID          string
	Title          string
	Artist         string
	SubArtist      string
	Genre          string
	ExhibitionID   *string
	ExhibitionName string
	PublishedAt    string
	Downloads      []BMSSearchURLEntry
	Previews       []BMSSearchPreview
	RelatedLinks   []BMSSearchURLEntry
	FetchedAt      time.Time
}

// BMSSearchURLEntry は URL + 説明のペア（DLリンク・関連リンク用）
type BMSSearchURLEntry struct {
	URL         string
	Description string
}

// BMSSearchPreview は再生プレビュー（YouTube/SoundCloud/NicoNico）
type BMSSearchPreview struct {
	Service   string
	Parameter string
}

// BMSSearchRepository は bmssearch_bms_id_md5 / bmssearch_bms の CRUD
type BMSSearchRepository interface {
	GetLinkByMD5(ctx context.Context, md5 string) (*BMSSearchLink, error)
	UpsertLinks(ctx context.Context, links []BMSSearchLink) error
	DeleteLinkByMD5(ctx context.Context, md5 string) error
	DeleteLinksByMD5s(ctx context.Context, md5s []string) error

	GetBMSByID(ctx context.Context, bmsID string) (*BMSSearchBMS, error)
	UpsertBMS(ctx context.Context, bms BMSSearchBMS) error
}
```

- [ ] **Step 2: `song.go` の `SongMeta` を更新**

`internal/domain/model/song.go:57-62` の `SongMeta` を以下に置換:

```go
// SongMeta は楽曲レベルの追加メタデータ
type SongMeta struct {
	FolderHash        string
	ReleaseYear       *int
	EventID           *string
	BMSSearchID       *string
	BMSSearchSource   *string
}
```

- [ ] **Step 3: `repository.go` の `MetaRepository` を拡張**

`internal/domain/model/repository.go` の `MetaRepository` インターフェースに追加（`UpdateSongMetaEvent` の直後）:

```go
	// BMS Search 紐付け情報（bms_search_id + bms_search_source）のみを更新する。
	// bmsID/source が空文字列の場合は NULL に更新（解除）。
	UpdateSongMetaBMSSearch(ctx context.Context, folderHash, bmsID, source string) error
```

- [ ] **Step 4: ビルド確認**

実行: `go build ./...`
期待: `ElsaRepository` が `MetaRepository` を満たさないというエラーが出る（次タスクで実装）

- [ ] **Step 5: コミット（コンパイルエラー覚悟で進める）**

このコミットは Task 4 と一緒にする。Step 5 は次タスクで完了させるためスキップ。

---

### Task 4: ElsaRepository 拡張（song_meta.bms_search_source 対応）

**Files:**
- Modify: `internal/adapter/persistence/elsa_repository.go`
- Modify: `internal/usecase/usecase_test.go`

- [ ] **Step 1: 失敗するテストを追加**

`internal/adapter/persistence/elsa_repository_test.go` がある場合は同ファイル末尾、なければ新規作成:

```go
func TestUpdateSongMetaBMSSearch(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()

	repo := persistence.NewElsaRepository(db)
	ctx := context.Background()

	// 設定
	if err := repo.UpdateSongMetaBMSSearch(ctx, "folder1", "bms-1", "official"); err != nil {
		t.Fatal(err)
	}
	m, err := repo.GetSongMeta(ctx, "folder1")
	if err != nil {
		t.Fatal(err)
	}
	if m == nil || m.BMSSearchID == nil || *m.BMSSearchID != "bms-1" {
		t.Errorf("BMSSearchID = %v, want bms-1", m)
	}
	if m.BMSSearchSource == nil || *m.BMSSearchSource != "official" {
		t.Errorf("BMSSearchSource = %v, want official", m.BMSSearchSource)
	}

	// 解除（空文字列 → NULL）
	if err := repo.UpdateSongMetaBMSSearch(ctx, "folder1", "", ""); err != nil {
		t.Fatal(err)
	}
	m, _ = repo.GetSongMeta(ctx, "folder1")
	if m.BMSSearchID != nil {
		t.Errorf("BMSSearchID after unlink = %v, want nil", *m.BMSSearchID)
	}
	if m.BMSSearchSource != nil {
		t.Errorf("BMSSearchSource after unlink = %v, want nil", *m.BMSSearchSource)
	}
}
```

`newTestDB` ヘルパーが既にある場合はそれを使う。なければ既存のテスト（例: `difficulty_table_repository_test.go`）からパターンをコピーすること。

- [ ] **Step 2: テスト実行 → 失敗確認**

実行: `go test ./internal/adapter/persistence/ -run TestUpdateSongMetaBMSSearch -v`
期待: FAIL（メソッド未定義）

- [ ] **Step 3: `GetSongMeta` / `UpsertSongMeta` を bms_search_source 対応に修正**

`internal/adapter/persistence/elsa_repository.go:25-53` の `GetSongMeta` と `UpsertSongMeta` を以下に置換:

```go
func (r *ElsaRepository) GetSongMeta(ctx context.Context, folderHash string) (*model.SongMeta, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT folder_hash, release_year, event_id, bms_search_id, bms_search_source FROM song_meta WHERE folder_hash = ?`,
		folderHash,
	)

	var m model.SongMeta
	if err := row.Scan(&m.FolderHash, &m.ReleaseYear, &m.EventID, &m.BMSSearchID, &m.BMSSearchSource); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func (r *ElsaRepository) UpsertSongMeta(ctx context.Context, meta model.SongMeta) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO song_meta (folder_hash, release_year, event_id, bms_search_id, bms_search_source)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(folder_hash) DO UPDATE SET
		   release_year      = excluded.release_year,
		   event_id          = excluded.event_id,
		   bms_search_id     = excluded.bms_search_id,
		   bms_search_source = excluded.bms_search_source,
		   updated_at        = datetime('now')`,
		meta.FolderHash, meta.ReleaseYear, meta.EventID, meta.BMSSearchID, meta.BMSSearchSource,
	)
	return err
}
```

- [ ] **Step 4: `UpdateSongMetaBMSSearch` を追加**

`elsa_repository.go` の `UpdateSongMetaEvent` の直後に追加:

```go
// UpdateSongMetaBMSSearch は song_meta.bms_search_id と bms_search_source のみを更新する。
// 空文字列が渡されたフィールドは NULL になる（解除）。
func (r *ElsaRepository) UpdateSongMetaBMSSearch(ctx context.Context, folderHash, bmsID, source string) error {
	var bmsIDParam, sourceParam interface{}
	if bmsID != "" {
		bmsIDParam = bmsID
	}
	if source != "" {
		sourceParam = source
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO song_meta (folder_hash, bms_search_id, bms_search_source)
		 VALUES (?, ?, ?)
		 ON CONFLICT(folder_hash) DO UPDATE SET
		   bms_search_id     = excluded.bms_search_id,
		   bms_search_source = excluded.bms_search_source,
		   updated_at        = datetime('now')`,
		folderHash, bmsIDParam, sourceParam,
	)
	return err
}
```

- [ ] **Step 5: 既存の `UpdateSongMetaEvent` も `bms_search_source = 'official'` を書き込むよう修正**

`UpdateSongMetaEvent` 全体を以下に置換（既存呼び出し箇所は公式ヒット起因のみのため `'official'` 固定で問題なし）:

```go
func (r *ElsaRepository) UpdateSongMetaEvent(ctx context.Context, folderHash string, eventID string, bmsSearchID string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO song_meta (folder_hash, event_id, bms_search_id, bms_search_source)
		 VALUES (?, ?, ?, 'official')
		 ON CONFLICT(folder_hash) DO UPDATE SET
		   event_id          = excluded.event_id,
		   bms_search_id     = excluded.bms_search_id,
		   bms_search_source = 'official',
		   updated_at        = datetime('now')`,
		folderHash, eventID, bmsSearchID,
	)
	return err
}
```

- [ ] **Step 6: モック更新（usecase_test.go）**

`internal/usecase/usecase_test.go` の `mockMetaRepo` の `UpdateSongMetaEvent` の直後に追加:

```go
func (m *mockMetaRepo) UpdateSongMetaBMSSearch(_ context.Context, _, _, _ string) error {
	return nil
}
```

- [ ] **Step 7: ビルドとテスト確認**

実行: `go build ./...` → 期待: 成功
実行: `go test ./internal/adapter/persistence/ ./internal/usecase/ -v` → 期待: 全 PASS

- [ ] **Step 8: コミット**

```bash
git add internal/domain/model/ internal/adapter/persistence/elsa_repository.go internal/adapter/persistence/elsa_repository_test.go internal/usecase/usecase_test.go
git commit -m "feat: BMSSearch ドメインエンティティ追加と SongMeta.BMSSearchSource 対応"
```

---

### Task 5: BMSSearchRepository 実装

**Files:**
- Create: `internal/adapter/persistence/bmssearch_repository.go`
- Create: `internal/adapter/persistence/bmssearch_repository_test.go`

- [ ] **Step 1: 失敗するテストを書く**

`internal/adapter/persistence/bmssearch_repository_test.go` を新規作成:

```go
package persistence_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

func newTestDBWithMigration(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := persistence.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestBMSSearchRepository_UpsertAndGetLink(t *testing.T) {
	db := newTestDBWithMigration(t)
	defer db.Close()
	repo := persistence.NewBMSSearchRepository(db)
	ctx := context.Background()

	now := time.Unix(1700000000, 0)
	links := []model.BMSSearchLink{
		{MD5: "md5a", BMSID: "bms-1", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
		{MD5: "md5b", BMSID: "bms-1", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
	}
	if err := repo.UpsertLinks(ctx, links); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetLinkByMD5(ctx, "md5a")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.BMSID != "bms-1" || got.Source != model.BMSSearchSourceOfficial {
		t.Errorf("got = %+v", got)
	}

	// UPSERT: source 上書き
	links2 := []model.BMSSearchLink{
		{MD5: "md5a", BMSID: "bms-2", Source: model.BMSSearchSourceUnofficial, ResolvedAt: now},
	}
	if err := repo.UpsertLinks(ctx, links2); err != nil {
		t.Fatal(err)
	}
	got2, _ := repo.GetLinkByMD5(ctx, "md5a")
	if got2.BMSID != "bms-2" || got2.Source != model.BMSSearchSourceUnofficial {
		t.Errorf("got2 = %+v, expected upsert", got2)
	}
}

func TestBMSSearchRepository_DeleteLinks(t *testing.T) {
	db := newTestDBWithMigration(t)
	defer db.Close()
	repo := persistence.NewBMSSearchRepository(db)
	ctx := context.Background()

	now := time.Unix(1700000000, 0)
	_ = repo.UpsertLinks(ctx, []model.BMSSearchLink{
		{MD5: "x", BMSID: "b", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
		{MD5: "y", BMSID: "b", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
		{MD5: "z", BMSID: "b", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
	})

	if err := repo.DeleteLinkByMD5(ctx, "x"); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetLinkByMD5(ctx, "x")
	if got != nil {
		t.Errorf("got = %+v, want nil after delete", got)
	}

	if err := repo.DeleteLinksByMD5s(ctx, []string{"y", "z"}); err != nil {
		t.Fatal(err)
	}
	g, _ := repo.GetLinkByMD5(ctx, "y")
	if g != nil {
		t.Errorf("y should be deleted")
	}
}

func TestBMSSearchRepository_UpsertAndGetBMS(t *testing.T) {
	db := newTestDBWithMigration(t)
	defer db.Close()
	repo := persistence.NewBMSSearchRepository(db)
	ctx := context.Background()

	exID := "ex-1"
	bms := model.BMSSearchBMS{
		BMSID:          "bms-1",
		Title:          "Test Song",
		Artist:         "Artist",
		SubArtist:      "feat. X",
		Genre:          "TECHNO",
		ExhibitionID:   &exID,
		ExhibitionName: "BOFXX",
		PublishedAt:    "2024-08-01T00:00:00Z",
		Downloads: []model.BMSSearchURLEntry{
			{URL: "https://dl.example.com/x.zip", Description: "本体"},
		},
		Previews: []model.BMSSearchPreview{
			{Service: "YOUTUBE", Parameter: "abc123"},
		},
		RelatedLinks: []model.BMSSearchURLEntry{
			{URL: "https://twitter.com/a", Description: "作者"},
		},
		FetchedAt: time.Unix(1700000000, 0),
	}
	if err := repo.UpsertBMS(ctx, bms); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetBMSByID(ctx, "bms-1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Title != "Test Song" || got.ExhibitionID == nil || *got.ExhibitionID != "ex-1" {
		t.Errorf("got = %+v", got)
	}
	if len(got.Downloads) != 1 || got.Downloads[0].URL != "https://dl.example.com/x.zip" {
		t.Errorf("downloads = %+v", got.Downloads)
	}
	if len(got.Previews) != 1 || got.Previews[0].Service != "YOUTUBE" {
		t.Errorf("previews = %+v", got.Previews)
	}

	// 独立曲（exhibition_id=NULL）
	bms2 := model.BMSSearchBMS{BMSID: "bms-2", Title: "Solo", FetchedAt: time.Unix(1700000001, 0)}
	if err := repo.UpsertBMS(ctx, bms2); err != nil {
		t.Fatal(err)
	}
	got2, _ := repo.GetBMSByID(ctx, "bms-2")
	if got2 == nil || got2.ExhibitionID != nil {
		t.Errorf("got2.ExhibitionID = %v, want nil", got2)
	}
}
```

- [ ] **Step 2: テスト実行 → 失敗確認**

実行: `go test ./internal/adapter/persistence/ -run TestBMSSearchRepository -v`
期待: FAIL（パッケージにシンボル未定義）

- [ ] **Step 3: 実装**

`internal/adapter/persistence/bmssearch_repository.go` を新規作成:

```go
package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

var _ model.BMSSearchRepository = (*BMSSearchRepository)(nil)

type BMSSearchRepository struct {
	db *sql.DB
}

func NewBMSSearchRepository(db *sql.DB) *BMSSearchRepository {
	return &BMSSearchRepository{db: db}
}

func (r *BMSSearchRepository) GetLinkByMD5(ctx context.Context, md5 string) (*model.BMSSearchLink, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT md5, bms_id, source, resolved_at FROM bmssearch_bms_id_md5 WHERE md5 = ?`,
		md5,
	)
	var l model.BMSSearchLink
	var src string
	var resolvedUnix int64
	if err := row.Scan(&l.MD5, &l.BMSID, &src, &resolvedUnix); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	l.Source = model.BMSSearchSource(src)
	l.ResolvedAt = time.Unix(resolvedUnix, 0)
	return &l, nil
}

func (r *BMSSearchRepository) UpsertLinks(ctx context.Context, links []model.BMSSearchLink) error {
	if len(links) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO bmssearch_bms_id_md5 (md5, bms_id, source, resolved_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(md5) DO UPDATE SET
		   bms_id      = excluded.bms_id,
		   source      = excluded.source,
		   resolved_at = excluded.resolved_at`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, l := range links {
		if _, err := stmt.ExecContext(ctx, l.MD5, l.BMSID, string(l.Source), l.ResolvedAt.Unix()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *BMSSearchRepository) DeleteLinkByMD5(ctx context.Context, md5 string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM bmssearch_bms_id_md5 WHERE md5 = ?`, md5)
	return err
}

func (r *BMSSearchRepository) DeleteLinksByMD5s(ctx context.Context, md5s []string) error {
	if len(md5s) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(md5s))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(md5s))
	for i, m := range md5s {
		args[i] = m
	}
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM bmssearch_bms_id_md5 WHERE md5 IN (`+placeholders+`)`,
		args...,
	)
	return err
}

func (r *BMSSearchRepository) GetBMSByID(ctx context.Context, bmsID string) (*model.BMSSearchBMS, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT bms_id, title, artist, subartist, genre,
		       exhibition_id, exhibition_name, published_at,
		       downloads_json, previews_json, related_links_json, fetched_at
		FROM bmssearch_bms WHERE bms_id = ?`, bmsID)
	var b model.BMSSearchBMS
	var exID sql.NullString
	var dlsJSON, prevsJSON, relJSON string
	var fetchedUnix int64
	if err := row.Scan(
		&b.BMSID, &b.Title, &b.Artist, &b.SubArtist, &b.Genre,
		&exID, &b.ExhibitionName, &b.PublishedAt,
		&dlsJSON, &prevsJSON, &relJSON, &fetchedUnix,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if exID.Valid {
		s := exID.String
		b.ExhibitionID = &s
	}
	b.FetchedAt = time.Unix(fetchedUnix, 0)
	if err := json.Unmarshal([]byte(dlsJSON), &b.Downloads); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(prevsJSON), &b.Previews); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(relJSON), &b.RelatedLinks); err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *BMSSearchRepository) UpsertBMS(ctx context.Context, bms model.BMSSearchBMS) error {
	dlsJSON, err := json.Marshal(emptyIfNilURLEntries(bms.Downloads))
	if err != nil {
		return err
	}
	prevsJSON, err := json.Marshal(emptyIfNilPreviews(bms.Previews))
	if err != nil {
		return err
	}
	relJSON, err := json.Marshal(emptyIfNilURLEntries(bms.RelatedLinks))
	if err != nil {
		return err
	}
	var exIDParam interface{}
	if bms.ExhibitionID != nil {
		exIDParam = *bms.ExhibitionID
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO bmssearch_bms
		  (bms_id, title, artist, subartist, genre, exhibition_id, exhibition_name,
		   published_at, downloads_json, previews_json, related_links_json, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(bms_id) DO UPDATE SET
		  title              = excluded.title,
		  artist             = excluded.artist,
		  subartist          = excluded.subartist,
		  genre              = excluded.genre,
		  exhibition_id      = excluded.exhibition_id,
		  exhibition_name    = excluded.exhibition_name,
		  published_at       = excluded.published_at,
		  downloads_json     = excluded.downloads_json,
		  previews_json      = excluded.previews_json,
		  related_links_json = excluded.related_links_json,
		  fetched_at         = excluded.fetched_at`,
		bms.BMSID, bms.Title, bms.Artist, bms.SubArtist, bms.Genre,
		exIDParam, bms.ExhibitionName, bms.PublishedAt,
		string(dlsJSON), string(prevsJSON), string(relJSON), bms.FetchedAt.Unix(),
	)
	return err
}

func emptyIfNilURLEntries(v []model.BMSSearchURLEntry) []model.BMSSearchURLEntry {
	if v == nil {
		return []model.BMSSearchURLEntry{}
	}
	return v
}

func emptyIfNilPreviews(v []model.BMSSearchPreview) []model.BMSSearchPreview {
	if v == nil {
		return []model.BMSSearchPreview{}
	}
	return v
}
```

- [ ] **Step 4: テスト実行 → 成功確認**

実行: `go test ./internal/adapter/persistence/ -run TestBMSSearchRepository -v`
期待: 全 PASS

- [ ] **Step 5: コミット**

```bash
git add internal/adapter/persistence/bmssearch_repository.go internal/adapter/persistence/bmssearch_repository_test.go
git commit -m "feat: BMSSearchRepository 実装（bmssearch_bms_id_md5 / bmssearch_bms）"
```

---

## Phase 3: ゲートウェイ層拡張

### Task 6: BMSSearchClient 拡張

**Files:**
- Modify: `internal/adapter/gateway/bmssearch_client.go`
- Modify: `internal/adapter/gateway/bmssearch_client_test.go`

- [ ] **Step 1: 失敗するテストを追加**

`internal/adapter/gateway/bmssearch_client_test.go` 末尾に追加:

```go
func TestSearchBMSesByTitle_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bmses/search" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		if q.Get("title") != "Test Song" || q.Get("limit") != "20" ||
			q.Get("orderBy") != "PUBLISHED" || q.Get("orderDirection") != "DESC" {
			t.Errorf("unexpected query: %v", q)
		}
		w.Write([]byte(`[
			{"id":"bms-1","title":"Test Song","artist":"A","subartist":"","genre":"G",
			 "exhibition":{"id":"ex","name":"BOFXX"},"publishedAt":"2024-01-01",
			 "downloads":[{"url":"https://x","description":"本体"}],
			 "previews":[{"service":"YOUTUBE","parameter":"abc"}],
			 "relatedLinks":[]},
			{"id":"bms-2","title":"Other","artist":"B"}
		]`))
	}))
	defer srv.Close()
	c := NewBMSSearchClientWithBaseURL(srv.URL)
	got, err := c.SearchBMSesByTitle(context.Background(), "Test Song", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "bms-1" {
		t.Errorf("got = %+v", got)
	}
	if len(got[0].Downloads) != 1 || got[0].Downloads[0].URL != "https://x" {
		t.Errorf("downloads = %+v", got[0].Downloads)
	}
}

func TestSearchBMSesByTitle_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	c := NewBMSSearchClientWithBaseURL(srv.URL)
	got, err := c.SearchBMSesByTitle(context.Background(), "nope", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got = %+v, want empty", got)
	}
}

func TestLookupBMS_FullFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"BMS-1","title":"T","artist":"A","subartist":"S","genre":"G",
			"exhibition":{"id":"EX","name":"E"},"publishedAt":"2024",
			"downloads":[{"url":"u","description":"d"}],
			"previews":[{"service":"YOUTUBE","parameter":"p"}],
			"relatedLinks":[{"url":"r","description":"rd"}]}`))
	}))
	defer srv.Close()
	c := NewBMSSearchClientWithBaseURL(srv.URL)
	b, err := c.LookupBMS(context.Background(), "BMS-1")
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "T" || b.SubArtist != "S" || b.Genre != "G" {
		t.Errorf("metadata mismatch: %+v", b)
	}
	if len(b.Downloads) != 1 || len(b.Previews) != 1 || len(b.RelatedLinks) != 1 {
		t.Errorf("array fields mismatch: %+v", b)
	}
}
```

- [ ] **Step 2: テスト実行 → 失敗確認**

実行: `go test ./internal/adapter/gateway/ -run "TestSearchBMSesByTitle|TestLookupBMS_FullFields" -v`
期待: FAIL（メソッド未定義 / フィールド不足）

- [ ] **Step 3: `BMSSearchBMS` 構造体を拡張**

`internal/adapter/gateway/bmssearch_client.go:27-31` の `BMSSearchBMS` を以下に置換:

```go
type BMSSearchBMS struct {
	ID           string               `json:"id"`
	Title        string               `json:"title"`
	Artist       string               `json:"artist"`
	SubArtist    string               `json:"subartist"`
	Genre        string               `json:"genre"`
	Exhibition   *BMSSearchExhibition `json:"exhibition"`
	PublishedAt  string               `json:"publishedAt"`
	Downloads    []BMSSearchURLEntry  `json:"downloads"`
	Previews     []BMSSearchPreview   `json:"previews"`
	RelatedLinks []BMSSearchURLEntry  `json:"relatedLinks"`
}

type BMSSearchURLEntry struct {
	URL         string `json:"url"`
	Description string `json:"description"`
}

type BMSSearchPreview struct {
	Service   string `json:"service"`
	Parameter string `json:"parameter"`
}
```

- [ ] **Step 4: `SearchBMSesByTitle` メソッドを追加**

`bmssearch_client.go` の `FetchAllExhibitions` の直前に追加:

```go
// SearchBMSesByTitle はテキストでフォールバック検索を行う。
// 公式 md5 ヒットしなかったときに使用する。limit は通常 20。
func (c *BMSSearchClient) SearchBMSesByTitle(ctx context.Context, title string, limit int) ([]BMSSearchBMS, error) {
	c.rateLimit()
	q := url.Values{}
	q.Set("title", title)
	q.Set("limit", fmt.Sprintf("%d", limit))
	q.Set("orderBy", "PUBLISHED")
	q.Set("orderDirection", "DESC")
	u := fmt.Sprintf("%s/bmses/search?%s", c.baseURL, q.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("BMS Search search: HTTP %d for %s", resp.StatusCode, u)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var arr []BMSSearchBMS
	if err := json.Unmarshal(body, &arr); err != nil {
		return nil, fmt.Errorf("BMS Search search parse: %w", err)
	}
	return arr, nil
}
```

ファイル先頭の import に `"net/url"` を追加（未追加なら）。

- [ ] **Step 5: テスト実行 → 成功確認**

実行: `go test ./internal/adapter/gateway/ -v`
期待: 全 PASS（既存3件 + 新規3件）

- [ ] **Step 6: コミット**

```bash
git add internal/adapter/gateway/bmssearch_client.go internal/adapter/gateway/bmssearch_client_test.go
git commit -m "feat: BMSSearchClient に SearchBMSesByTitle 追加と BMSSearchBMS 構造体拡張"
```

---

## Phase 4: ユースケース層

### Task 7: 正規化・スコアリング pure functions

**Files:**
- Create: `internal/usecase/bmssearch_scoring.go`
- Create: `internal/usecase/bmssearch_scoring_test.go`

**注意:** スコア配点・閾値・正規化ルールは Task 1 の調査結果（`docs/superpowers/specs/2026-04-27-bmssearch-info-fallback-probe.md`）で確定した値を使う。下記の数値は設計ドキュメント初期案。調査結果と異なる場合は調査結果を優先すること。

- [ ] **Step 1: 失敗するテストを書く**

`internal/usecase/bmssearch_scoring_test.go` を新規作成:

```go
package usecase_test

import (
	"testing"

	"github.com/meta-BE/bms-elsa/internal/usecase"
)

func TestNormalizeTitle(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Test Song", "test song"},
		{"  Spaces  ", "spaces"},
		{"FULLWIDTH", "fullwidth"},
		{"全角カタカナ", "全角カタカナ"},
	}
	for _, c := range cases {
		got := usecase.NormalizeTitle(c.in)
		if got != c.want {
			t.Errorf("NormalizeTitle(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStripTrailingDecorations(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Title [ANBMS]", "Title"},
		{"Title (BMS Edition)", "Title"},
		{"Title -Remix-", "Title"},
		{"Title [A] (B)", "Title"},
		{"Plain", "Plain"},
	}
	for _, c := range cases {
		got := usecase.StripTrailingDecorations(c.in)
		if got != c.want {
			t.Errorf("StripTrailingDecorations(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestScoreCandidate_TitleExact(t *testing.T) {
	score := usecase.ScoreCandidate(usecase.ScoreInput{
		QueryTitle:     "Test Song",
		QueryArtist:    "Artist",
		CandidateTitle: "Test Song",
		CandidateArtist: "Artist",
	})
	// title 完全一致(60) + artist 完全一致(20) + token率(10) = 90
	if score < 80 {
		t.Errorf("score = %d, want >=80", score)
	}
}

func TestScoreCandidate_TitleNormalized(t *testing.T) {
	score := usecase.ScoreCandidate(usecase.ScoreInput{
		QueryTitle:      "Test Song",
		QueryArtist:     "Artist",
		CandidateTitle:  "test  song",
		CandidateArtist: "ARTIST",
	})
	// title 正規化後一致(50) + artist 正規化後一致(15) + α
	if score < 60 {
		t.Errorf("score = %d, want >=60", score)
	}
}

func TestScoreCandidate_BelowThreshold(t *testing.T) {
	score := usecase.ScoreCandidate(usecase.ScoreInput{
		QueryTitle:      "Test Song",
		QueryArtist:     "Artist",
		CandidateTitle:  "Completely Different",
		CandidateArtist: "Other",
	})
	if score >= 50 {
		t.Errorf("score = %d, want <50", score)
	}
}

func TestPickBestCandidate_Empty(t *testing.T) {
	got, ok := usecase.PickBestCandidate(nil, "T", "A", 50)
	if ok || got != -1 {
		t.Errorf("got idx=%d ok=%v, want -1, false", got, ok)
	}
}

func TestPickBestCandidate_BelowThreshold(t *testing.T) {
	cands := []usecase.ScoreCandidateRef{
		{Title: "Foo", Artist: "X"},
		{Title: "Bar", Artist: "Y"},
	}
	got, ok := usecase.PickBestCandidate(cands, "Test", "Artist", 50)
	if ok {
		t.Errorf("got idx=%d, want not ok (below threshold)", got)
	}
}

func TestPickBestCandidate_Tied(t *testing.T) {
	cands := []usecase.ScoreCandidateRef{
		{Title: "Test Song", Artist: "Artist"},
		{Title: "Test Song", Artist: "Artist"},
	}
	_, ok := usecase.PickBestCandidate(cands, "Test Song", "Artist", 50)
	if ok {
		t.Errorf("tied top should not be picked")
	}
}

func TestPickBestCandidate_Picked(t *testing.T) {
	cands := []usecase.ScoreCandidateRef{
		{Title: "Other", Artist: "Z"},
		{Title: "Test Song", Artist: "Artist"},
		{Title: "Different", Artist: "W"},
	}
	idx, ok := usecase.PickBestCandidate(cands, "Test Song", "Artist", 50)
	if !ok || idx != 1 {
		t.Errorf("got idx=%d ok=%v, want 1, true", idx, ok)
	}
}
```

- [ ] **Step 2: テスト実行 → 失敗確認**

実行: `go test ./internal/usecase/ -run "TestNormalizeTitle|TestStripTrailingDecorations|TestScoreCandidate|TestPickBestCandidate" -v`
期待: FAIL（シンボル未定義）

- [ ] **Step 3: 実装**

`internal/usecase/bmssearch_scoring.go` を新規作成:

```go
package usecase

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// NormalizeTitle はタイトルを比較用に正規化する。
// 大小無視・前後空白除去・連続空白の単一化・NFKC 正規化（全半角統一）。
func NormalizeTitle(s string) string {
	s = norm.NFKC.String(s)
	s = strings.ToLower(strings.TrimSpace(s))
	// 連続空白を1つに圧縮
	out := strings.Builder{}
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				out.WriteRune(' ')
			}
			prevSpace = true
		} else {
			out.WriteRune(r)
			prevSpace = false
		}
	}
	return strings.TrimSpace(out.String())
}

// StripTrailingDecorations は末尾の [...] / (...) / -...- を再帰的に剥離する。
// "Title [A] (B)" → "Title"
func StripTrailingDecorations(s string) string {
	s = strings.TrimSpace(s)
	for {
		trimmed := s
		for _, pair := range [][2]rune{{'[', ']'}, {'(', ')'}, {'-', '-'}} {
			if len(trimmed) > 0 && rune(trimmed[len(trimmed)-1]) == pair[1] {
				idx := strings.LastIndexByte(trimmed[:len(trimmed)-1], byte(pair[0]))
				if idx > 0 {
					trimmed = strings.TrimSpace(trimmed[:idx])
				}
			}
		}
		if trimmed == s {
			break
		}
		s = trimmed
	}
	return s
}

type ScoreInput struct {
	QueryTitle      string
	QueryArtist     string
	CandidateTitle  string
	CandidateArtist string
}

// ScoreCandidate は候補1件のスコアを計算する（最大90点）。
// title 系3項目は最高1項目のみ採用、artist 系2項目も同様。token 率は独立加算。
func ScoreCandidate(in ScoreInput) int {
	score := 0

	// title 系（排他）
	titleScore := 0
	if in.QueryTitle == in.CandidateTitle {
		titleScore = 60
	} else if NormalizeTitle(in.QueryTitle) == NormalizeTitle(in.CandidateTitle) {
		titleScore = 50
	} else {
		nq := NormalizeTitle(in.QueryTitle)
		nc := NormalizeTitle(in.CandidateTitle)
		if nq != "" && nc != "" && (strings.Contains(nc, nq) || strings.Contains(nq, nc)) {
			titleScore = 25
		}
	}
	score += titleScore

	// artist 系（排他）
	artistScore := 0
	if in.QueryArtist != "" {
		if in.QueryArtist == in.CandidateArtist {
			artistScore = 20
		} else if NormalizeTitle(in.QueryArtist) == NormalizeTitle(in.CandidateArtist) {
			artistScore = 15
		}
	}
	score += artistScore

	// artist トークン共通率（独立加算、最大10点）
	score += artistTokenScore(in.QueryArtist, in.CandidateArtist)

	return score
}

func artistTokenScore(a, b string) int {
	at := tokenize(NormalizeTitle(a))
	bt := tokenize(NormalizeTitle(b))
	if len(at) == 0 || len(bt) == 0 {
		return 0
	}
	common := 0
	bset := make(map[string]struct{}, len(bt))
	for _, t := range bt {
		bset[t] = struct{}{}
	}
	for _, t := range at {
		if _, ok := bset[t]; ok {
			common++
		}
	}
	denom := len(at)
	if len(bt) > denom {
		denom = len(bt)
	}
	return common * 10 / denom
}

func tokenize(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || r == '/' || r == ',' || r == '&'
	})
	return fields
}

// ScoreCandidateRef はスコアリング対象の最小情報
type ScoreCandidateRef struct {
	Title  string
	Artist string
}

// PickBestCandidate は候補群から最高スコアのインデックスを返す。
// 閾値未満 / 同点首位の場合は ok=false を返す。
func PickBestCandidate(cands []ScoreCandidateRef, queryTitle, queryArtist string, threshold int) (int, bool) {
	if len(cands) == 0 {
		return -1, false
	}
	bestIdx := -1
	bestScore := -1
	tie := false
	for i, c := range cands {
		s := ScoreCandidate(ScoreInput{
			QueryTitle:      queryTitle,
			QueryArtist:     queryArtist,
			CandidateTitle:  c.Title,
			CandidateArtist: c.Artist,
		})
		if s > bestScore {
			bestScore = s
			bestIdx = i
			tie = false
		} else if s == bestScore {
			tie = true
		}
	}
	if bestScore < threshold || tie {
		return -1, false
	}
	return bestIdx, true
}
```

- [ ] **Step 4: 依存追加**

`golang.org/x/text` が `go.mod` にない場合: `go get golang.org/x/text/unicode/norm`
既にあれば不要。

実行: `go mod tidy` で確認。

- [ ] **Step 5: テスト実行 → 成功確認**

実行: `go test ./internal/usecase/ -run "TestNormalizeTitle|TestStripTrailingDecorations|TestScoreCandidate|TestPickBestCandidate" -v`
期待: 全 PASS

- [ ] **Step 6: コミット**

```bash
git add internal/usecase/bmssearch_scoring.go internal/usecase/bmssearch_scoring_test.go go.mod go.sum
git commit -m "feat: BMSSearch フォールバック検索の正規化・スコアリング追加"
```

---

### Task 8: BMSSearchResolver 実装

**Files:**
- Create: `internal/usecase/bmssearch_resolver.go`
- Create: `internal/usecase/bmssearch_resolver_test.go`

- [ ] **Step 1: 失敗するテストを書く**

`internal/usecase/bmssearch_resolver_test.go` を新規作成:

```go
package usecase_test

import (
	"context"
	"testing"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

// 共有モック（必要な機能だけ実装）
type fakeBMSClient struct {
	patternFn func(ctx context.Context, md5 string) (*gateway.BMSSearchPattern, error)
	bmsFn     func(ctx context.Context, id string) (*gateway.BMSSearchBMS, error)
	searchFn  func(ctx context.Context, title string, limit int) ([]gateway.BMSSearchBMS, error)
}

func (f *fakeBMSClient) LookupPatternByMD5(ctx context.Context, md5 string) (*gateway.BMSSearchPattern, error) {
	return f.patternFn(ctx, md5)
}
func (f *fakeBMSClient) LookupBMS(ctx context.Context, id string) (*gateway.BMSSearchBMS, error) {
	return f.bmsFn(ctx, id)
}
func (f *fakeBMSClient) SearchBMSesByTitle(ctx context.Context, title string, limit int) ([]gateway.BMSSearchBMS, error) {
	return f.searchFn(ctx, title, limit)
}

type fakeBMSSearchRepo struct {
	links     map[string]*model.BMSSearchLink
	bmsCache  map[string]*model.BMSSearchBMS
	upsertFn  func([]model.BMSSearchLink)
	upsertBMS func(model.BMSSearchBMS)
}

func newFakeBMSSearchRepo() *fakeBMSSearchRepo {
	return &fakeBMSSearchRepo{
		links:    map[string]*model.BMSSearchLink{},
		bmsCache: map[string]*model.BMSSearchBMS{},
	}
}

func (f *fakeBMSSearchRepo) GetLinkByMD5(_ context.Context, md5 string) (*model.BMSSearchLink, error) {
	return f.links[md5], nil
}
func (f *fakeBMSSearchRepo) UpsertLinks(_ context.Context, links []model.BMSSearchLink) error {
	for i := range links {
		l := links[i]
		f.links[l.MD5] = &l
	}
	if f.upsertFn != nil {
		f.upsertFn(links)
	}
	return nil
}
func (f *fakeBMSSearchRepo) DeleteLinkByMD5(_ context.Context, md5 string) error {
	delete(f.links, md5)
	return nil
}
func (f *fakeBMSSearchRepo) DeleteLinksByMD5s(_ context.Context, md5s []string) error {
	for _, m := range md5s {
		delete(f.links, m)
	}
	return nil
}
func (f *fakeBMSSearchRepo) GetBMSByID(_ context.Context, id string) (*model.BMSSearchBMS, error) {
	return f.bmsCache[id], nil
}
func (f *fakeBMSSearchRepo) UpsertBMS(_ context.Context, b model.BMSSearchBMS) error {
	f.bmsCache[b.BMSID] = &b
	if f.upsertBMS != nil {
		f.upsertBMS(b)
	}
	return nil
}

func TestResolveForFolder_OfficialHit(t *testing.T) {
	bmsClient := &fakeBMSClient{
		patternFn: func(_ context.Context, md5 string) (*gateway.BMSSearchPattern, error) {
			if md5 == "md5b" {
				p := &gateway.BMSSearchPattern{}
				p.BMS.ID = "bms-1"
				return p, nil
			}
			return nil, nil
		},
		bmsFn: func(_ context.Context, id string) (*gateway.BMSSearchBMS, error) {
			return &gateway.BMSSearchBMS{ID: id, Title: "T", Artist: "A"}, nil
		},
		searchFn: func(_ context.Context, _ string, _ int) ([]gateway.BMSSearchBMS, error) {
			t.Errorf("search should not be called when official hit succeeds")
			return nil, nil
		},
	}
	bmssearchRepo := newFakeBMSSearchRepo()
	metaRepo := &mockMetaRepo{
		updateSongMetaBMSSearchFn: func(_ context.Context, _, _, _ string) error { return nil },
	}

	resolver := usecase.NewBMSSearchResolver(bmsClient, bmssearchRepo, metaRepo)
	bmsID, source, err := resolver.ResolveForFolder(context.Background(), "folder1", []string{"md5a", "md5b", "md5c"}, "T", "A")
	if err != nil {
		t.Fatal(err)
	}
	if bmsID != "bms-1" || source != model.BMSSearchSourceOfficial {
		t.Errorf("got bmsID=%q source=%q", bmsID, source)
	}
	// 全 md5 がリンクされていること
	for _, m := range []string{"md5a", "md5b", "md5c"} {
		if l := bmssearchRepo.links[m]; l == nil || l.BMSID != "bms-1" || l.Source != model.BMSSearchSourceOfficial {
			t.Errorf("link[%s] = %+v", m, l)
		}
	}
	// bmssearch_bms に保存されていること
	if bmssearchRepo.bmsCache["bms-1"] == nil {
		t.Errorf("bms not cached")
	}
}

func TestResolveForFolder_FallbackHit(t *testing.T) {
	bmsClient := &fakeBMSClient{
		patternFn: func(_ context.Context, _ string) (*gateway.BMSSearchPattern, error) { return nil, nil },
		bmsFn: func(_ context.Context, id string) (*gateway.BMSSearchBMS, error) {
			return &gateway.BMSSearchBMS{ID: id, Title: "Test Song", Artist: "Artist"}, nil
		},
		searchFn: func(_ context.Context, title string, _ int) ([]gateway.BMSSearchBMS, error) {
			return []gateway.BMSSearchBMS{
				{ID: "bms-x", Title: "Test Song", Artist: "Artist"},
				{ID: "bms-y", Title: "Different", Artist: "Other"},
			}, nil
		},
	}
	bmssearchRepo := newFakeBMSSearchRepo()
	metaRepo := &mockMetaRepo{
		updateSongMetaBMSSearchFn: func(_ context.Context, _, _, _ string) error { return nil },
	}

	resolver := usecase.NewBMSSearchResolver(bmsClient, bmssearchRepo, metaRepo)
	bmsID, source, err := resolver.ResolveForFolder(context.Background(), "f1", []string{"md5a"}, "Test Song", "Artist")
	if err != nil {
		t.Fatal(err)
	}
	if bmsID != "bms-x" || source != model.BMSSearchSourceUnofficial {
		t.Errorf("got bmsID=%q source=%q", bmsID, source)
	}
}

func TestResolveForFolder_BothFail(t *testing.T) {
	bmsClient := &fakeBMSClient{
		patternFn: func(_ context.Context, _ string) (*gateway.BMSSearchPattern, error) { return nil, nil },
		bmsFn:     func(_ context.Context, _ string) (*gateway.BMSSearchBMS, error) { return nil, nil },
		searchFn: func(_ context.Context, _ string, _ int) ([]gateway.BMSSearchBMS, error) {
			return []gateway.BMSSearchBMS{
				{ID: "bms-z", Title: "Foo", Artist: "Bar"},
			}, nil
		},
	}
	bmssearchRepo := newFakeBMSSearchRepo()
	metaRepo := &mockMetaRepo{}

	resolver := usecase.NewBMSSearchResolver(bmsClient, bmssearchRepo, metaRepo)
	bmsID, source, err := resolver.ResolveForFolder(context.Background(), "f1", []string{"md5a"}, "Test Song", "Artist")
	if err != nil {
		t.Fatal(err)
	}
	if bmsID != "" || source != "" {
		t.Errorf("got bmsID=%q source=%q, want empty", bmsID, source)
	}
	if len(bmssearchRepo.links) != 0 {
		t.Errorf("no link should be written, got %+v", bmssearchRepo.links)
	}
}

func TestResolveForOrphanMD5_OfficialHit(t *testing.T) {
	bmsClient := &fakeBMSClient{
		patternFn: func(_ context.Context, _ string) (*gateway.BMSSearchPattern, error) {
			p := &gateway.BMSSearchPattern{}
			p.BMS.ID = "bms-1"
			return p, nil
		},
		bmsFn: func(_ context.Context, id string) (*gateway.BMSSearchBMS, error) {
			return &gateway.BMSSearchBMS{ID: id}, nil
		},
		searchFn: func(_ context.Context, _ string, _ int) ([]gateway.BMSSearchBMS, error) { return nil, nil },
	}
	bmssearchRepo := newFakeBMSSearchRepo()
	metaRepo := &mockMetaRepo{}

	called := false
	metaRepo.updateSongMetaBMSSearchFn = func(_ context.Context, _, _, _ string) error {
		called = true
		return nil
	}

	resolver := usecase.NewBMSSearchResolver(bmsClient, bmssearchRepo, metaRepo)
	bmsID, source, err := resolver.ResolveForOrphanMD5(context.Background(), "md5x", "T", "A")
	if err != nil {
		t.Fatal(err)
	}
	if bmsID != "bms-1" || source != model.BMSSearchSourceOfficial {
		t.Errorf("got bmsID=%q source=%q", bmsID, source)
	}
	if called {
		t.Error("ResolveForOrphanMD5 should NOT touch song_meta")
	}
	if l := bmssearchRepo.links["md5x"]; l == nil {
		t.Error("link should be saved")
	}
}
```

`mockMetaRepo` に `updateSongMetaBMSSearchFn` フィールドを追加する必要があるが、`internal/usecase/usecase_test.go` の `UpdateSongMetaBMSSearch` メソッドを以下のように差し替えること:

```go
func (m *mockMetaRepo) UpdateSongMetaBMSSearch(ctx context.Context, folderHash, bmsID, source string) error {
	if m.updateSongMetaBMSSearchFn != nil {
		return m.updateSongMetaBMSSearchFn(ctx, folderHash, bmsID, source)
	}
	return nil
}
```

そして `mockMetaRepo` 構造体定義に `updateSongMetaBMSSearchFn func(ctx context.Context, folderHash, bmsID, source string) error` フィールドを追加。

- [ ] **Step 2: テスト実行 → 失敗確認**

実行: `go test ./internal/usecase/ -run TestResolveFor -v`
期待: FAIL（Resolver 未定義）

- [ ] **Step 3: Resolver の interface 抽出 + 実装**

`internal/usecase/bmssearch_resolver.go` を新規作成:

```go
package usecase

import (
	"context"
	"time"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

// BMSSearchAPI は BMSSearchClient のうち Resolver が使う部分のインターフェース
type BMSSearchAPI interface {
	LookupPatternByMD5(ctx context.Context, md5 string) (*gateway.BMSSearchPattern, error)
	LookupBMS(ctx context.Context, bmsID string) (*gateway.BMSSearchBMS, error)
	SearchBMSesByTitle(ctx context.Context, title string, limit int) ([]gateway.BMSSearchBMS, error)
}

const (
	fallbackSearchLimit     = 20
	fallbackScoreThreshold  = 50
)

type BMSSearchResolver struct {
	bmsClient     BMSSearchAPI
	bmssearchRepo model.BMSSearchRepository
	metaRepo      model.MetaRepository
}

func NewBMSSearchResolver(
	bmsClient BMSSearchAPI,
	bmssearchRepo model.BMSSearchRepository,
	metaRepo model.MetaRepository,
) *BMSSearchResolver {
	return &BMSSearchResolver{
		bmsClient:     bmsClient,
		bmssearchRepo: bmssearchRepo,
		metaRepo:      metaRepo,
	}
}

// ResolveForFolder は楽曲フォルダ単位の解決。
// 公式ヒット時 + フォールバック採用時に bmssearch_bms_id_md5 / bmssearch_bms / song_meta を書き込む。
// 未紐付け時は ("", "", nil)。
func (r *BMSSearchResolver) ResolveForFolder(
	ctx context.Context,
	folderHash string,
	md5s []string,
	title, artist string,
) (string, model.BMSSearchSource, error) {
	bmsID, hit, err := r.tryOfficial(ctx, md5s)
	if err != nil {
		return "", "", err
	}
	if hit {
		if err := r.persist(ctx, md5s, bmsID, model.BMSSearchSourceOfficial); err != nil {
			return "", "", err
		}
		if err := r.metaRepo.UpdateSongMetaBMSSearch(ctx, folderHash, bmsID, string(model.BMSSearchSourceOfficial)); err != nil {
			return "", "", err
		}
		return bmsID, model.BMSSearchSourceOfficial, nil
	}

	bmsID, hit, err = r.tryFallback(ctx, title, artist)
	if err != nil {
		return "", "", err
	}
	if hit {
		if err := r.persist(ctx, md5s, bmsID, model.BMSSearchSourceUnofficial); err != nil {
			return "", "", err
		}
		if err := r.metaRepo.UpdateSongMetaBMSSearch(ctx, folderHash, bmsID, string(model.BMSSearchSourceUnofficial)); err != nil {
			return "", "", err
		}
		return bmsID, model.BMSSearchSourceUnofficial, nil
	}
	return "", "", nil
}

// ResolveForOrphanMD5 は未所持 md5 単位の解決。song_meta は触らない。
func (r *BMSSearchResolver) ResolveForOrphanMD5(
	ctx context.Context,
	md5, title, artist string,
) (string, model.BMSSearchSource, error) {
	bmsID, hit, err := r.tryOfficial(ctx, []string{md5})
	if err != nil {
		return "", "", err
	}
	if hit {
		if err := r.persist(ctx, []string{md5}, bmsID, model.BMSSearchSourceOfficial); err != nil {
			return "", "", err
		}
		return bmsID, model.BMSSearchSourceOfficial, nil
	}
	bmsID, hit, err = r.tryFallback(ctx, title, artist)
	if err != nil {
		return "", "", err
	}
	if hit {
		if err := r.persist(ctx, []string{md5}, bmsID, model.BMSSearchSourceUnofficial); err != nil {
			return "", "", err
		}
		return bmsID, model.BMSSearchSourceUnofficial, nil
	}
	return "", "", nil
}

// tryOfficial は公式 md5 ヒットを試み、最初に見つかった bmsID を返す
func (r *BMSSearchResolver) tryOfficial(ctx context.Context, md5s []string) (string, bool, error) {
	for _, md5 := range md5s {
		p, err := r.bmsClient.LookupPatternByMD5(ctx, md5)
		if err != nil {
			continue
		}
		if p == nil {
			continue
		}
		return p.BMS.ID, true, nil
	}
	return "", false, nil
}

// tryFallback はテキスト検索で候補を取得しスコアリング採用する
func (r *BMSSearchResolver) tryFallback(ctx context.Context, title, artist string) (string, bool, error) {
	if title == "" {
		return "", false, nil
	}
	cands, err := r.bmsClient.SearchBMSesByTitle(ctx, title, fallbackSearchLimit)
	if err != nil {
		return "", false, err
	}
	if len(cands) == 0 {
		return "", false, nil
	}
	refs := make([]ScoreCandidateRef, len(cands))
	for i, c := range cands {
		refs[i] = ScoreCandidateRef{Title: c.Title, Artist: c.Artist}
	}
	idx, ok := PickBestCandidate(refs, title, artist, fallbackScoreThreshold)
	if !ok {
		return "", false, nil
	}
	return cands[idx].ID, true, nil
}

// persist は md5s に対するリンク UPSERT と bmssearch_bms 取得・UPSERT を行う
func (r *BMSSearchResolver) persist(
	ctx context.Context,
	md5s []string,
	bmsID string,
	source model.BMSSearchSource,
) error {
	now := time.Now()
	links := make([]model.BMSSearchLink, len(md5s))
	for i, m := range md5s {
		links[i] = model.BMSSearchLink{MD5: m, BMSID: bmsID, Source: source, ResolvedAt: now}
	}
	if err := r.bmssearchRepo.UpsertLinks(ctx, links); err != nil {
		return err
	}
	// bmssearch_bms キャッシュ確認 → 未取得なら API
	cached, err := r.bmssearchRepo.GetBMSByID(ctx, bmsID)
	if err != nil {
		return err
	}
	if cached != nil {
		return nil
	}
	apiBMS, err := r.bmsClient.LookupBMS(ctx, bmsID)
	if err != nil {
		return err
	}
	if apiBMS == nil {
		return nil
	}
	return r.bmssearchRepo.UpsertBMS(ctx, gatewayBMSToModel(*apiBMS, now))
}

func gatewayBMSToModel(g gateway.BMSSearchBMS, fetchedAt time.Time) model.BMSSearchBMS {
	var exID *string
	exName := ""
	if g.Exhibition != nil {
		s := g.Exhibition.ID
		exID = &s
		exName = g.Exhibition.Name
	}
	dls := make([]model.BMSSearchURLEntry, len(g.Downloads))
	for i, d := range g.Downloads {
		dls[i] = model.BMSSearchURLEntry{URL: d.URL, Description: d.Description}
	}
	prevs := make([]model.BMSSearchPreview, len(g.Previews))
	for i, p := range g.Previews {
		prevs[i] = model.BMSSearchPreview{Service: p.Service, Parameter: p.Parameter}
	}
	rels := make([]model.BMSSearchURLEntry, len(g.RelatedLinks))
	for i, r := range g.RelatedLinks {
		rels[i] = model.BMSSearchURLEntry{URL: r.URL, Description: r.Description}
	}
	return model.BMSSearchBMS{
		BMSID:          g.ID,
		Title:          g.Title,
		Artist:         g.Artist,
		SubArtist:      g.SubArtist,
		Genre:          g.Genre,
		ExhibitionID:   exID,
		ExhibitionName: exName,
		PublishedAt:    g.PublishedAt,
		Downloads:      dls,
		Previews:       prevs,
		RelatedLinks:   rels,
		FetchedAt:      fetchedAt,
	}
}
```

- [ ] **Step 4: テスト実行 → 成功確認**

実行: `go test ./internal/usecase/ -run TestResolveFor -v`
期待: 全 PASS

- [ ] **Step 5: コミット**

```bash
git add internal/usecase/bmssearch_resolver.go internal/usecase/bmssearch_resolver_test.go internal/usecase/usecase_test.go
git commit -m "feat: BMSSearchResolver で公式/フォールバック解決の共通ロジックを実装"
```

---

### Task 9: LookupBMSSearchUseCase 実装

**Files:**
- Create: `internal/usecase/lookup_bmssearch.go`
- Create: `internal/usecase/lookup_bmssearch_test.go`

- [ ] **Step 1: 失敗するテストを書く**

`internal/usecase/lookup_bmssearch_test.go` を新規作成:

```go
package usecase_test

import (
	"context"
	"testing"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

type fakeChartFolderResolver struct {
	folderFn func(md5 string) (folderHash string, md5sInFolder []string, title, artist string, found bool)
	entryFn  func(md5 string) (title, artist string, found bool)
}

func (f *fakeChartFolderResolver) FindFolderInfoByMD5(_ context.Context, md5 string) (string, []string, string, string, bool, error) {
	fh, m, t, a, ok := f.folderFn(md5)
	return fh, m, t, a, ok, nil
}

func (f *fakeChartFolderResolver) FindOrphanInfoByMD5(_ context.Context, md5 string) (string, string, bool, error) {
	t, a, ok := f.entryFn(md5)
	return t, a, ok, nil
}

func TestLookupBMSSearch_OwnedChart(t *testing.T) {
	bmsClient := &fakeBMSClient{
		patternFn: func(_ context.Context, md5 string) (*gateway.BMSSearchPattern, error) {
			if md5 == "m1" {
				p := &gateway.BMSSearchPattern{}
				p.BMS.ID = "bms-1"
				return p, nil
			}
			return nil, nil
		},
		bmsFn: func(_ context.Context, id string) (*gateway.BMSSearchBMS, error) {
			return &gateway.BMSSearchBMS{ID: id, Title: "Owned"}, nil
		},
		searchFn: func(_ context.Context, _ string, _ int) ([]gateway.BMSSearchBMS, error) { return nil, nil },
	}
	bmssearchRepo := newFakeBMSSearchRepo()
	metaRepo := &mockMetaRepo{}
	resolver := usecase.NewBMSSearchResolver(bmsClient, bmssearchRepo, metaRepo)

	folderResolver := &fakeChartFolderResolver{
		folderFn: func(_ string) (string, []string, string, string, bool) {
			return "folder1", []string{"m0", "m1"}, "Owned", "Artist", true
		},
		entryFn: func(_ string) (string, string, bool) { t.Errorf("orphan path should not be hit"); return "", "", false },
	}

	uc := usecase.NewLookupBMSSearchUseCase(resolver, folderResolver, bmssearchRepo)
	dto, err := uc.Execute(context.Background(), "m1")
	if err != nil {
		t.Fatal(err)
	}
	if dto == nil || !dto.HasInfo || dto.BMSID != "bms-1" {
		t.Errorf("dto = %+v", dto)
	}
	if dto.Source != string(model.BMSSearchSourceOfficial) {
		t.Errorf("source = %q", dto.Source)
	}
}

func TestLookupBMSSearch_OrphanMD5(t *testing.T) {
	bmsClient := &fakeBMSClient{
		patternFn: func(_ context.Context, _ string) (*gateway.BMSSearchPattern, error) {
			p := &gateway.BMSSearchPattern{}
			p.BMS.ID = "bms-orphan"
			return p, nil
		},
		bmsFn: func(_ context.Context, _ string) (*gateway.BMSSearchBMS, error) {
			return &gateway.BMSSearchBMS{ID: "bms-orphan", Title: "Orphan"}, nil
		},
		searchFn: func(_ context.Context, _ string, _ int) ([]gateway.BMSSearchBMS, error) { return nil, nil },
	}
	bmssearchRepo := newFakeBMSSearchRepo()
	metaRepo := &mockMetaRepo{}
	resolver := usecase.NewBMSSearchResolver(bmsClient, bmssearchRepo, metaRepo)

	folderResolver := &fakeChartFolderResolver{
		folderFn: func(_ string) (string, []string, string, string, bool) { return "", nil, "", "", false },
		entryFn:  func(_ string) (string, string, bool) { return "Orphan", "X", true },
	}

	uc := usecase.NewLookupBMSSearchUseCase(resolver, folderResolver, bmssearchRepo)
	dto, err := uc.Execute(context.Background(), "morph")
	if err != nil {
		t.Fatal(err)
	}
	if dto == nil || !dto.HasInfo || dto.BMSID != "bms-orphan" {
		t.Errorf("dto = %+v", dto)
	}
}

func TestLookupBMSSearch_NotResolved(t *testing.T) {
	bmsClient := &fakeBMSClient{
		patternFn: func(_ context.Context, _ string) (*gateway.BMSSearchPattern, error) { return nil, nil },
		bmsFn:     func(_ context.Context, _ string) (*gateway.BMSSearchBMS, error) { return nil, nil },
		searchFn:  func(_ context.Context, _ string, _ int) ([]gateway.BMSSearchBMS, error) { return nil, nil },
	}
	bmssearchRepo := newFakeBMSSearchRepo()
	metaRepo := &mockMetaRepo{}
	resolver := usecase.NewBMSSearchResolver(bmsClient, bmssearchRepo, metaRepo)

	folderResolver := &fakeChartFolderResolver{
		folderFn: func(_ string) (string, []string, string, string, bool) {
			return "f1", []string{"m1"}, "T", "A", true
		},
		entryFn: func(_ string) (string, string, bool) { return "", "", false },
	}

	uc := usecase.NewLookupBMSSearchUseCase(resolver, folderResolver, bmssearchRepo)
	dto, err := uc.Execute(context.Background(), "m1")
	if err != nil {
		t.Fatal(err)
	}
	if dto == nil || dto.HasInfo {
		t.Errorf("dto = %+v, want hasInfo=false", dto)
	}
}
```

- [ ] **Step 2: テスト実行 → 失敗確認**

実行: `go test ./internal/usecase/ -run TestLookupBMSSearch -v`
期待: FAIL（シンボル未定義）

- [ ] **Step 3: 実装**

`internal/usecase/lookup_bmssearch.go` を新規作成:

```go
package usecase

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/app/dto"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

// ChartFolderResolver は md5 から所属フォルダや難易度表エントリ情報を解決する
// （SongdataReader と DifficultyTableRepository を組み合わせる）
type ChartFolderResolver interface {
	// FindFolderInfoByMD5 は md5 が所属する楽曲フォルダ情報を返す。
	// 戻り値: folderHash, フォルダ内全 md5, 楽曲タイトル, 楽曲アーティスト, 見つかったか
	FindFolderInfoByMD5(ctx context.Context, md5 string) (string, []string, string, string, bool, error)

	// FindOrphanInfoByMD5 は未所持 md5 の難易度表エントリから title/artist を解決する。
	FindOrphanInfoByMD5(ctx context.Context, md5 string) (string, string, bool, error)
}

type LookupBMSSearchUseCase struct {
	resolver       *BMSSearchResolver
	folderResolver ChartFolderResolver
	bmssearchRepo  model.BMSSearchRepository
}

func NewLookupBMSSearchUseCase(
	resolver *BMSSearchResolver,
	folderResolver ChartFolderResolver,
	bmssearchRepo model.BMSSearchRepository,
) *LookupBMSSearchUseCase {
	return &LookupBMSSearchUseCase{
		resolver:       resolver,
		folderResolver: folderResolver,
		bmssearchRepo:  bmssearchRepo,
	}
}

func (u *LookupBMSSearchUseCase) Execute(ctx context.Context, md5 string) (*dto.BMSSearchInfoDTO, error) {
	var bmsID string
	var source model.BMSSearchSource

	folderHash, md5s, title, artist, ownedFound, err := u.folderResolver.FindFolderInfoByMD5(ctx, md5)
	if err != nil {
		return nil, err
	}
	if ownedFound {
		bmsID, source, err = u.resolver.ResolveForFolder(ctx, folderHash, md5s, title, artist)
	} else {
		t, a, found, err2 := u.folderResolver.FindOrphanInfoByMD5(ctx, md5)
		if err2 != nil {
			return nil, err2
		}
		if !found {
			return &dto.BMSSearchInfoDTO{HasInfo: false}, nil
		}
		bmsID, source, err = u.resolver.ResolveForOrphanMD5(ctx, md5, t, a)
	}
	if err != nil {
		return nil, err
	}
	if bmsID == "" {
		return &dto.BMSSearchInfoDTO{HasInfo: false}, nil
	}
	bms, err := u.bmssearchRepo.GetBMSByID(ctx, bmsID)
	if err != nil {
		return nil, err
	}
	if bms == nil {
		return &dto.BMSSearchInfoDTO{HasInfo: false}, nil
	}
	return dto.BMSSearchBMSToDTO(*bms, string(source)), nil
}
```

- [ ] **Step 4: 一旦ビルド確認（DTO/コンバーターは Task 12 で追加するため失敗想定）**

実行: `go build ./...`
期待: `dto.BMSSearchInfoDTO` 未定義エラー（Task 12 で対応するため進める）

OR: Task 12（DTO 追加）を先にやってから Task 9 を完了する順序にしてもよい。実装者の判断で。本プランでは Task 9 と Task 12 を続けて完了させてからコミットする。

スキップ: コミットは Task 12 完了後にまとめて。

---

### Task 10: UnlinkBMSSearchUseCase 実装

**Files:**
- Create: `internal/usecase/unlink_bmssearch.go`
- Create: `internal/usecase/unlink_bmssearch_test.go`

- [ ] **Step 1: 失敗するテストを書く**

`internal/usecase/unlink_bmssearch_test.go` を新規作成:

```go
package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

type fakeFolderMD5sResolver struct {
	md5s map[string][]string
}

func (f *fakeFolderMD5sResolver) ListMD5sInFolder(_ context.Context, folderHash string) ([]string, error) {
	return f.md5s[folderHash], nil
}

func TestUnlinkByFolder(t *testing.T) {
	bmssearchRepo := newFakeBMSSearchRepo()
	now := time.Now()
	_ = bmssearchRepo.UpsertLinks(context.Background(), []model.BMSSearchLink{
		{MD5: "m1", BMSID: "b", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
		{MD5: "m2", BMSID: "b", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
		{MD5: "m9", BMSID: "z", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
	})

	var clearedFolder, clearedBmsID, clearedSource string
	metaRepo := &mockMetaRepo{
		updateSongMetaBMSSearchFn: func(_ context.Context, fh, bmsID, src string) error {
			clearedFolder, clearedBmsID, clearedSource = fh, bmsID, src
			return nil
		},
	}
	folderResolver := &fakeFolderMD5sResolver{
		md5s: map[string][]string{"folder1": {"m1", "m2"}},
	}

	uc := usecase.NewUnlinkBMSSearchUseCase(bmssearchRepo, metaRepo, folderResolver)
	if err := uc.UnlinkByFolder(context.Background(), "folder1"); err != nil {
		t.Fatal(err)
	}
	if clearedFolder != "folder1" || clearedBmsID != "" || clearedSource != "" {
		t.Errorf("metaRepo args wrong: %q %q %q", clearedFolder, clearedBmsID, clearedSource)
	}
	if bmssearchRepo.links["m1"] != nil || bmssearchRepo.links["m2"] != nil {
		t.Errorf("links not deleted")
	}
	if bmssearchRepo.links["m9"] == nil {
		t.Errorf("m9 should remain")
	}
}

func TestUnlinkByMD5(t *testing.T) {
	bmssearchRepo := newFakeBMSSearchRepo()
	now := time.Now()
	_ = bmssearchRepo.UpsertLinks(context.Background(), []model.BMSSearchLink{
		{MD5: "morph", BMSID: "b", Source: model.BMSSearchSourceOfficial, ResolvedAt: now},
	})
	metaRepo := &mockMetaRepo{}
	folderResolver := &fakeFolderMD5sResolver{}

	uc := usecase.NewUnlinkBMSSearchUseCase(bmssearchRepo, metaRepo, folderResolver)
	if err := uc.UnlinkByMD5(context.Background(), "morph"); err != nil {
		t.Fatal(err)
	}
	if bmssearchRepo.links["morph"] != nil {
		t.Errorf("link should be deleted")
	}
}
```

- [ ] **Step 2: テスト実行 → 失敗確認**

実行: `go test ./internal/usecase/ -run TestUnlinkBy -v`
期待: FAIL

- [ ] **Step 3: 実装**

`internal/usecase/unlink_bmssearch.go` を新規作成:

```go
package usecase

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

// FolderMD5sResolver はフォルダに含まれる md5 一覧を返す
type FolderMD5sResolver interface {
	ListMD5sInFolder(ctx context.Context, folderHash string) ([]string, error)
}

type UnlinkBMSSearchUseCase struct {
	bmssearchRepo  model.BMSSearchRepository
	metaRepo       model.MetaRepository
	folderResolver FolderMD5sResolver
}

func NewUnlinkBMSSearchUseCase(
	bmssearchRepo model.BMSSearchRepository,
	metaRepo model.MetaRepository,
	folderResolver FolderMD5sResolver,
) *UnlinkBMSSearchUseCase {
	return &UnlinkBMSSearchUseCase{
		bmssearchRepo:  bmssearchRepo,
		metaRepo:       metaRepo,
		folderResolver: folderResolver,
	}
}

// UnlinkByFolder は楽曲フォルダ単位の解除（song_meta.bms_search_id/source を NULL にし、
// フォルダ内全 md5 の bmssearch_bms_id_md5 を DELETE）
func (u *UnlinkBMSSearchUseCase) UnlinkByFolder(ctx context.Context, folderHash string) error {
	if err := u.metaRepo.UpdateSongMetaBMSSearch(ctx, folderHash, "", ""); err != nil {
		return err
	}
	md5s, err := u.folderResolver.ListMD5sInFolder(ctx, folderHash)
	if err != nil {
		return err
	}
	if len(md5s) == 0 {
		return nil
	}
	return u.bmssearchRepo.DeleteLinksByMD5s(ctx, md5s)
}

// UnlinkByMD5 は未所持 md5 単位の解除
func (u *UnlinkBMSSearchUseCase) UnlinkByMD5(ctx context.Context, md5 string) error {
	return u.bmssearchRepo.DeleteLinkByMD5(ctx, md5)
}
```

- [ ] **Step 4: テスト実行 → 成功確認**

実行: `go test ./internal/usecase/ -run TestUnlinkBy -v`
期待: 全 PASS

- [ ] **Step 5: コミット保留（Task 12 と一緒に）**

---

### Task 11: SyncBMSSearchUseCase の Resolver 委譲改修

**Files:**
- Modify: `internal/usecase/sync_bmssearch.go`

- [ ] **Step 1: 既存テストの確認**

実行: `ls internal/usecase/ | grep sync_bmssearch_test`
既存テストがあれば内容を読む。なければスキップ。

- [ ] **Step 2: `SyncBMSSearchUseCase` を Resolver 委譲に書き換え**

`internal/usecase/sync_bmssearch.go` を以下に置換:

```go
package usecase

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

type SyncBMSSearchProgress struct {
	Current int
	Total   int
}

type SyncBMSSearchResult struct {
	Total     int
	Synced    int
	NotFound  int
	Failed    int
	Cancelled bool
}

type SyncBMSSearchUseCase struct {
	resolver      *BMSSearchResolver
	bmssearchRepo model.BMSSearchRepository
	metaRepo      model.MetaRepository
}

func NewSyncBMSSearchUseCase(
	resolver *BMSSearchResolver,
	bmssearchRepo model.BMSSearchRepository,
	metaRepo model.MetaRepository,
) *SyncBMSSearchUseCase {
	return &SyncBMSSearchUseCase{
		resolver:      resolver,
		bmssearchRepo: bmssearchRepo,
		metaRepo:      metaRepo,
	}
}

func (u *SyncBMSSearchUseCase) Execute(
	ctx context.Context,
	folders []string,
	md5sByFolder map[string][]string,
	titleByFolder map[string]string,
	artistByFolder map[string]string,
	progressFn func(SyncBMSSearchProgress),
) (*SyncBMSSearchResult, error) {
	total := len(folders)
	var synced, notFound atomic.Int64
	var completed atomic.Int64

	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup

	for _, folderHash := range folders {
		select {
		case <-ctx.Done():
			wg.Wait()
			return &SyncBMSSearchResult{
				Total: total, Synced: int(synced.Load()), NotFound: int(notFound.Load()),
				Cancelled: true,
			}, nil
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(fh string) {
			defer func() { <-sem; wg.Done() }()

			ok := u.syncFolder(ctx, fh, md5sByFolder[fh], titleByFolder[fh], artistByFolder[fh])
			if ok {
				synced.Add(1)
			} else {
				notFound.Add(1)
			}

			c := int(completed.Add(1))
			if progressFn != nil {
				progressFn(SyncBMSSearchProgress{Current: c, Total: total})
			}
		}(folderHash)
	}
	wg.Wait()

	return &SyncBMSSearchResult{
		Total: total, Synced: int(synced.Load()), NotFound: int(notFound.Load()),
	}, nil
}

func (u *SyncBMSSearchUseCase) syncFolder(
	ctx context.Context,
	folderHash string,
	md5s []string,
	title, artist string,
) bool {
	bmsID, _, err := u.resolver.ResolveForFolder(ctx, folderHash, md5s, title, artist)
	if err != nil || bmsID == "" {
		return false
	}
	// exhibition_id があり、かつローカル event テーブルに対応 event があれば event_id も更新
	bms, err := u.bmssearchRepo.GetBMSByID(ctx, bmsID)
	if err != nil || bms == nil || bms.ExhibitionID == nil {
		return true
	}
	event, err := u.metaRepo.GetEventByBMSSearchID(ctx, *bms.ExhibitionID)
	if err != nil || event == nil {
		return true
	}
	_ = u.metaRepo.UpdateSongMetaEvent(ctx, folderHash, *bms.ExhibitionID, bmsID)
	return true
}
```

- [ ] **Step 3: `EventHandler` の同期呼び出し側を修正**

`internal/app/event_handler.go` の `StartSyncBMSSearch` 系メソッドを探し、`syncBMSSearch.Execute` を呼ぶ箇所で title/artist マップも組み立てて渡すよう変更する。

該当箇所を読む（`grep -n "syncBMSSearch.Execute\|md5sByFolder" internal/app/event_handler.go`）。修正内容:

```go
// Before:
//   md5sByFolder := map[string][]string{}
//   for _, folder := range folders {
//       md5s, _ := h.songdataReader.ListMD5sByFolder(ctx, folder)
//       md5sByFolder[folder] = md5s
//   }
//   result, err := h.syncBMSSearch.Execute(ctx, folders, md5sByFolder, progressFn)

// After:
md5sByFolder := map[string][]string{}
titleByFolder := map[string]string{}
artistByFolder := map[string]string{}
for _, folder := range folders {
    s, _ := h.songdataReader.GetSongByFolder(ctx, folder)
    if s == nil {
        continue
    }
    md5s := make([]string, len(s.Charts))
    for i, c := range s.Charts {
        md5s[i] = c.MD5
    }
    md5sByFolder[folder] = md5s
    titleByFolder[folder] = s.Title
    artistByFolder[folder] = s.Artist
}
result, err := h.syncBMSSearch.Execute(ctx, folders, md5sByFolder, titleByFolder, artistByFolder, progressFn)
```

- [ ] **Step 4: 既存テスト（あれば）を更新**

`internal/usecase/sync_bmssearch_test.go` がある場合、`NewSyncBMSSearchUseCase` のシグネチャが変わったので呼び出しを更新。

- [ ] **Step 5: コミット保留（Task 12 と一緒に）**

---

## Phase 5: ハンドラー層・DI

### Task 12: DTO 追加 + LookupUseCase の動作確認

**Files:**
- Modify: `internal/app/dto/dto.go`

- [ ] **Step 1: DTO とコンバーターを追加**

`internal/app/dto/dto.go` の末尾に追加:

```go
// BMSSearchInfoDTO は詳細画面の BMS Search 情報カードに渡すデータ
type BMSSearchInfoDTO struct {
	HasInfo        bool                   `json:"hasInfo"`
	BMSID          string                 `json:"bmsId,omitempty"`
	Source         string                 `json:"source,omitempty"`
	Title          string                 `json:"title,omitempty"`
	Artist         string                 `json:"artist,omitempty"`
	SubArtist      string                 `json:"subArtist,omitempty"`
	Genre          string                 `json:"genre,omitempty"`
	ExhibitionID   string                 `json:"exhibitionId,omitempty"`
	ExhibitionName string                 `json:"exhibitionName,omitempty"`
	PublishedAt    string                 `json:"publishedAt,omitempty"`
	Downloads      []BMSSearchURLEntryDTO `json:"downloads,omitempty"`
	Previews       []BMSSearchPreviewDTO  `json:"previews,omitempty"`
	RelatedLinks   []BMSSearchURLEntryDTO `json:"relatedLinks,omitempty"`
}

type BMSSearchURLEntryDTO struct {
	URL         string `json:"url"`
	Description string `json:"description"`
}

type BMSSearchPreviewDTO struct {
	Service   string `json:"service"`
	Parameter string `json:"parameter"`
}

func BMSSearchBMSToDTO(b model.BMSSearchBMS, source string) *BMSSearchInfoDTO {
	d := &BMSSearchInfoDTO{
		HasInfo:        true,
		BMSID:          b.BMSID,
		Source:         source,
		Title:          b.Title,
		Artist:         b.Artist,
		SubArtist:      b.SubArtist,
		Genre:          b.Genre,
		ExhibitionName: b.ExhibitionName,
		PublishedAt:    b.PublishedAt,
	}
	if b.ExhibitionID != nil {
		d.ExhibitionID = *b.ExhibitionID
	}
	if len(b.Downloads) > 0 {
		d.Downloads = make([]BMSSearchURLEntryDTO, len(b.Downloads))
		for i, e := range b.Downloads {
			d.Downloads[i] = BMSSearchURLEntryDTO{URL: e.URL, Description: e.Description}
		}
	}
	if len(b.Previews) > 0 {
		d.Previews = make([]BMSSearchPreviewDTO, len(b.Previews))
		for i, p := range b.Previews {
			d.Previews[i] = BMSSearchPreviewDTO{Service: p.Service, Parameter: p.Parameter}
		}
	}
	if len(b.RelatedLinks) > 0 {
		d.RelatedLinks = make([]BMSSearchURLEntryDTO, len(b.RelatedLinks))
		for i, e := range b.RelatedLinks {
			d.RelatedLinks[i] = BMSSearchURLEntryDTO{URL: e.URL, Description: e.Description}
		}
	}
	return d
}
```

- [ ] **Step 2: ビルド確認**

実行: `go build ./...`
期待: 成功（Task 9・10・11 の参照も解決される）

- [ ] **Step 3: 全テスト実行**

実行: `go test ./internal/...`
期待: 全 PASS

- [ ] **Step 4: コミット（Task 9〜12 まとめて）**

```bash
git add internal/usecase/lookup_bmssearch.go internal/usecase/lookup_bmssearch_test.go \
        internal/usecase/unlink_bmssearch.go internal/usecase/unlink_bmssearch_test.go \
        internal/usecase/sync_bmssearch.go internal/app/event_handler.go \
        internal/app/dto/dto.go
git commit -m "feat: Lookup/Unlink/Sync ユースケース改修と BMSSearchInfoDTO 追加"
```

---

### Task 13: ChartFolderResolver / FolderMD5sResolver 実装と SongdataReader 拡張

**Files:**
- Modify: `internal/adapter/persistence/songdata_reader.go`

- [ ] **Step 1: 既存メソッド調査**

実行: `grep -n "ListMD5sByFolder\|GetSongByFolder\|FolderResolver" internal/adapter/persistence/songdata_reader.go`
既存に近いメソッドがあれば再利用する。

- [ ] **Step 2: `SongdataReader` に必要メソッドを追加**

`internal/adapter/persistence/songdata_reader.go` の末尾に追加（既に同等メソッドがある場合は省略）:

```go
// FindFolderInfoByMD5 は md5 から所属フォルダの楽曲情報を解決する。
// 戻り値の bool が false の場合、未所持 md5 として扱う。
func (r *SongdataReader) FindFolderInfoByMD5(ctx context.Context, md5 string) (string, []string, string, string, bool, error) {
	row := r.db.QueryRowContext(ctx, `SELECT folder FROM songdata.song WHERE md5 = ? LIMIT 1`, md5)
	var folder string
	if err := row.Scan(&folder); err != nil {
		if err == sql.ErrNoRows {
			return "", nil, "", "", false, nil
		}
		return "", nil, "", "", false, err
	}
	song, err := r.GetSongByFolder(ctx, folder)
	if err != nil || song == nil {
		return "", nil, "", "", false, err
	}
	md5s := make([]string, len(song.Charts))
	for i, c := range song.Charts {
		md5s[i] = c.MD5
	}
	return folder, md5s, song.Title, song.Artist, true, nil
}

// FindOrphanInfoByMD5 は未所持 md5 の難易度表エントリから title/artist を解決する。
func (r *SongdataReader) FindOrphanInfoByMD5(ctx context.Context, md5 string) (string, string, bool, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(title, ''), COALESCE(artist, '')
		 FROM difficulty_table_entry WHERE md5 = ? LIMIT 1`, md5)
	var title, artist string
	if err := row.Scan(&title, &artist); err != nil {
		if err == sql.ErrNoRows {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	if title == "" {
		return "", "", false, nil
	}
	return title, artist, true, nil
}

// ListMD5sInFolder はフォルダ内の全 md5 を返す。
func (r *SongdataReader) ListMD5sInFolder(ctx context.Context, folderHash string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT md5 FROM songdata.song WHERE folder = ?`, folderHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
```

- [ ] **Step 3: ビルド確認**

実行: `go build ./...`
期待: 成功

- [ ] **Step 4: コミット保留（Task 14 と一緒に）**

---

### Task 14: BMSSearchHandler 実装と DI 組み立て

**Files:**
- Create: `internal/app/bmssearch_handler.go`
- Modify: `app.go`
- Modify: `main.go`

- [ ] **Step 1: ハンドラー実装**

`internal/app/bmssearch_handler.go` を新規作成:

```go
package app

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/app/dto"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

type BMSSearchHandler struct {
	ctx           context.Context
	lookupUC      *usecase.LookupBMSSearchUseCase
	unlinkUC      *usecase.UnlinkBMSSearchUseCase
	bmssearchRepo model.BMSSearchRepository
	metaRepo      model.MetaRepository
	folderResolver usecase.ChartFolderResolver
}

func NewBMSSearchHandler(
	lookupUC *usecase.LookupBMSSearchUseCase,
	unlinkUC *usecase.UnlinkBMSSearchUseCase,
	bmssearchRepo model.BMSSearchRepository,
	metaRepo model.MetaRepository,
	folderResolver usecase.ChartFolderResolver,
) *BMSSearchHandler {
	return &BMSSearchHandler{
		lookupUC:       lookupUC,
		unlinkUC:       unlinkUC,
		bmssearchRepo:  bmssearchRepo,
		metaRepo:       metaRepo,
		folderResolver: folderResolver,
	}
}

func (h *BMSSearchHandler) SetContext(ctx context.Context) { h.ctx = ctx }

// GetBMSSearchInfoByMD5 は DB のみから情報を取得する（API 呼び出しなし）。詳細画面の初期表示用。
func (h *BMSSearchHandler) GetBMSSearchInfoByMD5(md5 string) (*dto.BMSSearchInfoDTO, error) {
	folderHash, _, _, _, owned, err := h.folderResolver.FindFolderInfoByMD5(h.ctx, md5)
	if err != nil {
		return nil, err
	}
	var bmsID string
	var source string
	if owned {
		meta, err := h.metaRepo.GetSongMeta(h.ctx, folderHash)
		if err != nil {
			return nil, err
		}
		if meta == nil || meta.BMSSearchID == nil {
			return &dto.BMSSearchInfoDTO{HasInfo: false}, nil
		}
		bmsID = *meta.BMSSearchID
		if meta.BMSSearchSource != nil {
			source = *meta.BMSSearchSource
		}
	} else {
		link, err := h.bmssearchRepo.GetLinkByMD5(h.ctx, md5)
		if err != nil {
			return nil, err
		}
		if link == nil {
			return &dto.BMSSearchInfoDTO{HasInfo: false}, nil
		}
		bmsID = link.BMSID
		source = string(link.Source)
	}
	bms, err := h.bmssearchRepo.GetBMSByID(h.ctx, bmsID)
	if err != nil {
		return nil, err
	}
	if bms == nil {
		return &dto.BMSSearchInfoDTO{HasInfo: false}, nil
	}
	return dto.BMSSearchBMSToDTO(*bms, source), nil
}

// LookupBMSSearchByMD5 は「取得」ボタン押下時。Resolver 経由で取得＆保存。
func (h *BMSSearchHandler) LookupBMSSearchByMD5(md5 string) (*dto.BMSSearchInfoDTO, error) {
	return h.lookupUC.Execute(h.ctx, md5)
}

// UnlinkBMSSearchByFolder は所持譜面の解除（song_meta + 全 md5 リンク削除）。
func (h *BMSSearchHandler) UnlinkBMSSearchByFolder(folderHash string) error {
	return h.unlinkUC.UnlinkByFolder(h.ctx, folderHash)
}

// UnlinkBMSSearchByMD5 は未所持 md5 の解除。
func (h *BMSSearchHandler) UnlinkBMSSearchByMD5(md5 string) error {
	return h.unlinkUC.UnlinkByMD5(h.ctx, md5)
}
```

- [ ] **Step 2: `app.go` に DI 組み立てを追加**

`app.go` の `App` struct にフィールド追加（`DiffImportHandler` の直後）:

```go
	BMSSearchHandler       *internalapp.BMSSearchHandler
```

`Init()` 内、`a.EventHandler = ...` 行の直後あたりに追加:

```go
	bmssearchRepo := persistence.NewBMSSearchRepository(db)
	bmsResolver := usecase.NewBMSSearchResolver(bmsSearchClient, bmssearchRepo, elsaRepo)
	lookupBMSSearch := usecase.NewLookupBMSSearchUseCase(bmsResolver, songdataReader, bmssearchRepo)
	unlinkBMSSearch := usecase.NewUnlinkBMSSearchUseCase(bmssearchRepo, elsaRepo, songdataReader)
	a.BMSSearchHandler = internalapp.NewBMSSearchHandler(lookupBMSSearch, unlinkBMSSearch, bmssearchRepo, elsaRepo, songdataReader)
```

注意: `syncBMSSearch` の生成も Resolver を使うよう変更する必要がある:

```go
// Before:
//   syncBMSSearch := usecase.NewSyncBMSSearchUseCase(bmsSearchClient, elsaRepo)
// After:
syncBMSSearch := usecase.NewSyncBMSSearchUseCase(bmsResolver, bmssearchRepo, elsaRepo)
```

(順序: bmssearchRepo, bmsResolver の作成を `syncBMSSearch` より先にする)

`startup` 内に追加:

```go
	a.BMSSearchHandler.SetContext(ctx)
```

- [ ] **Step 3: `main.go` の Bind に追加**

`main.go` の `Bind:` スライス（`a.DuplicateHandler` の直後）に追加:

```go
			app.BMSSearchHandler,
```

- [ ] **Step 4: ビルド確認**

実行: `go build ./...`
期待: 成功

- [ ] **Step 5: 全テスト**

実行: `go test ./internal/...`
期待: 全 PASS

- [ ] **Step 6: Wails バインディング再生成**

実行: `wails generate module`（必要な場合）または `wails dev` を一度起動してフロント側 `wailsjs/go/app/BMSSearchHandler.{ts,js}` が生成されることを確認。

期待: `frontend/wailsjs/go/app/BMSSearchHandler.ts` が生成され、`GetBMSSearchInfoByMD5`/`LookupBMSSearchByMD5`/`UnlinkBMSSearchByFolder`/`UnlinkBMSSearchByMD5` がエクスポートされる。

- [ ] **Step 7: コミット**

```bash
git add internal/app/bmssearch_handler.go internal/adapter/persistence/songdata_reader.go app.go main.go frontend/wailsjs/
git commit -m "feat: BMSSearchHandler 追加と Wails バインディング更新"
```

---

## Phase 6: フロントエンド

### Task 15: search アイコンを追加

**Files:**
- Modify: `frontend/src/components/icons.ts`

- [ ] **Step 1: アイコン追加**

`frontend/src/components/icons.ts` の `icons` オブジェクト末尾（`folderMove` の後）に追加:

```typescript
  // heroicons v2: 24/outline/magnifying-glass
  search: {
    viewBox: '0 0 24 24',
    type: 'stroke',
    strokeWidth: 1.5,
    paths: [{ d: 'm21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z' }],
  },
```

- [ ] **Step 2: ビルド確認（フロントエンド）**

実行: `cd frontend && npm run build` または型チェック `npx tsc --noEmit`
期待: 成功

- [ ] **Step 3: コミット**

```bash
git add frontend/src/components/icons.ts
git commit -m "feat: search アイコンを追加"
```

---

### Task 16: BMSSearchInfoCard.svelte 実装

**Files:**
- Create: `frontend/src/components/BMSSearchInfoCard.svelte`

- [ ] **Step 1: コンポーネント作成**

`frontend/src/components/BMSSearchInfoCard.svelte` を新規作成:

```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import type { dto } from '../../wailsjs/go/models'
  import { rewriteRules } from '../stores/rewriteRules'
  import { applyRewriteRules } from '../lib/urlRewrite'
  import Icon from './Icon.svelte'

  export let md5: string
  export let folderHash: string = ''
  export let info: dto.BMSSearchInfoDTO | null = null
  export let loading = false

  const dispatch = createEventDispatcher<{
    lookup: void
    unlink: void
  }>()

  $: hasInfo = info?.hasInfo === true

  function previewUrl(p: dto.BMSSearchPreviewDTO): string {
    switch (p.service) {
      case 'YOUTUBE':
        return `https://www.youtube.com/watch?v=${p.parameter}`
      case 'NICONICO':
        return `https://www.nicovideo.jp/watch/${p.parameter}`
      case 'SOUNDCLOUD':
      default:
        return p.parameter
    }
  }

  function rewrite(url: string): string {
    return applyRewriteRules(url, $rewriteRules)
  }
</script>

<div class="bg-base-200 rounded-lg p-3">
  <div class="flex items-center justify-between mb-2">
    <h3 class="text-sm font-semibold flex items-center gap-1">
      {#if info?.bmsId}
        <a href="https://bmssearch.net/bmses/{info.bmsId}" target="_blank" rel="noopener noreferrer" class="link link-primary">BMS Search情報</a>
      {:else}
        BMS Search情報
      {/if}
      {#if info?.source === 'unofficial'}
        <span class="tooltip tooltip-right" data-tip="テキスト検索により自動推定された紐付けです">
          <Icon name="search" cls="h-3.5 w-3.5 text-warning" />
        </span>
      {/if}
    </h3>
    <div class="flex items-center gap-1">
      <button class="btn btn-ghost btn-xs" disabled={loading} on:click={() => dispatch('lookup')}>
        {#if loading}
          <span class="loading loading-spinner loading-xs"></span>
        {:else}
          取得
        {/if}
      </button>
      {#if hasInfo}
        <button class="btn btn-ghost btn-xs" on:click={() => dispatch('unlink')}>解除</button>
      {/if}
    </div>
  </div>
  {#if hasInfo && info}
    <div class="text-xs space-y-1">
      {#if info.title}
        <p><span class="font-semibold">タイトル:</span> {info.title}</p>
      {/if}
      {#if info.artist}
        <p><span class="font-semibold">アーティスト:</span> {info.artist}</p>
      {/if}
      {#if info.subArtist}
        <p><span class="font-semibold">サブアーティスト:</span> {info.subArtist}</p>
      {/if}
      {#if info.genre}
        <p><span class="font-semibold">ジャンル:</span> {info.genre}</p>
      {/if}
      {#if info.exhibitionName}
        <p>
          <span class="font-semibold">イベント:</span>
          {#if info.exhibitionId}
            <a href="https://bmssearch.net/exhibitions/{info.exhibitionId}" target="_blank" rel="noopener noreferrer" class="link link-primary">{info.exhibitionName}</a>
          {:else}
            {info.exhibitionName}
          {/if}
        </p>
      {/if}
      {#if info.publishedAt}
        <p><span class="font-semibold">公開日:</span> {info.publishedAt}</p>
      {/if}
      {#if info.downloads?.length}
        <div>
          <span class="font-semibold">DLリンク:</span>
          <ul class="ml-4 list-disc">
            {#each info.downloads as d}
              <li>
                <a href={rewrite(d.url)} target="_blank" rel="noopener noreferrer" class="link link-primary">{rewrite(d.url)}</a>
                {#if d.description}<span class="text-base-content/60">— {d.description}</span>{/if}
              </li>
            {/each}
          </ul>
        </div>
      {/if}
      {#if info.previews?.length}
        <div>
          <span class="font-semibold">プレビュー:</span>
          <ul class="ml-4 list-disc">
            {#each info.previews as p}
              <li>
                <a href={previewUrl(p)} target="_blank" rel="noopener noreferrer" class="link link-primary">{p.service}: {previewUrl(p)}</a>
              </li>
            {/each}
          </ul>
        </div>
      {/if}
      {#if info.relatedLinks?.length}
        <div>
          <span class="font-semibold">関連リンク:</span>
          <ul class="ml-4 list-disc">
            {#each info.relatedLinks as r}
              <li>
                <a href={rewrite(r.url)} target="_blank" rel="noopener noreferrer" class="link link-primary">{rewrite(r.url)}</a>
                {#if r.description}<span class="text-base-content/60">— {r.description}</span>{/if}
              </li>
            {/each}
          </ul>
        </div>
      {/if}
    </div>
  {:else}
    <p class="text-xs text-base-content/50">BMS Search情報がありません。「取得」ボタンで取得してください。</p>
  {/if}
</div>
```

- [ ] **Step 2: 型チェック**

実行: `cd frontend && npx tsc --noEmit`
期待: エラーなし（`dto.BMSSearchInfoDTO` 等が解決できること）。解決できない場合は `frontend/wailsjs/go/models.ts` を再生成（`wails dev` を一度起動）。

- [ ] **Step 3: コミット**

```bash
git add frontend/src/components/BMSSearchInfoCard.svelte
git commit -m "feat: BMSSearchInfoCard コンポーネント追加"
```

---

### Task 17: SongDetail への配置

**Files:**
- Modify: `frontend/src/views/SongDetail.svelte`

- [ ] **Step 1: import と関数追加**

`SongDetail.svelte` の `<script>` ブロック上部のインポート群に追加:

```typescript
  import BMSSearchInfoCard from '../components/BMSSearchInfoCard.svelte'
  import {
    GetBMSSearchInfoByMD5,
    LookupBMSSearchByMD5,
    UnlinkBMSSearchByFolder,
  } from '../../wailsjs/go/app/BMSSearchHandler'
```

state に追加（`let loading = false` の直後あたり）:

```typescript
  let bmsSearchInfo: dto.BMSSearchInfoDTO | null = null
  let bmsSearchLoading = false
```

`loadDetail` 関数の中、`detail = await GetSongDetail(hash)` の後に追加:

```typescript
      // BMS Search 情報の並列取得（DB 読みのみ）
      if (detail?.charts?.[0]?.md5) {
        try {
          bmsSearchInfo = await GetBMSSearchInfoByMD5(detail.charts[0].md5)
        } catch (e) {
          console.error('Failed to load BMS Search info:', e)
          bmsSearchInfo = null
        }
      } else {
        bmsSearchInfo = null
      }
```

`<script>` 末尾に関数追加（`closeResult` の後）:

```typescript
  async function lookupBMSSearch() {
    if (!detail?.charts?.[0]?.md5) return
    bmsSearchLoading = true
    try {
      bmsSearchInfo = await LookupBMSSearchByMD5(detail.charts[0].md5)
    } finally {
      bmsSearchLoading = false
    }
  }

  async function unlinkBMSSearch() {
    if (!detail) return
    await UnlinkBMSSearchByFolder(detail.folderHash)
    bmsSearchInfo = await GetBMSSearchInfoByMD5(detail.charts[0]?.md5 ?? '')
  }
```

- [ ] **Step 2: テンプレートに配置**

`<!-- 譜面一覧 -->` ブロックの**直前**に挿入:

```svelte
    <!-- BMS Search 情報（楽曲レベル） -->
    {#if detail.charts.length > 0}
      <BMSSearchInfoCard
        md5={detail.charts[0].md5}
        folderHash={detail.folderHash}
        info={bmsSearchInfo}
        loading={bmsSearchLoading}
        on:lookup={lookupBMSSearch}
        on:unlink={unlinkBMSSearch}
      />
    {/if}
```

- [ ] **Step 3: 型チェック**

実行: `cd frontend && npx tsc --noEmit`
期待: エラーなし

- [ ] **Step 4: コミット保留（Task 19 と一緒に）**

---

### Task 18: ChartDetail への配置

**Files:**
- Modify: `frontend/src/views/ChartDetail.svelte`

- [ ] **Step 1: import と state 追加**

`<script>` のインポートに追加:

```typescript
  import BMSSearchInfoCard from '../components/BMSSearchInfoCard.svelte'
  import {
    GetBMSSearchInfoByMD5,
    LookupBMSSearchByMD5,
    UnlinkBMSSearchByFolder,
  } from '../../wailsjs/go/app/BMSSearchHandler'
```

state 追加:

```typescript
  let bmsSearchInfo: dto.BMSSearchInfoDTO | null = null
  let bmsSearchLoading = false
```

`loadChart` 関数の `chart = await GetChartDetailByMD5(...)` の後に追加:

```typescript
      try {
        bmsSearchInfo = await GetBMSSearchInfoByMD5(hash)
      } catch (e) {
        console.error('Failed to load BMS Search info:', e)
        bmsSearchInfo = null
      }
```

関数を追加:

```typescript
  async function lookupBMSSearch() {
    bmsSearchLoading = true
    try {
      bmsSearchInfo = await LookupBMSSearchByMD5(md5)
    } finally {
      bmsSearchLoading = false
    }
  }

  async function unlinkBMSSearch() {
    if (!folderHash) return
    await UnlinkBMSSearchByFolder(folderHash)
    bmsSearchInfo = await GetBMSSearchInfoByMD5(md5)
  }
```

- [ ] **Step 2: テンプレートに配置**

`{#if chart}` ブロックの中、`<ChartInfoCard {chart} />` と `<IRInfoCard ... />` の**間**に挿入:

```svelte
      <BMSSearchInfoCard
        {md5}
        {folderHash}
        info={bmsSearchInfo}
        loading={bmsSearchLoading}
        on:lookup={lookupBMSSearch}
        on:unlink={unlinkBMSSearch}
      />
```

- [ ] **Step 3: 型チェック**

実行: `cd frontend && npx tsc --noEmit`
期待: エラーなし

- [ ] **Step 4: コミット保留**

---

### Task 19: EntryDetail への配置

**Files:**
- Modify: `frontend/src/views/EntryDetail.svelte`

- [ ] **Step 1: import と state 追加**

`<script>` のインポートに追加:

```typescript
  import BMSSearchInfoCard from '../components/BMSSearchInfoCard.svelte'
  import {
    GetBMSSearchInfoByMD5,
    LookupBMSSearchByMD5,
    UnlinkBMSSearchByFolder,
    UnlinkBMSSearchByMD5,
  } from '../../wailsjs/go/app/BMSSearchHandler'
```

state 追加:

```typescript
  let bmsSearchInfo: dto.BMSSearchInfoDTO | null = null
  let bmsSearchLoading = false
```

`loadEntry` 関数の末尾（`if (!chart) { irMeta = ... }` の後）に追加:

```typescript
      try {
        bmsSearchInfo = await GetBMSSearchInfoByMD5(hash)
      } catch (e) {
        console.error('Failed to load BMS Search info:', e)
        bmsSearchInfo = null
      }
```

関数追加:

```typescript
  async function lookupBMSSearch() {
    bmsSearchLoading = true
    try {
      bmsSearchInfo = await LookupBMSSearchByMD5(md5)
    } finally {
      bmsSearchLoading = false
    }
  }

  async function unlinkBMSSearch() {
    // 導入済（chart あり）なら folder 単位、未導入なら md5 単位
    if (chart?.path) {
      // chart に folderHash が含まれていない場合は md5 単位フォールバック
      // SongdataReader 側の取り扱いに合わせる
      // ここでは chart があれば導入済として扱う
      // path から folder を引き出すのは backend 側の責務なので md5 単位でよければ後者を使う
    }
    if (chart) {
      // 導入済: フォルダ単位の解除のために、まず GetSongDetail 経由で folderHash を引く必要があるが、
      // EntryDetail は folderHash を直接持たない。
      // → ここでは md5 単位の解除に統一する（Backend 側で md5 → folder を引いて両系列を消すのが本来理想）。
      await UnlinkBMSSearchByMD5(md5)
    } else {
      await UnlinkBMSSearchByMD5(md5)
    }
    bmsSearchInfo = await GetBMSSearchInfoByMD5(md5)
  }
```

注意: 上の TODO 的な分岐は実装の単純化のため EntryDetail では md5 単位解除に統一する。導入済譜面で楽曲フォルダ全体の解除をしたい場合は楽曲詳細から行う（マニュアルでも明記）。

- [ ] **Step 2: テンプレートに配置**

`<IRInfoCard {md5} {ir} on:lookup={lookupIR} />` の**直後**に挿入:

```svelte
    <BMSSearchInfoCard
      {md5}
      info={bmsSearchInfo}
      loading={bmsSearchLoading}
      on:lookup={lookupBMSSearch}
      on:unlink={unlinkBMSSearch}
    />
```

- [ ] **Step 3: 型チェック**

実行: `cd frontend && npx tsc --noEmit`
期待: エラーなし

- [ ] **Step 4: フロントエンドビルド**

実行: `cd frontend && npm run build`
期待: 成功

- [ ] **Step 5: コミット**

```bash
git add frontend/src/views/SongDetail.svelte frontend/src/views/ChartDetail.svelte frontend/src/views/EntryDetail.svelte
git commit -m "feat: 詳細画面に BMSSearchInfoCard を配置"
```

---

## Phase 7: マニュアル更新と QA

### Task 20: マニュアル更新

**Files:**
- Modify: `docs/manual.md`

- [ ] **Step 1: 該当セクション更新**

`docs/manual.md` の「BMS Search同期」セクションの末尾、および詳細画面の説明セクションに以下を追記する。

「BMS Search同期」セクションへの追記（既存記述の末尾に追加）:

```markdown

### フォールバック検索（自動推定）

公式の md5 一致でヒットしなかった楽曲については、タイトルとアーティストによるテキスト検索で自動推定を試みます。
スコアが閾値以上で唯一の最高得点となった候補のみ採用し、`unofficial`（自動推定）として保存されます。
推定された紐付けは詳細画面の BMS Search 情報カードに虫眼鏡アイコンで表示され、ツールチップで判別できます。
```

詳細画面セクション（楽曲詳細・譜面詳細・難易度表エントリ詳細の説明箇所）への追記:

```markdown

### BMS Search 情報カード

楽曲詳細・譜面詳細・難易度表エントリ詳細画面には「BMS Search情報」カードが表示されます。
カードには BMS Search に登録されている楽曲のタイトル・アーティスト・サブアーティスト・ジャンル・イベント・公開日・ダウンロードリンク・プレビュー（YouTube/SoundCloud/NicoNico）・関連リンクが表示されます。

- **「取得」ボタン**: BMS Search API から楽曲メタデータを取得して保存します。所持譜面の場合はフォルダ内の譜面の md5 を順に試行し、見つからなければタイトル/アーティストでフォールバック検索します。
- **「解除」ボタン**: 保存されている紐付けを削除します。BMS Search 情報カードは「情報なし」表示に戻ります。
- **虫眼鏡アイコン**: アイコンが付いている場合、テキスト検索による自動推定（unofficial）であることを示します。誤推定だった場合は「解除」してから「取得」をやり直すか、無視してください。
- **DLリンク・関連リンク**: 設定画面で登録した URL 書き換えルールが自動適用されます。
```

- [ ] **Step 2: コミット**

```bash
git add docs/manual.md
git commit -m "docs: BMS Search 情報カードのマニュアル追加"
```

---

### Task 21: 全体ビルド・テスト・手動 QA

**Files:** なし（ビルドと手動確認のみ）

- [ ] **Step 1: 全体ビルド**

実行: `go build ./... && cd frontend && npm run build && cd ..`
期待: 全成功

- [ ] **Step 2: 全テスト**

実行: `go test ./...`
期待: 全 PASS

- [ ] **Step 3: 手動 QA チェックリスト**

`wails dev` で起動し、以下を順に確認する。すべて確認できたらチェックを入れる。

- [ ] 詳細画面オープンで既存表示が壊れていない（既存 IRInfoCard・楽曲ヘッダー）
- [ ] 楽曲詳細で「取得」ボタン → 公式ヒット楽曲 → official 表示・bmssearch_bms 内容反映
- [ ] 公式ミス楽曲 → フォールバック発動 → unofficial 虫眼鏡表示
- [ ] 「解除」ボタン → カードが「情報なし」表示に戻る
- [ ] 既存「BMS Search同期」（一括手動）が新スキーマにも書く（DB を確認: `bmssearch_bms_id_md5` と `bmssearch_bms` に行が入る）
- [ ] 譜面詳細で「取得」「解除」が動作
- [ ] 未所持 md5（難易度表エントリ）で「取得」ボタン動作
- [ ] UI 上の URL書き換えルールが DLリンク・関連リンクに適用されている
- [ ] 同フォルダの別譜面を選んでも同じ BMS Search 情報が表示される（共有キャッシュ動作）
- [ ] プレビューリンク（YouTube/SoundCloud/NicoNico）が正しい URL で開く
- [ ] イベントリンク（exhibition）が `https://bmssearch.net/exhibitions/{id}` を開く

- [ ] **Step 4: 最終コミット（必要なら）**

QA で見つかった軽微な修正があれば追加コミット。

```bash
git status
# 変更があればコミット
```

- [ ] **Step 5: PR 作成準備**

実行: `git log --oneline main..HEAD`
期待: 全 Task のコミットが順序通り並んでいる

PR 作成は別途ユーザー指示で行う。

---

## 完了条件

- すべてのチェックボックスにチェックが入っている
- `go test ./...` が全 PASS
- `go build ./...` と `cd frontend && npm run build` が成功
- 手動 QA チェックリストの全項目が確認済み
- マニュアルが更新されている

