# IR一括取得機能 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 未取得譜面のLR2IR情報をバックグラウンドで逐次取得し、進捗をリアルタイム表示する機能を追加する

**Architecture:** 新ユースケース`BulkFetchIRUseCase`が`IRClient`を直接呼び出して未取得譜面を順次取得。`IRHandler`にgoroutine起動/停止メソッドを追加し、`runtime.EventsEmit`で進捗をフロントに通知。SongTableヘッダーにインライン進捗UIを追加。

**Tech Stack:** Go 1.24 / SQLite (modernc.org/sqlite) / Svelte 4 / Wails v2 / DaisyUI

---

## 背景知識

### 既存コード構成
- `internal/port/ir_client.go` — `IRClient`インターフェース（`LookupByMD5(ctx, md5) → IRResponse`）
- `internal/adapter/gateway/lr2ir_client.go` — HTTP実装（1秒/リクエストのレートリミット内蔵）
- `internal/usecase/lookup_ir.go` — `LookupIRUseCase`（1MD5単位でIR取得→DB保存）
- `internal/domain/model/repository.go` — `MetaRepository`インターフェース
- `internal/adapter/persistence/elsa_repository.go` — SQLite実装（`UpsertChartMeta`等）
- `internal/app/ir_handler.go` — Wailsバインディング層
- `app.go` — DI組み立て・Wails起動
- `frontend/src/SongTable.svelte` — 楽曲一覧テーブル

### chart_metaテーブル
```sql
CREATE TABLE chart_meta (
    md5, sha256, lr2ir_tags, lr2ir_body_url, lr2ir_diff_url,
    lr2ir_notes, lr2ir_fetched_at, working_body_url, working_diff_url,
    UNIQUE(md5, sha256)
)
```
`lr2ir_fetched_at IS NULL` = 未取得。

### songdata.db
beatorajaのDB。`sd`スキーマとしてATTACH済み。`sd.song`テーブルに`md5`, `sha256`カラムあり。

### Wailsイベント
- Go側: `wailsRuntime.EventsEmit(ctx, "eventName", data)`
- JS側: `import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime'`
- `EventsOn`は解除関数を返す

---

### Task 1: LookupIRUseCaseの修正 — 未登録でもfetched_atを保存

**Files:**
- Modify: `internal/usecase/lookup_ir.go:21-44`

**現状:** `resp.Registered == false`の場合、DBに保存せずreturnしている（行26-29）。
**変更:** 未登録でも`ChartIRMeta{FetchedAt: &now}`でupsertする。

**Step 1: テスト作成**

Create: `internal/usecase/lookup_ir_test.go`

```go
package usecase_test

import (
	"context"
	"testing"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/port"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

type mockIRClient struct {
	lookupFunc func(ctx context.Context, md5 string) (*port.IRResponse, error)
}

func (m *mockIRClient) LookupByMD5(ctx context.Context, md5 string) (*port.IRResponse, error) {
	return m.lookupFunc(ctx, md5)
}

type mockMetaRepoForIR struct {
	model.MetaRepository
	upsertChartMetaCalls []model.ChartIRMeta
}

func (m *mockMetaRepoForIR) UpsertChartMeta(_ context.Context, meta model.ChartIRMeta) error {
	m.upsertChartMetaCalls = append(m.upsertChartMetaCalls, meta)
	return nil
}

func TestLookupIR_Registered(t *testing.T) {
	client := &mockIRClient{
		lookupFunc: func(_ context.Context, _ string) (*port.IRResponse, error) {
			return &port.IRResponse{
				Registered: true,
				Tags:       []string{"tag1"},
				BodyURL:    "http://example.com/body",
				DiffURL:    "http://example.com/diff",
				Notes:      "note",
			}, nil
		},
	}
	repo := &mockMetaRepoForIR{}
	uc := usecase.NewLookupIRUseCase(client, repo)

	resp, err := uc.Execute(context.Background(), "md5test", "sha256test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Registered {
		t.Fatal("expected Registered=true")
	}
	if len(repo.upsertChartMetaCalls) != 1 {
		t.Fatalf("UpsertChartMeta calls = %d, want 1", len(repo.upsertChartMetaCalls))
	}
	meta := repo.upsertChartMetaCalls[0]
	if meta.MD5 != "md5test" || meta.SHA256 != "sha256test" {
		t.Errorf("wrong md5/sha256: %q/%q", meta.MD5, meta.SHA256)
	}
	if meta.FetchedAt == nil {
		t.Error("FetchedAt should be set")
	}
	if meta.LR2IRBodyURL != "http://example.com/body" {
		t.Errorf("BodyURL = %q", meta.LR2IRBodyURL)
	}
}

func TestLookupIR_NotRegistered_StillSavesFetchedAt(t *testing.T) {
	client := &mockIRClient{
		lookupFunc: func(_ context.Context, _ string) (*port.IRResponse, error) {
			return &port.IRResponse{Registered: false}, nil
		},
	}
	repo := &mockMetaRepoForIR{}
	uc := usecase.NewLookupIRUseCase(client, repo)

	resp, err := uc.Execute(context.Background(), "md5notfound", "sha256notfound")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Registered {
		t.Fatal("expected Registered=false")
	}
	// 未登録でもfetched_atを保存する
	if len(repo.upsertChartMetaCalls) != 1 {
		t.Fatalf("UpsertChartMeta calls = %d, want 1", len(repo.upsertChartMetaCalls))
	}
	meta := repo.upsertChartMetaCalls[0]
	if meta.FetchedAt == nil {
		t.Error("FetchedAt should be set even for unregistered")
	}
	// 未登録なのでURLは空
	if meta.LR2IRBodyURL != "" {
		t.Errorf("BodyURL should be empty, got %q", meta.LR2IRBodyURL)
	}
}
```

**Step 2: テスト実行 → 失敗確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/usecase/ -run TestLookupIR -v`
Expected: `TestLookupIR_NotRegistered_StillSavesFetchedAt` がFAIL（UpsertChartMeta calls = 0, want 1）

**Step 3: 実装**

`internal/usecase/lookup_ir.go` のExecuteメソッドを修正:

```go
func (u *LookupIRUseCase) Execute(ctx context.Context, md5, sha256 string) (*port.IRResponse, error) {
	resp, err := u.irClient.LookupByMD5(ctx, md5)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	meta := model.ChartIRMeta{
		MD5:       md5,
		SHA256:    sha256,
		FetchedAt: &now,
	}
	if resp.Registered {
		meta.Tags = resp.Tags
		meta.LR2IRBodyURL = resp.BodyURL
		meta.LR2IRDiffURL = resp.DiffURL
		meta.LR2IRNotes = resp.Notes
	}
	if err := u.metaRepo.UpsertChartMeta(ctx, meta); err != nil {
		return nil, err
	}
	return resp, nil
}
```

**Step 4: テスト実行 → PASS確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/usecase/ -run TestLookupIR -v`
Expected: PASS

**Step 5: コミット**

```bash
git add internal/usecase/lookup_ir.go internal/usecase/lookup_ir_test.go
git commit -m "feat: 未登録譜面でもfetched_atを保存するよう修正"
```

---

### Task 2: ListUnfetchedChartKeysリポジトリメソッドの追加

**Files:**
- Modify: `internal/domain/model/song.go` — ChartKey型追加
- Modify: `internal/domain/model/repository.go` — MetaRepositoryにメソッド追加
- Modify: `internal/adapter/persistence/elsa_repository.go` — SQL実装

**Step 1: ドメインモデルにChartKey型追加**

`internal/domain/model/song.go` の末尾に追加:

```go
// ChartKey は譜面の識別キー（IR取得対象リスト用）
type ChartKey struct {
	MD5    string
	SHA256 string
}
```

**Step 2: MetaRepositoryにメソッド追加**

`internal/domain/model/repository.go` のMetaRepositoryインターフェースに追加:

```go
// IR未取得の譜面キー一覧を返す（chart_metaにレコードなし or lr2ir_fetched_at IS NULL）
ListUnfetchedChartKeys(ctx context.Context) ([]ChartKey, error)
```

**Step 3: ビルドエラー確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: FAIL（ElsaRepositoryがMetaRepositoryを満たさない）

**Step 4: SQL実装**

`internal/adapter/persistence/elsa_repository.go` に追加:

```go
func (r *ElsaRepository) ListUnfetchedChartKeys(ctx context.Context) ([]model.ChartKey, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT s.md5, s.sha256
		FROM sd.song s
		LEFT JOIN chart_meta cm ON s.md5 = cm.md5 AND s.sha256 = cm.sha256
		WHERE cm.id IS NULL OR cm.lr2ir_fetched_at IS NULL
		ORDER BY s.md5`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []model.ChartKey
	for rows.Next() {
		var k model.ChartKey
		if err := rows.Scan(&k.MD5, &k.SHA256); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}
```

**Step 5: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: PASS

**Step 6: コミット**

```bash
git add internal/domain/model/song.go internal/domain/model/repository.go internal/adapter/persistence/elsa_repository.go
git commit -m "feat: ListUnfetchedChartKeysリポジトリメソッドを追加"
```

---

### Task 3: BulkFetchIRUseCaseの実装

**Files:**
- Create: `internal/usecase/bulk_fetch_ir.go`
- Create: `internal/usecase/bulk_fetch_ir_test.go`

**Step 1: テスト作成**

Create: `internal/usecase/bulk_fetch_ir_test.go`

```go
package usecase_test

import (
	"context"
	"testing"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/port"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

// mockMetaRepoForBulk はBulkFetchIRのテスト用モック
type mockMetaRepoForBulk struct {
	model.MetaRepository
	unfetchedKeys        []model.ChartKey
	upsertChartMetaCalls []model.ChartIRMeta
}

func (m *mockMetaRepoForBulk) ListUnfetchedChartKeys(_ context.Context) ([]model.ChartKey, error) {
	return m.unfetchedKeys, nil
}

func (m *mockMetaRepoForBulk) UpsertChartMeta(_ context.Context, meta model.ChartIRMeta) error {
	m.upsertChartMetaCalls = append(m.upsertChartMetaCalls, meta)
	return nil
}

func TestBulkFetchIR_AllRegistered(t *testing.T) {
	repo := &mockMetaRepoForBulk{
		unfetchedKeys: []model.ChartKey{
			{MD5: "aaa", SHA256: "sha_aaa"},
			{MD5: "bbb", SHA256: "sha_bbb"},
		},
	}
	client := &mockIRClient{
		lookupFunc: func(_ context.Context, md5 string) (*port.IRResponse, error) {
			return &port.IRResponse{
				Registered: true,
				BodyURL:    "http://example.com/" + md5,
			}, nil
		},
	}

	uc := usecase.NewBulkFetchIRUseCase(client, repo)
	var progresses []usecase.BulkFetchProgress
	result, err := uc.Execute(context.Background(), func(p usecase.BulkFetchProgress) {
		progresses = append(progresses, p)
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("Total = %d, want 2", result.Total)
	}
	if result.Fetched != 2 {
		t.Errorf("Fetched = %d, want 2", result.Fetched)
	}
	if result.NotFound != 0 {
		t.Errorf("NotFound = %d, want 0", result.NotFound)
	}
	if len(progresses) != 2 {
		t.Errorf("progress callbacks = %d, want 2", len(progresses))
	}
	if progresses[1].Current != 2 || progresses[1].Total != 2 {
		t.Errorf("last progress = %d/%d", progresses[1].Current, progresses[1].Total)
	}
}

func TestBulkFetchIR_MixedResults(t *testing.T) {
	repo := &mockMetaRepoForBulk{
		unfetchedKeys: []model.ChartKey{
			{MD5: "found", SHA256: "sha1"},
			{MD5: "notfound", SHA256: "sha2"},
		},
	}
	client := &mockIRClient{
		lookupFunc: func(_ context.Context, md5 string) (*port.IRResponse, error) {
			if md5 == "found" {
				return &port.IRResponse{Registered: true, BodyURL: "http://example.com"}, nil
			}
			return &port.IRResponse{Registered: false}, nil
		},
	}

	uc := usecase.NewBulkFetchIRUseCase(client, repo)
	result, err := uc.Execute(context.Background(), nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fetched != 1 {
		t.Errorf("Fetched = %d, want 1", result.Fetched)
	}
	if result.NotFound != 1 {
		t.Errorf("NotFound = %d, want 1", result.NotFound)
	}
}

func TestBulkFetchIR_Cancellation(t *testing.T) {
	repo := &mockMetaRepoForBulk{
		unfetchedKeys: []model.ChartKey{
			{MD5: "aaa", SHA256: "sha1"},
			{MD5: "bbb", SHA256: "sha2"},
			{MD5: "ccc", SHA256: "sha3"},
		},
	}
	callCount := 0
	client := &mockIRClient{
		lookupFunc: func(_ context.Context, _ string) (*port.IRResponse, error) {
			callCount++
			return &port.IRResponse{Registered: true}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	uc := usecase.NewBulkFetchIRUseCase(client, repo)
	result, err := uc.Execute(ctx, func(p usecase.BulkFetchProgress) {
		if p.Current == 1 {
			cancel() // 1件目完了後にキャンセル
		}
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Cancelled {
		t.Error("expected Cancelled=true")
	}
	// キャンセル前に1件は処理済み
	if result.Fetched < 1 {
		t.Errorf("Fetched = %d, want >= 1", result.Fetched)
	}
}
```

**Step 2: ユースケース実装**

Create: `internal/usecase/bulk_fetch_ir.go`

```go
package usecase

import (
	"context"
	"time"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/port"
)

// BulkFetchProgress は進捗通知用
type BulkFetchProgress struct {
	Current int
	Total   int
}

// BulkFetchResult は一括取得の結果
type BulkFetchResult struct {
	Total     int
	Fetched   int // 登録済み
	NotFound  int // LR2IR未登録
	Failed    int // エラー（スキップ）
	Cancelled bool
}

type BulkFetchIRUseCase struct {
	irClient port.IRClient
	metaRepo model.MetaRepository
}

func NewBulkFetchIRUseCase(irClient port.IRClient, metaRepo model.MetaRepository) *BulkFetchIRUseCase {
	return &BulkFetchIRUseCase{irClient: irClient, metaRepo: metaRepo}
}

func (u *BulkFetchIRUseCase) Execute(ctx context.Context, progressFn func(BulkFetchProgress)) (*BulkFetchResult, error) {
	keys, err := u.metaRepo.ListUnfetchedChartKeys(ctx)
	if err != nil {
		return nil, err
	}

	result := &BulkFetchResult{Total: len(keys)}

	for i, key := range keys {
		// キャンセルチェック
		select {
		case <-ctx.Done():
			result.Cancelled = true
			return result, nil
		default:
		}

		resp, err := u.irClient.LookupByMD5(ctx, key.MD5)
		if err != nil {
			// context cancelの場合
			if ctx.Err() != nil {
				result.Cancelled = true
				return result, nil
			}
			result.Failed++
			if progressFn != nil {
				progressFn(BulkFetchProgress{Current: i + 1, Total: len(keys)})
			}
			continue
		}

		// DB保存（未登録でもfetched_atを記録）
		now := time.Now()
		meta := model.ChartIRMeta{
			MD5:       key.MD5,
			SHA256:    key.SHA256,
			FetchedAt: &now,
		}
		if resp.Registered {
			meta.Tags = resp.Tags
			meta.LR2IRBodyURL = resp.BodyURL
			meta.LR2IRDiffURL = resp.DiffURL
			meta.LR2IRNotes = resp.Notes
			result.Fetched++
		} else {
			result.NotFound++
		}

		if err := u.metaRepo.UpsertChartMeta(ctx, meta); err != nil {
			result.Failed++
		}

		if progressFn != nil {
			progressFn(BulkFetchProgress{Current: i + 1, Total: len(keys)})
		}
	}

	return result, nil
}
```

**Step 3: テスト実行 → PASS確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/usecase/ -run TestBulkFetchIR -v`
Expected: PASS

**Step 4: コミット**

```bash
git add internal/usecase/bulk_fetch_ir.go internal/usecase/bulk_fetch_ir_test.go
git commit -m "feat: BulkFetchIRUseCaseを追加"
```

---

### Task 4: IRHandlerにStartBulkFetch/StopBulkFetchを追加

**Files:**
- Modify: `internal/app/ir_handler.go`
- Modify: `app.go` — DI修正

**Step 1: IRHandler拡張**

`internal/app/ir_handler.go` を以下に全置換:

```go
package app

import (
	"context"
	"strings"
	"sync"

	"github.com/meta-BE/bms-elsa/internal/app/dto"
	"github.com/meta-BE/bms-elsa/internal/usecase"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type IRHandler struct {
	ctx         context.Context
	lookupIR    *usecase.LookupIRUseCase
	bulkFetchIR *usecase.BulkFetchIRUseCase
	updateChart *usecase.UpdateChartMetaUseCase

	mu         sync.Mutex
	running    bool
	cancelFunc context.CancelFunc
}

func NewIRHandler(
	li *usecase.LookupIRUseCase,
	bf *usecase.BulkFetchIRUseCase,
	uc *usecase.UpdateChartMetaUseCase,
) *IRHandler {
	return &IRHandler{lookupIR: li, bulkFetchIR: bf, updateChart: uc}
}

func (h *IRHandler) SetContext(ctx context.Context) { h.ctx = ctx }

func (h *IRHandler) LookupByMD5(md5, sha256 string) (*dto.ChartDTO, error) {
	resp, err := h.lookupIR.Execute(h.ctx, md5, sha256)
	if err != nil {
		return nil, err
	}
	if !resp.Registered {
		return nil, nil
	}
	result := &dto.ChartDTO{
		MD5:          md5,
		SHA256:       sha256,
		HasIRMeta:    true,
		LR2IRTags:    strings.Join(resp.Tags, ","),
		LR2IRBodyURL: resp.BodyURL,
		LR2IRDiffURL: resp.DiffURL,
		LR2IRNotes:   resp.Notes,
	}
	return result, nil
}

func (h *IRHandler) UpdateChartMeta(md5, sha256, workingBodyURL, workingDiffURL string) error {
	return h.updateChart.Execute(h.ctx, md5, sha256, workingBodyURL, workingDiffURL)
}

// StartBulkFetch はIR一括取得をバックグラウンドで開始する。二重起動不可。
func (h *IRHandler) StartBulkFetch() error {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return nil // 既に実行中
	}
	h.running = true
	ctx, cancel := context.WithCancel(h.ctx)
	h.cancelFunc = cancel
	h.mu.Unlock()

	go func() {
		defer func() {
			h.mu.Lock()
			h.running = false
			h.cancelFunc = nil
			h.mu.Unlock()
		}()

		result, err := h.bulkFetchIR.Execute(ctx, func(p usecase.BulkFetchProgress) {
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

// StopBulkFetch は実行中のIR一括取得を中断する
func (h *IRHandler) StopBulkFetch() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancelFunc != nil {
		h.cancelFunc()
	}
}

// IsBulkFetchRunning は一括取得が実行中かどうかを返す
func (h *IRHandler) IsBulkFetchRunning() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running
}
```

**Step 2: app.goのDI修正**

`app.go`のInit()内、`lookupIR`定義の後に`bulkFetchIR`を追加:

```go
bulkFetchIR := usecase.NewBulkFetchIRUseCase(irClient, elsaRepo)
```

`NewIRHandler`呼び出しを修正:

```go
a.IRHandler = internalapp.NewIRHandler(lookupIR, bulkFetchIR, updateChartMeta)
```

**Step 3: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: PASS

**Step 4: コミット**

```bash
git add internal/app/ir_handler.go app.go
git commit -m "feat: IRHandlerにStartBulkFetch/StopBulkFetchを追加"
```

---

### Task 5: フロントエンドUIの実装

**Files:**
- Modify: `frontend/src/SongTable.svelte`

**変更内容:** SongTableヘッダーバーに「IR取得」ボタンと進捗表示を追加。

**Step 1: SongTable.svelteの修正**

scriptブロック冒頭のimportに追加:

```typescript
import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime'
import { StartBulkFetch, StopBulkFetch } from '../wailsjs/go/app/IRHandler'
```

scriptブロックに状態管理を追加（`let inferenceModal` の直後あたり）:

```typescript
// IR一括取得の状態
let irFetching = false
let irProgress = { current: 0, total: 0 }
let irDoneMessage = ''
let irDoneTimer: ReturnType<typeof setTimeout> | null = null

function startBulkFetch() {
  irFetching = true
  irProgress = { current: 0, total: 0 }
  irDoneMessage = ''
  if (irDoneTimer) { clearTimeout(irDoneTimer); irDoneTimer = null }
  StartBulkFetch().catch((e: Error) => {
    console.error('StartBulkFetch failed:', e)
    irFetching = false
  })
}

function stopBulkFetch() {
  StopBulkFetch()
}

onMount(async () => {
  const offProgress = EventsOn('ir:progress', (data: { current: number; total: number }) => {
    irProgress = data
  })
  const offDone = EventsOn('ir:done', (data: { fetched: number; notFound: number; failed: number; cancelled: boolean }) => {
    irFetching = false
    const parts: string[] = []
    if (data.fetched > 0) parts.push(`${data.fetched}件取得`)
    if (data.notFound > 0) parts.push(`${data.notFound}件未登録`)
    if (data.failed > 0) parts.push(`${data.failed}件失敗`)
    if (data.cancelled) parts.push('中断')
    irDoneMessage = parts.join(', ')
    // 5秒後に消す
    irDoneTimer = setTimeout(() => { irDoneMessage = '' }, 5000)
    // 楽曲リスト再読み込み
    ListAllSongs().then(s => { songs = s || [] }).catch(console.error)
  })

  // 既存のonMount処理
  try {
    songs = (await ListAllSongs()) || []
  } catch (e) {
    console.error('Failed to load songs:', e)
  } finally {
    loading = false
  }

  return () => {
    offProgress()
    offDone()
    if (irDoneTimer) clearTimeout(irDoneTimer)
  }
})
```

注意: 既存のonMountとマージする。既存のonMountを削除し、上記の統合版に置き換える。

ヘッダーバーのボタンエリアに追加（`メタ推測`ボタンの前）:

```svelte
{#if irFetching}
  <span class="text-xs text-base-content/70">
    取得中: {irProgress.current.toLocaleString()} / {irProgress.total.toLocaleString()}
  </span>
  <button class="btn btn-xs btn-error btn-outline" on:click|stopPropagation={stopBulkFetch}>停止</button>
{:else if irDoneMessage}
  <span class="text-xs text-success">{irDoneMessage}</span>
{:else}
  <button class="btn btn-xs btn-outline" on:click|stopPropagation={startBulkFetch}>IR取得</button>
{/if}
```

**Step 2: Wailsバインディング生成**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails generate module`
Expected: `frontend/wailsjs/go/app/IRHandler.js` に `StartBulkFetch`, `StopBulkFetch`, `IsBulkFetchRunning` が生成される

**Step 3: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: PASS

**Step 4: コミット**

```bash
git add frontend/src/SongTable.svelte frontend/wailsjs/
git commit -m "feat: IR一括取得のUIを追加"
```

---

### Task 6: 結合テスト・動作確認

**Step 1: 全テスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./... -v`
Expected: PASS

**Step 2: ビルド**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: PASS

**Step 3: 動作確認チェックリスト**

ビルド成功後、実際のアプリで以下を確認:
1. SongTableヘッダーに「IR取得」ボタンが表示される
2. クリックで取得開始、進捗が「取得中: X / Y」で更新される
3. 「停止」ボタンで中断できる
4. 完了時にサマリーが表示され、5秒後に「IR取得」ボタンに戻る
5. 完了後、楽曲のIRカラムに●が増えている
