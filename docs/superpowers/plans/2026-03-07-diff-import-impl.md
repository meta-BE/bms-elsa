# 新規差分導入画面 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** BMS/BME/BMLファイルをD&Dし、WAV MinHash・LR2IR・タイトルマッチで導入先を推定してファイル移動する「差分導入」タブを追加する。

**Architecture:** BMSパーサーを拡張してヘッダー情報＋MD5を返すようにし、MinHash類似度検索（ベンチマークで方式決定）→IR問い合わせ→タイトルマッチの3段階推定ユースケースを構築。フロントエンドに5つ目のタブを追加し、D&Dでファイルを受け取りテーブル表示。

**Tech Stack:** Go 1.24, Wails v2, Svelte 4 + TypeScript, modernc.org/sqlite, TanStack Table

---

### Task 1: BMSパーサー拡張 — ParseBMSFile の実装

**Files:**
- Modify: `internal/domain/bms/parser.go`
- Modify: `internal/domain/bms/parser_test.go`

**参考ドキュメント:**
- `docs/plans/2026-03-07-diff-import-design.md` — ParsedBMS構造体定義
- 既存の `ParseWAVFiles` — RANDOM対応のロジック

**Step 1: ParsedBMS構造体と ParseBMSFile関数を実装**

`internal/domain/bms/parser.go` に以下を追加し、`ParseWAVFiles` を削除する。

```go
type ParsedBMS struct {
	MD5       string
	Title     string
	Subtitle  string
	Artist    string
	Subartist string
	Genre     string
	WAVFiles  []string
}
```

`ParseBMSFile(path string) (*ParsedBMS, error)`:
- ファイル全体を `os.ReadFile` で読み込み、`crypto/md5` でMD5を計算
- 行ごとにスキャンし、既存のRANDOMステートマシン（stack + skipDepth）を流用
- `#TITLE`, `#SUBTITLE`, `#ARTIST`, `#SUBARTIST`, `#GENRE` を大文字比較で検出し、最初にヒットした値を採用（RANDOM内の `#IF 1` も既存ルールで処理）
- WAV定義の処理は既存ロジックをそのまま移植
- ヘッダーフィールドは「最初に出現した値を採用」（RANDOM外で先に定義されていればそれが優先、RANDOM内の`#IF 1`で初出の場合はそれを採用）

**Step 2: ParseWAVFiles の呼び出し元を置換**

`ParseWAVFiles` を使用している箇所を検索し、`ParseBMSFile` + `.WAVFiles` に置換する。

該当箇所:
- `internal/app/scan_handler.go:84` — `bms.ParseWAVFiles(tgt.Path)` → `parsed, err := bms.ParseBMSFile(tgt.Path)` + `parsed.WAVFiles`
- `internal/domain/bms/parser_test.go` — テスト内の `bms.ParseWAVFiles(...)` を `bms.ParseBMSFile(...)` + `.WAVFiles` に変更

**Step 3: テストを追加**

`parser_test.go` に以下のテストを追加:

```go
func TestParseBMSFile_HeaderFields(t *testing.T) {
	path := filepath.Join(testdataDir(t), "[Clue]Random", "_random_s4.bms")
	parsed, err := bms.ParseBMSFile(path)
	if err != nil {
		t.Fatalf("ParseBMSFile failed: %v", err)
	}
	// RANDOM内の#IF 1: #GENRE "II - Fo1lowin¡Ì t¡Ìe C1¡Ìe .. ." （文字化けした値）
	// RANDOM外: #TITLE "Random [SP ANOTHER]"
	if parsed.Title != "Random [SP ANOTHER]" {
		t.Errorf("expected title 'Random [SP ANOTHER]', got %q", parsed.Title)
	}
	// #ARTIST はRANDOM内の#IF 1で定義
	// テストデータの実際の値を確認してから期待値を調整
	if parsed.Artist == "" {
		t.Error("artist should not be empty")
	}
	// WAVFiles は既存テストと同じ件数
	if len(parsed.WAVFiles) != 1063 {
		t.Errorf("expected 1063 WAV files, got %d", len(parsed.WAVFiles))
	}
	// MD5は空でないことを確認
	if parsed.MD5 == "" {
		t.Error("MD5 should not be empty")
	}
	if len(parsed.MD5) != 32 {
		t.Errorf("MD5 should be 32 hex chars, got %d", len(parsed.MD5))
	}
}

func TestParseBMSFile_NonRandomHeaders(t *testing.T) {
	path := filepath.Join(testdataDir(t), "[Feryquitous]Distorv", "Dstorv_act1_ego.bme")
	parsed, err := bms.ParseBMSFile(path)
	if err != nil {
		t.Fatalf("ParseBMSFile failed: %v", err)
	}
	if parsed.Title == "" {
		t.Error("title should not be empty")
	}
	if parsed.Artist == "" {
		t.Error("artist should not be empty")
	}
	if len(parsed.WAVFiles) != 631 {
		t.Errorf("expected 631 WAV files, got %d", len(parsed.WAVFiles))
	}
}
```

**Step 4: テスト実行**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
go test ./internal/domain/bms/... -v
```

期待: 全テスト PASS（既存テストも新規テストも）

**Step 5: コミット**

```bash
git add internal/domain/bms/parser.go internal/domain/bms/parser_test.go internal/app/scan_handler.go
git commit -m "feat: ParseBMSFileを実装しヘッダー+MD5パースを追加、ParseWAVFilesを置換"
```

---

### Task 2: MinHash類似度検索ベンチマーク

**Files:**
- Create: `internal/domain/bms/minhash_bench_test.go`

**参考ドキュメント:**
- `docs/plans/2026-03-07-diff-import-design.md` — MinHashベンチマーク計画
- `internal/domain/bms/minhash.go` — MinHashSignature, Similarity, MinHashFromBytes

**目的:** SQLiteカスタム関数方式 vs Go全件スキャン方式のパフォーマンスを比較する。

**Step 1: ベンチマークテスト作成**

`internal/domain/bms/minhash_bench_test.go`:

```go
package bms_test

import (
	"crypto/rand"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"modernc.org/sqlite"

	"github.com/meta-BE/bms-elsa/internal/domain/bms"
)

func init() {
	sqlite.MustRegisterDeterministicScalarFunction(
		"minhash_similarity",
		2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			blob1, ok1 := args[0].([]byte)
			blob2, ok2 := args[1].([]byte)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("expected BLOB arguments")
			}
			if len(blob1) != 256 || len(blob2) != 256 {
				return 0.0, nil
			}
			match := 0
			for i := 0; i < 64; i++ {
				v1 := binary.LittleEndian.Uint32(blob1[i*4:])
				v2 := binary.LittleEndian.Uint32(blob2[i*4:])
				if v1 == v2 {
					match++
				}
			}
			return float64(match) / 64.0, nil
		},
	)
}

// generateRandomMinhash はランダムなMinHash署名を生成する
func generateRandomMinhash() []byte {
	buf := make([]byte, 256)
	rand.Read(buf)
	return buf
}

func BenchmarkMinHashSimilarity_GoScan(b *testing.B) {
	// 3000件のユニークminhashを生成（実データの想定サイズ）
	const numRecords = 3000
	records := make([]bms.MinHashSignature, numRecords)
	for i := range records {
		buf := generateRandomMinhash()
		sig, _ := bms.MinHashFromBytes(buf)
		records[i] = sig
	}

	// クエリ用minhash
	queryBuf := generateRandomMinhash()
	query, _ := bms.MinHashFromBytes(queryBuf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bestSim := 0.0
		bestIdx := -1
		for j, rec := range records {
			sim := query.Similarity(rec)
			if sim > bestSim {
				bestSim = sim
				bestIdx = j
			}
		}
		_ = bestIdx
	}
}

func BenchmarkMinHashSimilarity_SQLiteCustomFunc(b *testing.B) {
	const numRecords = 3000

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	_, err = db.Exec(`CREATE TABLE chart_meta (md5 TEXT PRIMARY KEY, wav_minhash BLOB)`)
	if err != nil {
		b.Fatal(err)
	}

	// データ投入
	tx, _ := db.Begin()
	stmt, _ := tx.Prepare(`INSERT INTO chart_meta (md5, wav_minhash) VALUES (?, ?)`)
	for i := 0; i < numRecords; i++ {
		stmt.Exec(fmt.Sprintf("md5_%d", i), generateRandomMinhash())
	}
	stmt.Close()
	tx.Commit()

	queryMinhash := generateRandomMinhash()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var bestMD5 string
		var bestSim float64
		err := db.QueryRow(
			`SELECT md5, minhash_similarity(?, wav_minhash) as sim
			 FROM chart_meta
			 WHERE wav_minhash IS NOT NULL
			 ORDER BY sim DESC LIMIT 1`,
			queryMinhash,
		).Scan(&bestMD5, &bestSim)
		if err != nil {
			b.Fatal(err)
		}
	}
}
```

**Step 2: ベンチマーク実行**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
go test ./internal/domain/bms/... -bench=BenchmarkMinHash -benchmem -count=3
```

結果を比較し、採用する方式を決定する。

**Step 3: コミット**

```bash
git add internal/domain/bms/minhash_bench_test.go
git commit -m "bench: MinHash類似度検索のSQLiteカスタム関数 vs Go全件スキャンを比較"
```

---

### Task 3: MinHash類似度検索の本実装

**Files:**
- Create or Modify: 採用方式に応じて決定
- Modify: `internal/adapter/persistence/elsa_repository.go` — 新規クエリメソッド追加

**注意:** Task 2のベンチマーク結果に基づいて実装方式を決定する。以下は両方式の実装を記載。

**方式A（SQLiteカスタム関数）の場合:**

`app.go` の `Init()` で関数登録は不要（`init()` でグローバル登録済み）。

`internal/adapter/persistence/elsa_repository.go` に追加:

```go
// MinHashMatch はMinHash類似度検索の結果
type MinHashMatch struct {
	MD5        string
	FolderPath string
	Similarity float64
}

// FindMostSimilarByMinHash はクエリminhashに最も類似するレコードを返す
func (r *ElsaRepository) FindMostSimilarByMinHash(ctx context.Context, queryMinhash []byte, threshold float64) (*MinHashMatch, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT cm.md5, s.path, minhash_similarity(?, cm.wav_minhash) as sim
		 FROM chart_meta cm
		 JOIN songdata.song s ON cm.md5 = s.md5
		 WHERE cm.wav_minhash IS NOT NULL
		 ORDER BY sim DESC
		 LIMIT 1`,
		queryMinhash,
	)
	var m MinHashMatch
	if err := row.Scan(&m.MD5, &m.FolderPath, &m.Similarity); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if m.Similarity < threshold {
		return nil, nil
	}
	// パスからフォルダを抽出
	m.FolderPath = filepath.Dir(m.FolderPath)
	return &m, nil
}
```

**方式B（Go全件スキャン）の場合:**

```go
// FindMostSimilarByMinHash はクエリminhashに最も類似するレコードを返す
func (r *ElsaRepository) FindMostSimilarByMinHash(ctx context.Context, queryMinhash []byte, threshold float64) (*MinHashMatch, error) {
	query, err := bms.MinHashFromBytes(queryMinhash)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT cm.md5, cm.wav_minhash, s.path
		 FROM chart_meta cm
		 JOIN songdata.song s ON cm.md5 = s.md5
		 WHERE cm.wav_minhash IS NOT NULL
		 GROUP BY cm.wav_minhash`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var best *MinHashMatch
	for rows.Next() {
		var md5 string
		var blob []byte
		var path string
		if err := rows.Scan(&md5, &blob, &path); err != nil {
			return nil, err
		}
		sig, err := bms.MinHashFromBytes(blob)
		if err != nil {
			continue
		}
		sim := query.Similarity(sig)
		if sim >= threshold && (best == nil || sim > best.Similarity) {
			best = &MinHashMatch{MD5: md5, FolderPath: filepath.Dir(path), Similarity: sim}
		}
	}
	return best, rows.Err()
}
```

**Step 1: 採用方式で実装**

ベンチマーク結果に基づき、上記のいずれかを `elsa_repository.go` に追加する。方式Aの場合は `minhash_bench_test.go` の `init()` 内のカスタム関数登録を適切な場所（`internal/adapter/persistence/` 配下のinit等）に移動する。

**Step 2: テスト**

```bash
go test ./internal/adapter/persistence/... -v
go test ./... -count=1
```

**Step 3: コミット**

```bash
git add internal/adapter/persistence/elsa_repository.go
# 方式Aの場合は関数登録ファイルも追加
git commit -m "feat: MinHash類似度検索を実装（[方式名]）"
```

---

### Task 4: ファイル移動ユーティリティ

**Files:**
- Create: `internal/domain/fileutil/move.go`
- Create: `internal/domain/fileutil/move_test.go`

**Step 1: テスト作成**

```go
package fileutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/meta-BE/bms-elsa/internal/domain/fileutil"
)

func TestMoveFileToFolder(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	destDir := filepath.Join(tmpDir, "dest")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(destDir, 0755)

	// テストファイル作成
	srcPath := filepath.Join(srcDir, "test.bms")
	os.WriteFile(srcPath, []byte("test content"), 0644)

	// 移動実行
	err := fileutil.MoveFileToFolder(srcPath, destDir)
	if err != nil {
		t.Fatalf("MoveFileToFolder failed: %v", err)
	}

	// 移動先に存在することを確認
	destPath := filepath.Join(destDir, "test.bms")
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("file should exist at dest: %v", err)
	}

	// 移動元から消えていることを確認
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Error("file should not exist at src")
	}
}

func TestMoveFileToFolder_DestFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	destDir := filepath.Join(tmpDir, "dest")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(destDir, 0755)

	srcPath := filepath.Join(srcDir, "test.bms")
	os.WriteFile(srcPath, []byte("new"), 0644)

	// 移動先に同名ファイルが既に存在
	destPath := filepath.Join(destDir, "test.bms")
	os.WriteFile(destPath, []byte("existing"), 0644)

	err := fileutil.MoveFileToFolder(srcPath, destDir)
	if err == nil {
		t.Fatal("should return error when dest file exists")
	}
}

func TestMoveFileToFolder_DestDirNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "test.bms")
	os.WriteFile(srcPath, []byte("test"), 0644)

	err := fileutil.MoveFileToFolder(srcPath, filepath.Join(tmpDir, "nonexistent"))
	if err == nil {
		t.Fatal("should return error when dest dir does not exist")
	}
}
```

**Step 2: テスト実行（失敗確認）**

```bash
go test ./internal/domain/fileutil/... -v
```

**Step 3: 実装**

`internal/domain/fileutil/move.go`:

```go
package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// MoveFileToFolder はファイルを指定フォルダに移動する。
// 移動先に同名ファイルが存在する場合、移動先フォルダが存在しない場合はエラーを返す。
func MoveFileToFolder(srcPath, destFolder string) error {
	if _, err := os.Stat(destFolder); err != nil {
		return fmt.Errorf("移動先フォルダが存在しません: %s (%w)", destFolder, err)
	}

	filename := filepath.Base(srcPath)
	destPath := filepath.Join(destFolder, filename)

	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("移動先に同名ファイルが既に存在します: %s", destPath)
	}

	return os.Rename(srcPath, destPath)
}
```

**Step 4: テスト実行（成功確認）**

```bash
go test ./internal/domain/fileutil/... -v
```

**Step 5: コミット**

```bash
git add internal/domain/fileutil/
git commit -m "feat: 汎用ファイル移動ユーティリティを追加"
```

---

### Task 5: 差分導入ユースケース

**Files:**
- Create: `internal/usecase/estimate_diff_install.go`
- Create: `internal/usecase/execute_diff_import.go`

**参考:**
- `internal/usecase/estimate_install_location.go` — 既存の導入先推定ユースケース
- `internal/domain/bms/parser.go` — ParseBMSFile
- `internal/domain/bms/minhash.go` — ComputeMinHash
- `internal/port/ir_client.go` — IRClient
- `internal/domain/fileutil/move.go` — MoveFileToFolder

**Step 1: EstimateDiffInstallUseCase 実装（統一スコア方式）**

`internal/usecase/estimate_diff_install.go`:

```go
package usecase

import (
	"context"
	"path/filepath"
	"sort"
	"time"

	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
	"github.com/meta-BE/bms-elsa/internal/domain/bms"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/port"
)

const (
	minhashScoreMultiplier = 10.0 // MinHash類似度 × 10 = MinHashスコア（最大10点）
	irSkipThreshold        = 8.0  // MinHashスコアがこの値以上ならIR問い合わせをスキップ
)

// ImportCandidate は差分導入の推定結果
type ImportCandidate struct {
	FilePath    string
	FileName    string
	Title       string
	Subtitle    string
	Artist      string
	Subartist   string
	Genre       string
	MD5         string
	DestFolder  string  // 推定先フォルダ（空なら未推定）
	Score       float64 // 統合スコア（MinHashスコア + メタデータスコア）
	MatchMethod string  // 最もスコアに寄与した手段: "minhash" / "ir" / "title" / ""
}

type EstimateDiffInstallUseCase struct {
	elsaRepo        *persistence.ElsaRepository
	songRepo        model.SongRepository
	metaRepo        model.MetaRepository
	irClient        port.IRClient
	estimateUseCase *EstimateInstallLocationUseCase
}

func NewEstimateDiffInstallUseCase(
	elsaRepo *persistence.ElsaRepository,
	songRepo model.SongRepository,
	metaRepo model.MetaRepository,
	irClient port.IRClient,
	estimateUseCase *EstimateInstallLocationUseCase,
) *EstimateDiffInstallUseCase {
	return &EstimateDiffInstallUseCase{
		elsaRepo:        elsaRepo,
		songRepo:        songRepo,
		metaRepo:        metaRepo,
		irClient:        irClient,
		estimateUseCase: estimateUseCase,
	}
}

// folderScore はフォルダ単位のスコア集約用
type folderScore struct {
	FolderPath     string
	MinHashScore   float64
	MetadataScore  float64
	BestMethod     string // 最もスコアに寄与した手段
}

func (fs folderScore) Total() float64 {
	return fs.MinHashScore + fs.MetadataScore
}

// EstimateOne は1ファイルの導入先を統一スコア方式で推定する
func (u *EstimateDiffInstallUseCase) EstimateOne(ctx context.Context, filePath string) (*ImportCandidate, error) {
	parsed, err := bms.ParseBMSFile(filePath)
	if err != nil {
		return nil, err
	}

	candidate := &ImportCandidate{
		FilePath:  filePath,
		FileName:  filepath.Base(filePath),
		Title:     parsed.Title,
		Subtitle:  parsed.Subtitle,
		Artist:    parsed.Artist,
		Subartist: parsed.Subartist,
		Genre:     parsed.Genre,
		MD5:       parsed.MD5,
	}

	// フォルダごとのスコア集約map
	scores := make(map[string]*folderScore)

	// Step 1: WAV MinHash類似度検索
	sig := bms.ComputeMinHash(parsed.WAVFiles)
	match, err := u.elsaRepo.FindMostSimilarByMinHash(ctx, sig.Bytes(), 0.0)
	if err != nil {
		return nil, err
	}
	if match != nil && match.Similarity > 0 {
		mhScore := match.Similarity * minhashScoreMultiplier
		scores[match.FolderPath] = &folderScore{
			FolderPath:   match.FolderPath,
			MinHashScore: mhScore,
			BestMethod:   "minhash",
		}
	}

	// Step 2: MinHashスコアが閾値未満ならIR問い合わせ
	bestMinHash := 0.0
	for _, fs := range scores {
		if fs.MinHashScore > bestMinHash {
			bestMinHash = fs.MinHashScore
		}
	}

	title := parsed.Title
	artist := parsed.Artist

	if bestMinHash < irSkipThreshold {
		irResp, err := u.irClient.LookupByMD5(ctx, parsed.MD5)
		if err == nil && irResp != nil && irResp.Registered {
			u.saveIRResponse(ctx, parsed.MD5, irResp)
			// IR情報でEstimateInstallLocationを実行
			title = irResp.Title
			artist = irResp.Artist
		}
	}

	// Step 3: EstimateInstallLocation（IR情報またはパースしたtitle/artist）
	if title != "" {
		metaCandidates, err := u.estimateUseCase.Execute(ctx, title, artist, parsed.MD5)
		if err == nil {
			for _, mc := range metaCandidates {
				fs, ok := scores[mc.FolderPath]
				if ok {
					fs.MetadataScore = float64(mc.Score)
					// MinHash + メタデータ両方あればbestMethodはスコアが高い方
					if fs.MetadataScore > fs.MinHashScore {
						fs.BestMethod = bestMethodFromMatchTypes(mc.MatchTypes)
					}
				} else {
					scores[mc.FolderPath] = &folderScore{
						FolderPath:    mc.FolderPath,
						MetadataScore: float64(mc.Score),
						BestMethod:    bestMethodFromMatchTypes(mc.MatchTypes),
					}
				}
			}
		}
	}

	// Step 4: 統合スコア最上位を選択
	if len(scores) == 0 {
		return candidate, nil
	}

	var all []*folderScore
	for _, fs := range scores {
		all = append(all, fs)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].Total() > all[j].Total()
	})

	best := all[0]
	candidate.DestFolder = best.FolderPath
	candidate.Score = best.Total()
	candidate.MatchMethod = best.BestMethod
	return candidate, nil
}

func bestMethodFromMatchTypes(matchTypes []string) string {
	for _, mt := range matchTypes {
		if mt == "body_url" {
			return "ir"
		}
	}
	return "title"
}

func (u *EstimateDiffInstallUseCase) saveIRResponse(ctx context.Context, md5 string, resp *port.IRResponse) {
	now := time.Now()
	meta := model.ChartIRMeta{
		MD5:          md5,
		Tags:         resp.Tags,
		LR2IRBodyURL: resp.BodyURL,
		LR2IRDiffURL: resp.DiffURL,
		LR2IRNotes:   resp.Notes,
		FetchedAt:    &now,
	}
	u.metaRepo.UpsertChartMeta(ctx, meta)
}
```

**Step 2: ExecuteDiffImportUseCase 実装**

`internal/usecase/execute_diff_import.go`:

```go
package usecase

import "github.com/meta-BE/bms-elsa/internal/domain/fileutil"

// ImportRequest はファイル移動リクエスト
type ImportRequest struct {
	FilePath   string
	DestFolder string
}

// ImportResult はファイル移動結果
type ImportResult struct {
	Success int
	Failed  int
	Errors  []string
}

type ExecuteDiffImportUseCase struct{}

func NewExecuteDiffImportUseCase() *ExecuteDiffImportUseCase {
	return &ExecuteDiffImportUseCase{}
}

// Execute は確定済み候補のファイル移動を実行する
func (u *ExecuteDiffImportUseCase) Execute(requests []ImportRequest) ImportResult {
	var result ImportResult
	for _, req := range requests {
		if err := fileutil.MoveFileToFolder(req.FilePath, req.DestFolder); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, err.Error())
		} else {
			result.Success++
		}
	}
	return result
}
```

**Step 3: テスト実行**

```bash
go build ./...
go test ./internal/usecase/... -v
```

**Step 4: コミット**

```bash
git add internal/usecase/estimate_diff_install.go internal/usecase/execute_diff_import.go
git commit -m "feat: 差分導入の推定・実行ユースケースを追加"
```

---

### Task 6: DiffImportHandler（Wailsバインディング）

**Files:**
- Create: `internal/app/diff_import_handler.go`
- Modify: `app.go` — DI組み立て・Bind追加
- Modify: `main.go` — Bind追加

**参考:**
- `internal/app/scan_handler.go` — バックグラウンド処理・進捗イベントのパターン
- `app.go` — DI組み立てパターン

**Step 1: DiffImportHandler 実装**

`internal/app/diff_import_handler.go`:

```go
package app

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/meta-BE/bms-elsa/internal/usecase"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// DiffImportCandidateDTO はフロントエンドに返す推定結果
type DiffImportCandidateDTO struct {
	FilePath    string  `json:"filePath"`
	FileName    string  `json:"fileName"`
	Title       string  `json:"title"`
	Subtitle    string  `json:"subtitle"`
	Artist      string  `json:"artist"`
	Subartist   string  `json:"subartist"`
	DestFolder  string  `json:"destFolder"`
	Score       float64 `json:"score"`
	MatchMethod string  `json:"matchMethod"`
}

// DiffImportResultDTO はフロントエンドに返す導入結果
type DiffImportResultDTO struct {
	Success int      `json:"success"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors"`
}

type DiffImportHandler struct {
	ctx            context.Context
	estimateUC     *usecase.EstimateDiffInstallUseCase
	executeUC      *usecase.ExecuteDiffImportUseCase

	mu         sync.Mutex
	running    bool
	cancelFunc context.CancelFunc
}

func NewDiffImportHandler(
	estimateUC *usecase.EstimateDiffInstallUseCase,
	executeUC *usecase.ExecuteDiffImportUseCase,
) *DiffImportHandler {
	return &DiffImportHandler{
		estimateUC: estimateUC,
		executeUC:  executeUC,
	}
}

func (h *DiffImportHandler) SetContext(ctx context.Context) { h.ctx = ctx }

// isBMSFile はBMS/BME/BMLファイルかどうかを判定する
func isBMSFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".bms" || ext == ".bme" || ext == ".bml"
}

// collectBMSFiles はパスリストからBMSファイルを収集する（フォルダの場合は再帰的に）
func collectBMSFiles(paths []string) []string {
	var result []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if !d.IsDir() && isBMSFile(path) {
					result = append(result, path)
				}
				return nil
			})
		} else if isBMSFile(p) {
			result = append(result, p)
		}
	}
	return result
}

// ParseAndEstimate はD&D時に呼ばれ、パース→推定を一括実行する
func (h *DiffImportHandler) ParseAndEstimate(filePaths []string) ([]DiffImportCandidateDTO, error) {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return nil, nil
	}
	h.running = true
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		h.running = false
		h.cancelFunc = nil
		h.mu.Unlock()
	}()

	ctx, cancel := context.WithCancel(h.ctx)
	h.mu.Lock()
	h.cancelFunc = cancel
	h.mu.Unlock()

	bmsFiles := collectBMSFiles(filePaths)
	total := len(bmsFiles)
	var results []DiffImportCandidateDTO

	for i, fp := range bmsFiles {
		select {
		case <-ctx.Done():
			return results, nil
		default:
		}

		candidate, err := h.estimateUC.EstimateOne(ctx, fp)
		if err != nil {
			// パースエラーはスキップして続行
			wailsRuntime.EventsEmit(h.ctx, "diff-import:progress", map[string]int{
				"current": i + 1, "total": total,
			})
			continue
		}

		results = append(results, DiffImportCandidateDTO{
			FilePath:    candidate.FilePath,
			FileName:    candidate.FileName,
			Title:       candidate.Title,
			Subtitle:    candidate.Subtitle,
			Artist:      candidate.Artist,
			Subartist:   candidate.Subartist,
			DestFolder:  candidate.DestFolder,
			Score:       candidate.Score,
			MatchMethod: candidate.MatchMethod,
		})

		wailsRuntime.EventsEmit(h.ctx, "diff-import:progress", map[string]int{
			"current": i + 1, "total": total,
		})
	}

	wailsRuntime.EventsEmit(h.ctx, "diff-import:done", nil)
	return results, nil
}

// ExecuteImport は確定済み候補のファイル移動を実行する
func (h *DiffImportHandler) ExecuteImport(candidates []DiffImportCandidateDTO) DiffImportResultDTO {
	var requests []usecase.ImportRequest
	for _, c := range candidates {
		if c.DestFolder != "" {
			requests = append(requests, usecase.ImportRequest{
				FilePath:   c.FilePath,
				DestFolder: c.DestFolder,
			})
		}
	}
	result := h.executeUC.Execute(requests)
	return DiffImportResultDTO{
		Success: result.Success,
		Failed:  result.Failed,
		Errors:  result.Errors,
	}
}

// StopEstimate は実行中の推定処理を中断する
func (h *DiffImportHandler) StopEstimate() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancelFunc != nil {
		h.cancelFunc()
	}
}

// IsEstimateRunning は推定処理が実行中かどうかを返す
func (h *DiffImportHandler) IsEstimateRunning() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running
}
```

**Step 2: app.go にDI組み立てを追加**

`app.go` の `App` 構造体に `DiffImportHandler` フィールドを追加し、`Init()` 内でDI組み立て:

```go
// App構造体に追加
DiffImportHandler *internalapp.DiffImportHandler

// Init() 内のDI組み立てセクション末尾に追加
estimateDiffInstall := usecase.NewEstimateDiffInstallUseCase(elsaRepo, songdataReader, elsaRepo, irClient, estimateInstallLocation)
executeDiffImport := usecase.NewExecuteDiffImportUseCase()
a.DiffImportHandler = internalapp.NewDiffImportHandler(estimateDiffInstall, executeDiffImport)

// startup() に追加
a.DiffImportHandler.SetContext(ctx)
```

**Step 3: main.go のBind配列に追加**

```go
Bind: []interface{}{
	app,
	app.SongHandler,
	app.IRHandler,
	app.InferenceHandler,
	app.RewriteHandler,
	app.ChartHandler,
	app.DifficultyTableHandler,
	app.ScanHandler,
	app.DiffImportHandler,  // 追加
},
```

**Step 4: ビルド確認**

```bash
go build ./...
```

**Step 5: コミット**

```bash
git add internal/app/diff_import_handler.go app.go main.go
git commit -m "feat: DiffImportHandlerを追加しWailsバインディングに登録"
```

---

### Task 7: フロントエンド — 差分導入タブ

**Files:**
- Create: `frontend/src/views/DiffImportView.svelte`
- Modify: `frontend/src/App.svelte` — タブ追加

**参考:**
- `frontend/src/App.svelte` — 既存のタブ構成パターン
- `frontend/src/views/DuplicateView.svelte` — テーブル表示パターン
- `frontend/src/views/ChartListView.svelte` — TanStack Table + Virtual使用パターン

**Step 1: DiffImportView.svelte を作成**

`frontend/src/views/DiffImportView.svelte`:

- テーブル全体がD&Dドロップゾーン
- 件数0のとき「BMS/BME/BMLファイルをドラッグ＆ドロップして差分を追加」プレースホルダー表示
- テーブルカラム: ファイル名, TITLE（+SUBTITLE）, ARTIST（+SUBARTIST）, 推定先, スコア, 推定方法, 操作（クリアボタン）
- 下部に「推定先に導入」ボタン
- Wailsイベント `diff-import:progress` と `diff-import:done` をリスン
- D&D時に `DiffImportHandler.ParseAndEstimate()` を呼び出し
- 「推定先に導入」ボタンで `DiffImportHandler.ExecuteImport()` を呼び出し
- 「クリア」ボタンで該当行の `destFolder` を空にする
- Wails v2のD&D API: `window.runtime.EventsOn('wails:file-drop', ...)` またはHTML5のdragover/drop APIを使用

**Step 2: App.svelte にタブを追加**

`frontend/src/App.svelte`:

1. `activeTab` の型に `'diff-import'` を追加
2. タブバーに「差分導入」ボタンを追加
3. タブコンテンツに `DiffImportView` を追加（SplitPaneなし、テーブルのみ）

**Step 3: 動作確認**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
wails dev
```

手動で動作確認:
- 差分導入タブが表示される
- プレースホルダーテキストが表示される
- BMSファイルをD&Dすると推定処理が実行される
- テーブルに結果が表示される
- クリアボタンで推定先が除外される
- 「推定先に導入」ボタンでファイル移動が実行される

**Step 4: コミット**

```bash
git add frontend/src/views/DiffImportView.svelte frontend/src/App.svelte
git commit -m "feat: 差分導入タブのフロントエンドUIを追加"
```

---

### Task 8: マニュアル更新

**Files:**
- Modify: `docs/manual.md`

**Step 1: 差分導入の説明を追加**

`docs/manual.md` の「画面説明」セクションに「差分導入」サブセクションを追加。
「機能説明」セクションに「差分導入」の操作説明を追加。

**Step 2: コミット**

```bash
git add docs/manual.md
git commit -m "docs: マニュアルに差分導入画面の説明を追加"
```

---

### Task 9: 全体ビルド確認

**Step 1: Goテスト**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
go test ./... -v
```

**Step 2: ビルド**

```bash
go build ./...
```

期待: 全テスト PASS、ビルド成功
