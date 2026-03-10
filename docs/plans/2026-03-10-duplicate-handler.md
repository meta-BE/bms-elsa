# DuplicateHandler 新設 実装計画

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ScanDuplicatesUseCase を既存 Handler パターンに合わせて DuplicateHandler 経由に変更し、アーキテクチャの一貫性を確保する。

**Architecture:** `internal/app/duplicate_handler.go` を新設し、`app.go` の UseCase 直接保持を Handler 経由に変更する。フロントエンドの import パスも `App.ScanDuplicates` → `DuplicateHandler.ScanDuplicates` に更新する。

**Tech Stack:** Go (Wails v2), Svelte (frontend)

---

## Chunk 1: DuplicateHandler 新設とフロントエンド更新

### Task 1: DuplicateHandler を作成

**Files:**
- Create: `internal/app/duplicate_handler.go`

- [ ] **Step 1: DuplicateHandler を作成**

```go
package app

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/domain/similarity"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

type DuplicateHandler struct {
	ctx            context.Context
	scanDuplicates *usecase.ScanDuplicatesUseCase
}

func NewDuplicateHandler(scanDuplicates *usecase.ScanDuplicatesUseCase) *DuplicateHandler {
	return &DuplicateHandler{scanDuplicates: scanDuplicates}
}

func (h *DuplicateHandler) SetContext(ctx context.Context) { h.ctx = ctx }

func (h *DuplicateHandler) ScanDuplicates() ([]similarity.DuplicateGroup, error) {
	return h.scanDuplicates.Execute(h.ctx)
}
```

---

### Task 2: app.go を更新

**Files:**
- Modify: `app.go:23-36` (App struct)
- Modify: `app.go:67-73` (Init)
- Modify: `app.go:106-116` (startup)
- Modify: `app.go:244-247` (ScanDuplicates メソッド削除)

- [ ] **Step 1: App struct のフィールドを変更**

`scanDuplicates *usecase.ScanDuplicatesUseCase` を `DuplicateHandler *internalapp.DuplicateHandler` に置き換える。
import から `"github.com/meta-BE/bms-elsa/internal/usecase"` を削除（他で使っていなければ）。
import から `"github.com/meta-BE/bms-elsa/internal/domain/similarity"` を削除（他で使っていなければ）。

- [ ] **Step 2: Init() の DI 組み立てを変更**

```go
// 変更前:
// a.scanDuplicates = usecase.NewScanDuplicatesUseCase(songdataReader)

// 変更後:
scanDuplicates := usecase.NewScanDuplicatesUseCase(songdataReader)
a.DuplicateHandler = internalapp.NewDuplicateHandler(scanDuplicates)
```

- [ ] **Step 3: startup() に SetContext を追加**

```go
a.DuplicateHandler.SetContext(ctx)
```

- [ ] **Step 4: App.ScanDuplicates() メソッドを削除**

`app.go` 244-247行目の `ScanDuplicates` メソッドとそのコメントを削除する。

---

### Task 3: main.go の Bind を更新

**Files:**
- Modify: `main.go:34-44` (Bind)

- [ ] **Step 1: Bind に DuplicateHandler を追加**

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
    app.DiffImportHandler,
    app.DuplicateHandler,
},
```

---

### Task 4: ビルド確認

- [ ] **Step 1: Go ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: エラーなし

---

### Task 5: フロントエンドの import パスを更新

**Files:**
- Modify: `frontend/src/views/DuplicateView.svelte:3`

- [ ] **Step 1: import を更新**

```svelte
// 変更前:
import { ScanDuplicates } from '../../wailsjs/go/main/App'

// 変更後:
import { ScanDuplicates } from '../../wailsjs/go/app/DuplicateHandler'
```

注: Wails の自動生成コード（`frontend/wailsjs/go/`）は `wails dev` 再起動時に自動再生成される。
手動で確認するには `wails generate module` を実行する。

---

### Task 6: コミット

- [ ] **Step 1: 変更をコミット**

```bash
git add internal/app/duplicate_handler.go app.go main.go frontend/src/views/DuplicateView.svelte
git commit -m "refactor: ScanDuplicatesUseCase を DuplicateHandler 経由に変更"
```
