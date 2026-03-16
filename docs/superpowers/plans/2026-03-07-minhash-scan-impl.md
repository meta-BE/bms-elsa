# MinHashスキャン 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** songdata.db登録済み譜面のBMSファイルをパースしてMinHash署名を計算し、chart_meta.wav_minhashに保存する機能を実装する。

**Architecture:** IR一括取得と同じパターン（mutex + context.WithCancel + goroutine + Wailsイベント）でScanHandlerを実装。ElsaRepositoryに走査対象リスト取得とMinHash更新の2メソッドを追加。フロントエンドは譜面一覧タブにボタン・進捗表示を追加。

**Tech Stack:** Go, SQLite (modernc.org/sqlite), Svelte 4, TypeScript, Wails v2

---

### Task 1: ElsaRepositoryにListChartsWithoutMinhash追加

songdata.dbとchart_metaをJOINして、wav_minhashがNULLの譜面のmd5+pathリストを返すメソッドを追加する。

**Files:**
- Modify: `internal/adapter/persistence/elsa_repository.go`
- Test: `internal/adapter/persistence/songdata_reader_test.go`

**Step 1: テストを書く**

`songdata_reader_test.go` の末尾に追加:

```go
func TestListChartsWithoutMinhash(t *testing.T) {
	_, db := setupSongdataReader(t)
	repo := persistence.NewElsaRepository(db)

	targets, err := repo.ListChartsWithoutMinhash(context.Background())
	if err != nil {
		t.Fatalf("ListChartsWithoutMinhash failed: %v", err)
	}

	// songdata.dbには譜面が存在するので、chart_metaが空の状態では全譜面が対象
	if len(targets) == 0 {
		t.Fatal("expected non-empty targets, got 0")
	}

	// 各ターゲットにMD5とPathが設定されていることを確認
	for _, tgt := range targets {
		if tgt.MD5 == "" {
			t.Error("target has empty MD5")
		}
		if tgt.Path == "" {
			t.Error("target has empty Path")
		}
	}
}
```

**Step 2: テストが失敗することを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/adapter/persistence/ -run TestListChartsWithoutMinhash -v`
Expected: コンパイルエラー（ListChartsWithoutMinhashが存在しない）

**Step 3: 実装**

`elsa_repository.go` の末尾に追加:

```go
// ChartScanTarget はMinHashスキャン対象の譜面情報
type ChartScanTarget struct {
	MD5  string
	Path string
}

// ListChartsWithoutMinhash はwav_minhashが未計算の譜面リストを返す
func (r *ElsaRepository) ListChartsWithoutMinhash(ctx context.Context) ([]ChartScanTarget, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT s.md5, s.path
		FROM songdata.song s
		LEFT JOIN chart_meta cm ON s.md5 = cm.md5
		WHERE cm.id IS NULL OR cm.wav_minhash IS NULL
		ORDER BY s.md5`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []ChartScanTarget
	for rows.Next() {
		var t ChartScanTarget
		if err := rows.Scan(&t.MD5, &t.Path); err != nil {
			return nil, err
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}
```

**Step 4: テストがパスすることを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/adapter/persistence/ -run TestListChartsWithoutMinhash -v`
Expected: PASS

**Step 5: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add internal/adapter/persistence/elsa_repository.go internal/adapter/persistence/songdata_reader_test.go
git commit -m "feat: ElsaRepositoryにListChartsWithoutMinhashを追加"
```

---

### Task 2: ElsaRepositoryにUpdateWavMinhash追加

chart_metaのwav_minhashカラムを更新するメソッドを追加する。chart_metaにレコードが存在しない場合はINSERTする。

**Files:**
- Modify: `internal/adapter/persistence/elsa_repository.go`
- Test: `internal/adapter/persistence/songdata_reader_test.go`

**Step 1: テストを書く**

`songdata_reader_test.go` の末尾に追加:

```go
func TestUpdateWavMinhash(t *testing.T) {
	_, db := setupSongdataReader(t)
	repo := persistence.NewElsaRepository(db)

	// テスト用のMinHashデータ（256バイト）
	minhash := make([]byte, 256)
	for i := range minhash {
		minhash[i] = byte(i)
	}

	// songdata.dbから実在するMD5を1つ取得
	targets, err := repo.ListChartsWithoutMinhash(context.Background())
	if err != nil || len(targets) == 0 {
		t.Fatal("ListChartsWithoutMinhash failed or empty")
	}
	md5 := targets[0].MD5

	// UpdateWavMinhashを実行
	if err := repo.UpdateWavMinhash(context.Background(), md5, minhash); err != nil {
		t.Fatalf("UpdateWavMinhash failed: %v", err)
	}

	// wav_minhashが保存されたことを確認
	var stored []byte
	err = db.QueryRow(`SELECT wav_minhash FROM chart_meta WHERE md5 = ?`, md5).Scan(&stored)
	if err != nil {
		t.Fatalf("failed to read wav_minhash: %v", err)
	}
	if len(stored) != 256 {
		t.Fatalf("expected 256 bytes, got %d", len(stored))
	}
	for i, b := range stored {
		if b != byte(i) {
			t.Fatalf("byte %d: expected %d, got %d", i, byte(i), b)
		}
	}

	// 更新後はListChartsWithoutMinhashから除外されることを確認
	targets2, err := repo.ListChartsWithoutMinhash(context.Background())
	if err != nil {
		t.Fatalf("ListChartsWithoutMinhash after update failed: %v", err)
	}
	for _, tgt := range targets2 {
		if tgt.MD5 == md5 {
			t.Errorf("md5 %s should not appear after minhash update", md5)
		}
	}
}
```

**Step 2: テストが失敗することを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/adapter/persistence/ -run TestUpdateWavMinhash -v`
Expected: コンパイルエラー（UpdateWavMinhashが存在しない）

**Step 3: 実装**

`elsa_repository.go` の末尾に追加:

```go
// UpdateWavMinhash はchart_metaのwav_minhashを更新する。レコードが存在しない場合はINSERTする。
func (r *ElsaRepository) UpdateWavMinhash(ctx context.Context, md5 string, minhash []byte) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO chart_meta (md5, wav_minhash)
		 VALUES (?, ?)
		 ON CONFLICT(md5) DO UPDATE SET
		   wav_minhash = excluded.wav_minhash,
		   updated_at  = datetime('now')`,
		md5, minhash,
	)
	return err
}
```

**Step 4: テストがパスすることを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/adapter/persistence/ -run TestUpdateWavMinhash -v`
Expected: PASS

**Step 5: 既存テストが壊れていないことを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/adapter/persistence/ -v`
Expected: 全テストPASS

**Step 6: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add internal/adapter/persistence/elsa_repository.go internal/adapter/persistence/songdata_reader_test.go
git commit -m "feat: ElsaRepositoryにUpdateWavMinhashを追加"
```

---

### Task 3: ScanHandler作成

IR一括取得（`IRHandler`）と同じパターンで、MinHashスキャンのハンドラーを作成する。

**Files:**
- Create: `internal/app/scan_handler.go`

**Step 1: ScanHandlerを実装**

`internal/app/scan_handler.go` を作成:

```go
package app

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
	"github.com/meta-BE/bms-elsa/internal/domain/bms"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ScanHandler struct {
	ctx      context.Context
	elsaRepo *persistence.ElsaRepository

	mu         sync.Mutex
	running    bool
	cancelFunc context.CancelFunc
}

func NewScanHandler(elsaRepo *persistence.ElsaRepository) *ScanHandler {
	return &ScanHandler{elsaRepo: elsaRepo}
}

func (h *ScanHandler) SetContext(ctx context.Context) { h.ctx = ctx }

// StartMinHashScan はMinHash一括計算をバックグラウンドで開始する。二重起動不可。
func (h *ScanHandler) StartMinHashScan() error {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return nil
	}
	h.running = true
	h.mu.Unlock()

	targets, err := h.elsaRepo.ListChartsWithoutMinhash(h.ctx)
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

		total := len(targets)
		computed := 0
		skipped := 0
		failed := 0
		cancelled := false

		for i, tgt := range targets {
			select {
			case <-ctx.Done():
				cancelled = true
				goto done
			default:
			}

			// ファイル存在チェック
			if _, err := os.Stat(tgt.Path); err != nil {
				skipped++
				wailsRuntime.EventsEmit(h.ctx, "scan:progress", map[string]int{
					"current": i + 1, "total": total,
				})
				continue
			}

			// BMSパース
			wavFiles, err := bms.ParseWAVFiles(tgt.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "scan: parse error %s: %v\n", tgt.Path, err)
				failed++
				wailsRuntime.EventsEmit(h.ctx, "scan:progress", map[string]int{
					"current": i + 1, "total": total,
				})
				continue
			}

			// MinHash計算・保存
			sig := bms.ComputeMinHash(wavFiles)
			if err := h.elsaRepo.UpdateWavMinhash(h.ctx, tgt.MD5, sig.Bytes()); err != nil {
				fmt.Fprintf(os.Stderr, "scan: db error %s: %v\n", tgt.MD5, err)
				failed++
				wailsRuntime.EventsEmit(h.ctx, "scan:progress", map[string]int{
					"current": i + 1, "total": total,
				})
				continue
			}

			computed++
			wailsRuntime.EventsEmit(h.ctx, "scan:progress", map[string]int{
				"current": i + 1, "total": total,
			})
		}

	done:
		wailsRuntime.EventsEmit(h.ctx, "scan:done", map[string]interface{}{
			"total":     total,
			"computed":  computed,
			"skipped":   skipped,
			"failed":    failed,
			"cancelled": cancelled,
		})
	}()

	return nil
}

// StopMinHashScan は実行中のMinHashスキャンを中断する
func (h *ScanHandler) StopMinHashScan() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancelFunc != nil {
		h.cancelFunc()
	}
}

// IsMinHashScanRunning はMinHashスキャンが実行中かどうかを返す
func (h *ScanHandler) IsMinHashScanRunning() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running
}
```

**Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./internal/app/`
Expected: SUCCESS（ビルド成功）

**Step 3: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add internal/app/scan_handler.go
git commit -m "feat: MinHashスキャン用ScanHandlerを作成"
```

---

### Task 4: app.goとmain.goにScanHandler登録

AppにScanHandlerフィールドを追加し、Init()で初期化、startup()でSetContext、main.goでBind登録する。

**Files:**
- Modify: `app.go`
- Modify: `main.go`

**Step 1: app.goを修正**

`app.go` の `App` 構造体に `ScanHandler` フィールドを追加:

```go
type App struct {
	ctx                    context.Context
	db                     *sql.DB
	SongHandler            *internalapp.SongHandler
	IRHandler              *internalapp.IRHandler
	InferenceHandler       *internalapp.InferenceHandler
	RewriteHandler         *internalapp.RewriteHandler
	ChartHandler           *internalapp.ChartHandler
	DifficultyTableHandler *internalapp.DifficultyTableHandler
	ScanHandler            *internalapp.ScanHandler
	songReader             *persistence.SongdataReader
	elsaRepo               *persistence.ElsaRepository
}
```

`Init()` メソッドの末尾（`return nil` の前）に追加:

```go
	a.ScanHandler = internalapp.NewScanHandler(elsaRepo)
```

`startup()` メソッドに追加:

```go
	a.ScanHandler.SetContext(ctx)
```

**Step 2: main.goを修正**

`Bind` リストに `app.ScanHandler` を追加:

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
		},
```

**Step 3: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build .`
Expected: SUCCESS

**Step 4: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add app.go main.go
git commit -m "feat: ScanHandlerをApp・Wailsバインディングに登録"
```

---

### Task 5: フロントエンドにMinHashスキャンUI追加

譜面一覧タブにMinHash計算ボタン・進捗表示・停止ボタン・完了メッセージを追加する。IR一括取得UIと同じパターン。

**Files:**
- Modify: `frontend/src/views/ChartListView.svelte`

**Step 1: import追加**

`ChartListView.svelte` の既存import（`StartBulkFetch, StopBulkFetch` のimport行の後）に追加:

```typescript
import { StartMinHashScan, StopMinHashScan } from '../../wailsjs/go/app/ScanHandler'
```

**Step 2: 状態変数追加**

IR一括取得の状態変数（`let irDoneTimer` の行の後）に追加:

```typescript
  // MinHashスキャンの状態
  let scanRunning = false
  let scanProgress = { current: 0, total: 0 }
  let scanDoneMessage = ''
  let scanDoneTimer: ReturnType<typeof setTimeout> | null = null
```

**Step 3: 関数追加**

`stopBulkFetch` 関数の後に追加:

```typescript
  function startMinHashScan() {
    scanRunning = true
    scanProgress = { current: 0, total: 0 }
    scanDoneMessage = ''
    if (scanDoneTimer) { clearTimeout(scanDoneTimer); scanDoneTimer = null }
    StartMinHashScan().catch((e: Error) => {
      console.error('[Scan] StartMinHashScan failed:', e)
      scanRunning = false
    })
  }

  function stopMinHashScan() {
    StopMinHashScan()
  }
```

**Step 4: イベントリスナー追加**

`onMount` 内の `offDone = EventsOn('ir:done', ...)` ブロックの後に追加:

```typescript
    offScanProgress = EventsOn('scan:progress', (data: { current: number; total: number }) => {
      scanProgress = data
    })
    offScanDone = EventsOn('scan:done', (data: { total: number; computed: number; skipped: number; failed: number; cancelled: boolean }) => {
      scanRunning = false
      const parts: string[] = []
      if (data.total === 0) {
        scanDoneMessage = '対象なし'
      } else {
        if (data.computed > 0) parts.push(`${data.computed}件計算`)
        if (data.skipped > 0) parts.push(`${data.skipped}件スキップ`)
        if (data.failed > 0) parts.push(`${data.failed}件失敗`)
        if (data.cancelled) parts.push('中断')
        scanDoneMessage = parts.join(', ') || '完了'
      }
      scanDoneTimer = setTimeout(() => { scanDoneMessage = '' }, 5000)
    })
```

リスナー変数宣言を追加（`let offDone` の行の後）:

```typescript
  let offScanProgress: (() => void) | null = null
  let offScanDone: (() => void) | null = null
```

`onDestroy` に追加:

```typescript
    offScanProgress?.()
    offScanDone?.()
    if (scanDoneTimer) clearTimeout(scanDoneTimer)
```

**Step 5: テンプレートにMinHashスキャンUIを追加**

IR取得ボタン群（`{#if irFetching}...{/if}` ブロック）の前に追加:

```svelte
      {#if scanRunning}
        <span class="text-xs text-base-content/70">
          計算中: {scanProgress.current.toLocaleString()} / {scanProgress.total.toLocaleString()}
        </span>
        <button class="btn btn-xs btn-error btn-outline" on:click|stopPropagation={stopMinHashScan}>停止</button>
      {:else if scanDoneMessage}
        <span class="text-xs text-success">{scanDoneMessage}</span>
      {:else}
        <button class="btn btn-xs btn-outline" on:click|stopPropagation={startMinHashScan}>MinHash計算</button>
      {/if}
```

**Step 6: フロントエンドビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run build`
Expected: ビルド成功（wailsjsバインディングはWails生成後に型が揃う。ビルドエラーの場合はTask 6のwails generateで解消）

**Step 7: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add frontend/src/views/ChartListView.svelte
git commit -m "feat: 譜面一覧タブにMinHashスキャンUIを追加"
```

---

### Task 6: Wailsバインディング再生成・フルビルド

Wailsバインディングを再生成し、フルビルドして動作確認する。

**Files:**
- 自動生成: `frontend/wailsjs/go/app/ScanHandler.{js,d.ts}`

**Step 1: Wailsバインディング再生成**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails generate module`
Expected: `frontend/wailsjs/go/app/ScanHandler.js` と `ScanHandler.d.ts` が生成される

**Step 2: フロントエンドビルド**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run build`
Expected: ビルド成功

**Step 3: Wailsフルビルド**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: `build/bin/` にアプリが生成される

**Step 4: 全テスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./... -v`
Expected: 全テストPASS

**Step 5: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add -A
git commit -m "feat: Wailsバインディング再生成・フルビルド確認"
```

注意: `frontend/wailsjs/` は `.gitignore` に含まれているため、`git add -A` でも追加されない。ビルド成果物のみが対象。
