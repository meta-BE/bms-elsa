# 重複検知スキャンのバックグラウンドタスク化 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 重複検知スキャンを起動時に自動実行するバックグラウンドタスクに変更し、他の3タスクと統一的なUIにする

**Architecture:** MinHashスキャン完了時のコールバックで重複検知スキャンを起動。DuplicateHandlerにScanHandlerと同じバックグラウンドパターン（mutex/goroutine/EventsEmit）を追加し、結果をメモリキャッシュ。フロントはイベントリッスン+結果取得APIで表示。

**Tech Stack:** Go (Wails), Svelte, TypeScript

---

### Task 1: ScanHandler にコールバック引数を追加

**Files:**
- Modify: `internal/app/scan_handler.go:29-76`

- [ ] **Step 1: StartMinHashScan のシグネチャにonDoneを追加**

`internal/app/scan_handler.go` を以下のように変更:

```go
// StartMinHashScan はMinHash一括計算をバックグラウンドで開始する。二重起動不可。
func (h *ScanHandler) StartMinHashScan(onDone func()) error {
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

		wailsRuntime.EventsEmit(h.ctx, "scan:done", map[string]any{
			"total":     result.Total,
			"computed":  result.Computed,
			"skipped":   result.Skipped,
			"failed":    result.Failed,
			"cancelled": result.Cancelled,
		})

		if onDone != nil {
			onDone()
		}
	}()

	return nil
}
```

- [ ] **Step 2: ビルド確認**

Run: `go build ./...`
Expected: 成功

- [ ] **Step 3: コミット**

```bash
git add internal/app/scan_handler.go
git commit -m "feat: StartMinHashScanにonDoneコールバック引数を追加"
```

---

### Task 2: DuplicateHandler にバックグラウンド実行パターンを追加

**Files:**
- Modify: `internal/app/duplicate_handler.go`

- [ ] **Step 1: DuplicateHandler にフィールドとメソッドを追加**

`internal/app/duplicate_handler.go` を以下の内容で全体書き換え:

```go
package app

import (
	"context"
	"sync"

	"github.com/meta-BE/bms-elsa/internal/domain/similarity"
	"github.com/meta-BE/bms-elsa/internal/usecase"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// MergeFoldersResultDTO はフロントエンドに返すマージ結果
type MergeFoldersResultDTO struct {
	Success  bool   `json:"success"`
	Moved    int    `json:"moved"`
	Replaced int    `json:"replaced"`
	Skipped  int    `json:"skipped"`
	Errors   int    `json:"errors"`
	ErrorMsg string `json:"errorMsg"`
}

type DuplicateHandler struct {
	ctx            context.Context
	scanDuplicates *usecase.ScanDuplicatesUseCase
	mergeFolders   *usecase.MergeFoldersUseCase

	mu      sync.Mutex
	running bool
	results []similarity.DuplicateGroup
}

func NewDuplicateHandler(
	scanDuplicates *usecase.ScanDuplicatesUseCase,
	mergeFolders *usecase.MergeFoldersUseCase,
) *DuplicateHandler {
	return &DuplicateHandler{
		scanDuplicates: scanDuplicates,
		mergeFolders:   mergeFolders,
	}
}

func (h *DuplicateHandler) SetContext(ctx context.Context) { h.ctx = ctx }

// StartScanDuplicates はバックグラウンドで重複検知スキャンを開始する。二重起動不可。
func (h *DuplicateHandler) StartScanDuplicates() {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return
	}
	h.running = true
	h.mu.Unlock()

	wailsRuntime.EventsEmit(h.ctx, "dup:progress", map[string]int{
		"current": 0,
		"total":   1,
	})

	go func() {
		defer func() {
			h.mu.Lock()
			h.running = false
			h.mu.Unlock()
		}()

		groups, err := h.scanDuplicates.Execute(h.ctx)
		if err != nil {
			wailsRuntime.EventsEmit(h.ctx, "dup:done", map[string]any{
				"groups": 0,
				"error":  err.Error(),
			})
			return
		}

		h.mu.Lock()
		h.results = groups
		h.mu.Unlock()

		wailsRuntime.EventsEmit(h.ctx, "dup:progress", map[string]int{
			"current": 1,
			"total":   1,
		})
		wailsRuntime.EventsEmit(h.ctx, "dup:done", map[string]any{
			"groups": len(groups),
			"error":  "",
		})
	}()
}

// GetDuplicateGroups はキャッシュ済みのスキャン結果を返す
func (h *DuplicateHandler) GetDuplicateGroups() []similarity.DuplicateGroup {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.results
}

// IsDuplicateScanRunning はスキャンが実行中かどうかを返す
func (h *DuplicateHandler) IsDuplicateScanRunning() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running
}

// MergeFolders は srcDir を destDir にマージする
func (h *DuplicateHandler) MergeFolders(srcDir, destDir string) (*MergeFoldersResultDTO, error) {
	result, err := h.mergeFolders.Execute(h.ctx, srcDir, destDir)
	if err != nil {
		return &MergeFoldersResultDTO{Success: false, ErrorMsg: err.Error()}, nil
	}
	return &MergeFoldersResultDTO{
		Success:  result.Errors == 0,
		Moved:    result.Moved,
		Replaced: result.Replaced,
		Skipped:  result.Skipped,
		Errors:   result.Errors,
		ErrorMsg: result.ErrorMsg,
	}, nil
}
```

- [ ] **Step 2: ビルド確認**

Run: `go build ./...`
Expected: 成功

- [ ] **Step 3: コミット**

```bash
git add internal/app/duplicate_handler.go
git commit -m "feat: DuplicateHandlerにバックグラウンドスキャン実行パターンを追加"
```

---

### Task 3: startup() でコールバックチェーンを接続

**Files:**
- Modify: `app.go:137-141`

- [ ] **Step 1: startup() のMinHashスキャン起動にコールバックを追加**

`app.go` の startup() メソッド内を変更:

```go
	// バックグラウンドタスクを並列起動
	a.ScanHandler.StartMinHashScan(func() {
		a.DuplicateHandler.StartScanDuplicates()
	})
	a.DifficultyTableHandler.RefreshAllDifficultyTablesAsync()
	a.RewriteHandler.StartInferWorkingURLs()
```

- [ ] **Step 2: ビルド確認**

Run: `go build ./...`
Expected: 成功

- [ ] **Step 3: コミット**

```bash
git add app.go
git commit -m "feat: startup()でMinHash完了後に重複検知スキャンを自動起動"
```

---

### Task 4: DuplicateView.svelte をバックグラウンド結果表示に変更

**Files:**
- Modify: `frontend/src/views/DuplicateView.svelte`

- [ ] **Step 1: DuplicateView.svelte を書き換え**

スキャンボタンを削除し、イベントリッスン+結果取得に変更:

```svelte
<script lang="ts">
  import { createEventDispatcher, onMount, onDestroy } from 'svelte'
  import { handleArrowNav } from '../utils/arrowNav'
  import { GetDuplicateGroups, IsDuplicateScanRunning } from '../../wailsjs/go/app/DuplicateHandler'
  import { EventsOn } from '../../wailsjs/runtime/runtime'
  import type { similarity } from '../../wailsjs/go/models'

  const dispatch = createEventDispatcher()

  export let active = false

  let groups: similarity.DuplicateGroup[] = []
  let scanning = true
  let selectedGroupID: number | null = null

  async function loadResults() {
    const result = await GetDuplicateGroups()
    groups = (result || []).sort((a, b) => b.Score - a.Score)
  }

  let offDupDone: (() => void) | null = null

  onMount(async () => {
    // 既に完了しているか確認
    const running = await IsDuplicateScanRunning()
    if (!running) {
      await loadResults()
      scanning = false
    }

    offDupDone = EventsOn('dup:done', async () => {
      await loadResults()
      scanning = false
    })
  })

  onDestroy(() => {
    offDupDone?.()
  })

  function handleSelect(group: similarity.DuplicateGroup) {
    selectedGroupID = group.ID
    dispatch('select', group)
  }

  function handleKeyNav(e: KeyboardEvent) {
    if (!active || scanning) return
    handleArrowNav(e, {
      selected: selectedGroupID !== null ? String(selectedGroupID) : null,
      items: groups,
      getKey: (g: similarity.DuplicateGroup) => String(g.ID),
      onSelect: (g: similarity.DuplicateGroup) => handleSelect(g),
    })
  }

  $: selectedGroup = groups.find(g => g.ID === selectedGroupID) || null

  // App.svelte から呼び出される公開メソッド
  export function removeMember(folderHash: string) {
    for (const group of groups) {
      const idx = group.Members.findIndex(m => m.FolderHash === folderHash)
      if (idx !== -1) {
        group.Members.splice(idx, 1)
        groups = groups // リアクティビティ発火
        if (group.Members.length <= 1) {
          groups = groups.filter(g => g.ID !== group.ID)
          if (selectedGroupID === group.ID) {
            selectedGroupID = null
            dispatch('select', null)
          }
        }
        break
      }
    }
  }
</script>

<svelte:window on:keydown={handleKeyNav} />

{#if scanning}
  <div class="flex items-center justify-center h-full text-base-content/40 text-sm">
    スキャン中...
  </div>
{:else if groups.length === 0}
  <div class="flex items-center justify-center h-full text-base-content/40 text-sm">
    重複グループなし
  </div>
{:else}
  <div class="flex items-center gap-2 px-2 py-1 text-sm text-base-content/60 border-b border-base-300">
    <span>{groups.length} グループ</span>
  </div>
  <div class="overflow-y-auto h-full">
    <table class="table table-xs table-pin-rows">
      <thead>
        <tr>
          <th class="w-16">類似度</th>
          <th>タイトル</th>
          <th class="w-16">件数</th>
        </tr>
      </thead>
      <tbody>
        {#each groups as group}
          <tr
            class="cursor-pointer hover:bg-base-200 {selectedGroupID === group.ID ? 'bg-primary/10' : ''}"
            on:click={() => handleSelect(group)}
          >
            <td class="text-sm font-mono">{group.Score}%</td>
            <td class="text-sm">{group.Members[0]?.Title || ''}</td>
            <td class="text-sm">{group.Members.length}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}
```

- [ ] **Step 2: フロントエンドビルド確認**

Run: `cd frontend && npm run build`
Expected: 成功

- [ ] **Step 3: コミット**

```bash
git add frontend/src/views/DuplicateView.svelte
git commit -m "feat: DuplicateViewをバックグラウンドスキャン結果の自動表示に変更"
```

---

### Task 5: Settings.svelte に重複検知スキャンのセクションを追加

**Files:**
- Modify: `frontend/src/settings/Settings.svelte`

- [ ] **Step 1: import に IsDuplicateScanRunning を追加**

`frontend/src/settings/Settings.svelte` の import を変更:

既存:
```typescript
  import { IsMinHashScanRunning } from '../../wailsjs/go/app/ScanHandler'
```
の後に追加:
```typescript
  import { IsDuplicateScanRunning } from '../../wailsjs/go/app/DuplicateHandler'
```

- [ ] **Step 2: 状態変数を追加**

`rewriteResult` 変数宣言の後に追加:

```typescript
  let dupState: 'running' | 'done' | 'error' = 'done'
  let dupProgress = { current: 0, total: 0 }
  let dupError = ''
  let dupResult = ''
```

- [ ] **Step 3: open() に重複検知スキャンの状態確認を追加**

`open()` 内の `IsInferring` チェックの後（`dialog.showModal()` の前）に追加:

```typescript
    try {
      if (await IsDuplicateScanRunning()) {
        dupState = 'running'
        dupProgress = { current: 0, total: 1 }
      }
    } catch {}
```

- [ ] **Step 4: onMount にイベントリスナーを追加**

イベントリスナー変数宣言に追加:
```typescript
  let offDupProgress: (() => void) | null = null
  let offDupDone: (() => void) | null = null
```

onMount 内の `offRewriteDone = EventsOn(...)` の後に追加:
```typescript
    offDupProgress = EventsOn('dup:progress', (data: { current: number; total: number }) => {
      dupState = 'running'
      dupProgress = data
    })
    offDupDone = EventsOn('dup:done', (data: { groups: number; error: string }) => {
      if (data.error) {
        dupState = 'error'
        dupError = data.error
      } else {
        dupState = 'done'
        dupResult = `${data.groups}グループ検出`
      }
      dupProgress = { current: 1, total: 1 }
    })
```

onDestroy 内に追加:
```typescript
    offDupProgress?.()
    offDupDone?.()
```

- [ ] **Step 5: UIセクションを追加**

「動作URL推定」セクションの `</div>` の後、`</div><!-- space-y-3 -->` の前に追加:

```svelte
      <!-- 重複検知スキャン -->
      <div>
        <div class="flex items-center justify-between text-sm mb-1">
          <span>重複検知スキャン</span>
          {#if dupState === 'running'}
            <span class="text-xs text-base-content/50">実行中...</span>
          {:else if dupState === 'error'}
            <span class="text-xs text-error">エラー</span>
          {:else}
            <span class="text-xs text-success">完了</span>
          {/if}
        </div>
        {#if dupState === 'running' && dupProgress.total > 0}
          <ProgressBar current={dupProgress.current} total={dupProgress.total} />
        {/if}
        {#if dupState !== 'running' && dupResult}
          <p class="text-xs text-base-content/50">{dupResult}</p>
        {/if}
      </div>
```

- [ ] **Step 6: フロントエンドビルド確認**

Run: `cd frontend && npm run build`
Expected: 成功

- [ ] **Step 7: コミット**

```bash
git add frontend/src/settings/Settings.svelte
git commit -m "feat: Settings.svelteに重複検知スキャンのバックグラウンドタスク表示を追加"
```

---

### Task 6: 旧 ScanDuplicates RPC の削除と全体ビルド確認

**Files:**
- Modify: `internal/app/duplicate_handler.go`（ScanDuplicates メソッドの削除）

- [ ] **Step 1: ScanDuplicates メソッドを削除**

`internal/app/duplicate_handler.go` から旧 `ScanDuplicates` メソッドを削除する。Task 2 で書き換え済みのため、すでに削除されている場合はスキップ。

- [ ] **Step 2: 全体ビルド確認**

Run: `go build ./... && cd frontend && npm run build`
Expected: 両方成功

- [ ] **Step 3: コミット**

```bash
git add -A
git commit -m "chore: 旧ScanDuplicates同期メソッドを削除しビルド確認"
```

---

### Task 7: マニュアル更新

**Files:**
- Modify: `docs/manual.md`

- [ ] **Step 1: マニュアルの重複検知セクションを更新**

重複検知の説明を「起動時に自動実行される」旨に変更する。「スキャン実行」ボタンの記述を削除し、バックグラウンドタスクとして自動実行される旨を記載する。

- [ ] **Step 2: コミット**

```bash
git add docs/manual.md
git commit -m "docs: マニュアルの重複検知説明を起動時自動実行に更新"
```
