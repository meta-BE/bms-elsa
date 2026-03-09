# バックエンドリファクタリング実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** IR統合・型移動・ScanHandler/ScanDuplicatesのusecase化を依存順に実施し、アーキテクチャの一貫性を改善する

**Architecture:** 依存順序 Task 1→2→3→4。Task 1（IR統合）は独立。Task 2（型移動）が Task 3, 4 の前提。各タスクは個別コミット。

**Tech Stack:** Go 1.24, Wails v2

---

## 設計判断

### SongGroup を model に移動する（similarity.SongInfo に統一しない）

`persistence.SongGroup` と `similarity.SongInfo` はフィールドが完全一致しており、統一する選択肢もあった。しかし以下の理由で `model.SongGroup` として独立させる:

- `SongRepository` インターフェース（model パッケージ）の戻り値型に `similarity.SongInfo`（別ドメインパッケージ）を使うと、model → similarity の依存が生まれる。model はプロジェクト全体の基盤なのでインポートグラフは最小に保つべき
- `SongGroup` はDBクエリ結果の表現であり、`SongInfo` は類似度計算の入力表現。概念的にも別物
- 変換は usecase 内の10行程度で、重複の実害は小さい

### ScanDuplicates の戻り値型は similarity.DuplicateGroup のまま

`app.go` の `ScanDuplicates()` メソッドは `[]similarity.DuplicateGroup` を返す。usecase 化しても戻り値型は変わらないため、`app.go` の `similarity` import は残る。これを解消するには DTO 変換層が必要だが、以下の理由で現状を許容する:

- `similarity.DuplicateGroup` は純粋なドメイン型であり、外部依存を持たない
- DTO に変換しても Wails バインドが JSON シリアライズするだけなので、型を増やすメリットがない
- `app.go` が `similarity` を import すること自体は、usecase 経由で処理する限り大きな問題ではない

### ScanMinHashUseCase.Execute はターゲット一覧を引数で受け取る

`ListChartsWithoutMinhash` を usecase 内部で呼ぶ設計もあったが、引数で受け取る方式を採用する:

- `BulkFetchIRUseCase.Execute` が `md5s []string` を引数で受け取るパターンと統一される
- ハンドラー側でターゲット取得とgoroutine管理を担い、usecase はループ処理に集中する責務分離
- ターゲット取得時のエラーでgoroutineを起動しない（`running` フラグのリセットがシンプル）

### IR統合は関数型引数で差し替え（インターフェースにしない）

`startBulkFetchWith` の差し替え部分を `func(ctx context.Context) ([]string, error)` で受け取る。Strategy パターン（インターフェース）にしない理由:

- 差異が1行（メソッド呼び出し1つ）だけなので、インターフェース定義は過剰
- クロージャで `tableID` をキャプチャできるため、ラッパー側のコードが最小になる
- 将来スコープが増えても、公開メソッドを1つ追加するだけで済む

---

### Task 1: IR一括取得メソッドの統合

`StartBulkFetch`（62-118行）と `StartDifficultyTableBulkFetch`（121-176行）の差異は md5 取得クエリの1行のみ。残り56行が完全コピペ。

**Files:**
- Modify: `internal/app/ir_handler.go`

**変更内容:**

プライベートメソッド `startBulkFetchWith` を抽出し、公開メソッドを薄いラッパーにする。

```go
// startBulkFetchWith はIR一括取得の共通処理。fetchMD5s で取得対象を差し替える。
func (h *IRHandler) startBulkFetchWith(fetchMD5s func(ctx context.Context) ([]string, error)) error {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return nil
	}
	h.running = true
	h.mu.Unlock()

	md5s, err := fetchMD5s(h.ctx)
	if err != nil {
		h.mu.Lock()
		h.running = false
		h.mu.Unlock()
		return err
	}

	ctx, cancel := context.WithCancel(h.ctx)
	h.mu.Lock()
	h.cancelFunc = cancel
	h.mu.Unlock()

	go func() {
		defer func() {
			h.mu.Lock()
			h.running = false
			h.cancelFunc = nil
			h.mu.Unlock()
		}()

		result, err := h.bulkFetchIR.Execute(ctx, md5s, func(p usecase.BulkFetchProgress) {
			wailsRuntime.EventsEmit(h.ctx, "ir:progress", map[string]int{
				"current": p.Current,
				"total":   p.Total,
			})
		})

		doneData := map[string]interface{}{
			"cancelled": false,
			"error":     "",
		}
		if err != nil {
			doneData["error"] = err.Error()
		}
		if result != nil {
			doneData["total"] = result.Total
			doneData["fetched"] = result.Fetched
			doneData["notFound"] = result.NotFound
			doneData["failed"] = result.Failed
			doneData["cancelled"] = result.Cancelled
		}
		wailsRuntime.EventsEmit(h.ctx, "ir:done", doneData)
	}()

	return nil
}

func (h *IRHandler) StartBulkFetch() error {
	return h.startBulkFetchWith(h.metaRepo.ListUnfetchedChartMD5s)
}

func (h *IRHandler) StartDifficultyTableBulkFetch(tableID int) error {
	return h.startBulkFetchWith(func(ctx context.Context) ([]string, error) {
		return h.metaRepo.ListUnfetchedDTEntryMD5s(ctx, tableID)
	})
}
```

**確認:**

```bash
cd /path/to/bms-elsa && go build ./...
```

**コミット:**

```bash
git add internal/app/ir_handler.go
git commit -m "refactor: IR一括取得の共通処理をstartBulkFetchWithに抽出"
```

---

### Task 2: persistence層の独自型を domain/model に移動

`ChartScanTarget`（Task 3 の前提）と `SongGroup`（Task 4 の前提）を移動する。

**Files:**
- Modify: `internal/domain/model/repository.go`
- Modify: `internal/adapter/persistence/elsa_repository.go`
- Modify: `internal/adapter/persistence/songdata_reader.go`
- Modify: `internal/app/scan_handler.go`
- Modify: `app.go`
- Modify: `internal/usecase/usecase_test.go`

**Step 1: model にスキャン用型を追加し、MetaRepository / SongRepository にメソッドを追加**

`internal/domain/model/repository.go` に以下を追加:

```go
// ChartScanTarget はMinHashスキャン対象の譜面情報
type ChartScanTarget struct {
	MD5  string
	Path string
}

// SongGroup は重複スキャン用のfolder単位の楽曲情報
type SongGroup struct {
	FolderHash string
	Title      string
	Artist     string
	Genre      string
	MinBPM     float64
	MaxBPM     float64
	ChartCount int
	Path       string // 代表パス（フォルダまで）
}
```

`MetaRepository` インターフェースに追加:

```go
	// MinHashスキャン対象の譜面リスト
	ListChartsWithoutMinhash(ctx context.Context) ([]ChartScanTarget, error)
	// wav_minhashを更新（レコードがなければINSERT）
	UpdateWavMinhash(ctx context.Context, md5 string, minhash []byte) error
```

`SongRepository` インターフェースに追加:

```go
	// folder単位で楽曲グループを返す（重複スキャン用）
	ListSongGroupsForDuplicateScan(ctx context.Context) ([]SongGroup, error)
```

**Step 2: persistence層の型定義を削除し、model の型を使うように変更**

`internal/adapter/persistence/elsa_repository.go`:
- `ChartScanTarget` 型定義（333-337行）を削除
- `ListChartsWithoutMinhash` の戻り値を `[]model.ChartScanTarget` に変更
- `import` に `model` を追加（既にあれば不要）

`internal/adapter/persistence/songdata_reader.go`:
- `SongGroup` 型定義（468-477行）を削除
- `ListSongGroupsForDuplicateScan` の戻り値を `[]model.SongGroup` に変更

**Step 3: ハンドラー・app.go のインポートを更新**

`internal/app/scan_handler.go`:
- `persistence` パッケージの import を削除（Task 3 でさらに変更するが、この時点ではまだ `elsaRepo *persistence.ElsaRepository` が残る）
- `tgt` の型が `model.ChartScanTarget` になるが、フィールドアクセスは同じなので影響なし

`app.go`:
- `a.songReader.ListSongGroupsForDuplicateScan()` の戻り値が `[]model.SongGroup` になる
- フィールドアクセスは同じなので `ScanDuplicates` メソッド内の変更なし

**Step 4: テストのモックにメソッドを追加**

`internal/usecase/usecase_test.go` の `mockMetaRepo` に追加:

```go
func (m *mockMetaRepo) ListChartsWithoutMinhash(_ context.Context) ([]model.ChartScanTarget, error) {
	return nil, nil
}

func (m *mockMetaRepo) UpdateWavMinhash(_ context.Context, _ string, _ []byte) error {
	return nil
}
```

`mockSongRepo` に追加:

```go
func (m *mockSongRepo) ListSongGroupsForDuplicateScan(_ context.Context) ([]model.SongGroup, error) {
	return nil, nil
}
```

**確認:**

```bash
cd /path/to/bms-elsa && go build ./... && go test ./...
```

**コミット:**

```bash
git add internal/domain/model/repository.go internal/adapter/persistence/elsa_repository.go internal/adapter/persistence/songdata_reader.go internal/app/scan_handler.go app.go internal/usecase/usecase_test.go
git commit -m "refactor: ChartScanTarget・SongGroupをdomain/modelに移動"
```

---

### Task 3: ScanHandler の MinHash スキャンロジックを usecase 層に抽出

`BulkFetchIRUseCase` の「進捗コールバック付きループ」パターンを踏襲する。

**Files:**
- Create: `internal/usecase/scan_minhash.go`
- Modify: `internal/app/scan_handler.go`
- Modify: `app.go`

**Step 1: usecase を作成**

`internal/usecase/scan_minhash.go`:

```go
package usecase

import (
	"context"
	"fmt"
	"os"

	"github.com/meta-BE/bms-elsa/internal/domain/bms"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

// ScanMinHashProgress はスキャン進捗
type ScanMinHashProgress struct {
	Current int
	Total   int
}

// ScanMinHashResult はスキャン結果
type ScanMinHashResult struct {
	Total     int
	Computed  int
	Skipped   int
	Failed    int
	Cancelled bool
}

type ScanMinHashUseCase struct {
	metaRepo model.MetaRepository
}

func NewScanMinHashUseCase(metaRepo model.MetaRepository) *ScanMinHashUseCase {
	return &ScanMinHashUseCase{metaRepo: metaRepo}
}

func (u *ScanMinHashUseCase) Execute(ctx context.Context, targets []model.ChartScanTarget, progressFn func(ScanMinHashProgress)) *ScanMinHashResult {
	result := &ScanMinHashResult{Total: len(targets)}

	for i, tgt := range targets {
		select {
		case <-ctx.Done():
			result.Cancelled = true
			return result
		default:
		}

		if _, err := os.Stat(tgt.Path); err != nil {
			result.Skipped++
			if progressFn != nil {
				progressFn(ScanMinHashProgress{Current: i + 1, Total: len(targets)})
			}
			continue
		}

		parsed, err := bms.ParseBMSFile(tgt.Path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scan: parse error %s: %v\n", tgt.Path, err)
			result.Failed++
			if progressFn != nil {
				progressFn(ScanMinHashProgress{Current: i + 1, Total: len(targets)})
			}
			continue
		}

		sig := bms.ComputeMinHash(parsed.WAVFiles)
		if err := u.metaRepo.UpdateWavMinhash(ctx, tgt.MD5, sig.Bytes()); err != nil {
			fmt.Fprintf(os.Stderr, "scan: db error %s: %v\n", tgt.MD5, err)
			result.Failed++
			if progressFn != nil {
				progressFn(ScanMinHashProgress{Current: i + 1, Total: len(targets)})
			}
			continue
		}

		result.Computed++
		if progressFn != nil {
			progressFn(ScanMinHashProgress{Current: i + 1, Total: len(targets)})
		}
	}

	return result
}
```

**Step 2: ScanHandler を薄いラッパーに変更**

`internal/app/scan_handler.go` を以下に書き換え:

```go
package app

import (
	"context"
	"sync"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ScanHandler struct {
	ctx          context.Context
	metaRepo     model.MetaRepository
	scanMinHash  *usecase.ScanMinHashUseCase

	mu         sync.Mutex
	running    bool
	cancelFunc context.CancelFunc
}

func NewScanHandler(metaRepo model.MetaRepository, scanMinHash *usecase.ScanMinHashUseCase) *ScanHandler {
	return &ScanHandler{metaRepo: metaRepo, scanMinHash: scanMinHash}
}

func (h *ScanHandler) SetContext(ctx context.Context) { h.ctx = ctx }

func (h *ScanHandler) StartMinHashScan() error {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return nil
	}
	h.running = true
	h.mu.Unlock()

	targets, err := h.metaRepo.ListChartsWithoutMinhash(h.ctx)
	if err != nil {
		h.mu.Lock()
		h.running = false
		h.mu.Unlock()
		return err
	}

	ctx, cancel := context.WithCancel(h.ctx)
	h.mu.Lock()
	h.cancelFunc = cancel
	h.mu.Unlock()

	go func() {
		defer func() {
			h.mu.Lock()
			h.running = false
			h.cancelFunc = nil
			h.mu.Unlock()
		}()

		result := h.scanMinHash.Execute(ctx, targets, func(p usecase.ScanMinHashProgress) {
			wailsRuntime.EventsEmit(h.ctx, "scan:progress", map[string]int{
				"current": p.Current,
				"total":   p.Total,
			})
		})

		wailsRuntime.EventsEmit(h.ctx, "scan:done", map[string]interface{}{
			"total":     result.Total,
			"computed":  result.Computed,
			"skipped":   result.Skipped,
			"failed":    result.Failed,
			"cancelled": result.Cancelled,
		})
	}()

	return nil
}

func (h *ScanHandler) StopMinHashScan() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancelFunc != nil {
		h.cancelFunc()
	}
}

func (h *ScanHandler) IsMinHashScanRunning() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running
}
```

**Step 3: app.go の DI を更新**

`app.go` の `Init()` 内:

```go
// 変更前
a.ScanHandler = internalapp.NewScanHandler(elsaRepo)

// 変更後
scanMinHash := usecase.NewScanMinHashUseCase(elsaRepo)
a.ScanHandler = internalapp.NewScanHandler(elsaRepo, scanMinHash)
```

`app.go` の import から `"github.com/meta-BE/bms-elsa/internal/domain/bms"` を削除（もし残っていれば）。

**確認:**

```bash
cd /path/to/bms-elsa && go build ./... && go test ./...
```

**コミット:**

```bash
git add internal/usecase/scan_minhash.go internal/app/scan_handler.go app.go
git commit -m "refactor: ScanHandlerのMinHashロジックをScanMinHashUseCaseに抽出"
```

---

### Task 4: ScanDuplicates を usecase 化

`app.go` から `similarity` パッケージ直接参照と `SongGroup→SongInfo` 変換ロジックを解消する。

**Files:**
- Create: `internal/usecase/scan_duplicates.go`
- Modify: `app.go`

**Step 1: usecase を作成**

`internal/usecase/scan_duplicates.go`:

```go
package usecase

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/domain/similarity"
)

type ScanDuplicatesUseCase struct {
	songRepo model.SongRepository
}

func NewScanDuplicatesUseCase(songRepo model.SongRepository) *ScanDuplicatesUseCase {
	return &ScanDuplicatesUseCase{songRepo: songRepo}
}

func (u *ScanDuplicatesUseCase) Execute(ctx context.Context) ([]similarity.DuplicateGroup, error) {
	groups, err := u.songRepo.ListSongGroupsForDuplicateScan(ctx)
	if err != nil {
		return nil, err
	}

	songs := make([]similarity.SongInfo, len(groups))
	for i, g := range groups {
		songs[i] = similarity.SongInfo{
			FolderHash: g.FolderHash,
			Title:      g.Title,
			Artist:     g.Artist,
			Genre:      g.Genre,
			MinBPM:     g.MinBPM,
			MaxBPM:     g.MaxBPM,
			ChartCount: g.ChartCount,
			Path:       g.Path,
		}
	}

	return similarity.FindDuplicateGroups(songs, 60), nil
}
```

**Step 2: app.go の ScanDuplicates をusecase委譲に変更**

```go
// 変更前
func (a *App) ScanDuplicates() ([]similarity.DuplicateGroup, error) {
	groups, err := a.songReader.ListSongGroupsForDuplicateScan(a.ctx)
	// ... 型変換 + FindDuplicateGroups ...
}

// 変更後
func (a *App) ScanDuplicates() ([]similarity.DuplicateGroup, error) {
	return a.scanDuplicates.Execute(a.ctx)
}
```

`App` 構造体に `scanDuplicates *usecase.ScanDuplicatesUseCase` フィールドを追加。

`Init()` に追加:

```go
a.scanDuplicates = usecase.NewScanDuplicatesUseCase(songdataReader)
```

`app.go` の import から不要になるものを削除:
- `"github.com/meta-BE/bms-elsa/internal/domain/similarity"` — `ScanDuplicates` の戻り値型で参照が残るため**削除不可**
- `a.songReader` フィールド — `ScanDuplicates` 以外で使っていなければ削除可能。ただし他で使用している場合は残す

**確認:**

```bash
cd /path/to/bms-elsa && go build ./... && go test ./...
```

**コミット:**

```bash
git add internal/usecase/scan_duplicates.go app.go
git commit -m "refactor: ScanDuplicatesをusecaseに抽出しapp.goのsimilarity直接参照を解消"
```
