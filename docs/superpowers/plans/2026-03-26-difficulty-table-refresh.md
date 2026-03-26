# 難易度表更新の並列化・進捗表示・個別更新 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 難易度表の一括更新を並列化（最大5並列）し、進捗表示とキャンセル機能を追加する。メインUIに個別更新ボタンを追加する。

**Architecture:** IR一括取得（`ir_handler.go`）と同じWails Eventsパターンを踏襲。バックエンドでgoroutine+semaphoreによる並列フェッチを実行し、`dt:refresh-progress` / `dt:refresh-done` イベントでフロントエンドに進捗を通知する。メインUIの個別更新は既存の同期RPC `RefreshDifficultyTable(id)` をそのまま使用。

**Tech Stack:** Go (sync, context), Wails v2 Events, Svelte 4, TypeScript

---

## ファイル構成

| 操作 | ファイル | 責務 |
|------|---------|------|
| 変更 | `internal/app/difficulty_table_handler.go` | 非同期一括更新API、進捗状態管理、キャンセル |
| 変更 | `frontend/src/settings/DifficultyTableSettings.svelte` | イベント駆動の進捗表示、停止ボタン |
| 変更 | `frontend/src/views/DifficultyTableView.svelte` | 個別更新ボタン追加 |
| 自動生成 | `frontend/wailsjs/go/app/DifficultyTableHandler.{js,d.ts}` | Wailsバインディング（`wails generate module`） |

---

### Task 1: バックエンド — 非同期一括更新API

**Files:**
- Modify: `internal/app/difficulty_table_handler.go`

- [ ] **Step 1: DifficultyTableHandler構造体に状態管理フィールドを追加**

`ir_handler.go` の `IRHandler` と同じパターンで、mutex・running・cancelFunc・進捗カウンタを追加する。

```go
// difficulty_table_handler.go のimportに追加:
import (
	"context"
	"sync"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
	"github.com/meta-BE/bms-elsa/internal/app/dto"
	"github.com/meta-BE/bms-elsa/internal/usecase"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// DifficultyTableHandler構造体を以下に置き換え:
type DifficultyTableHandler struct {
	ctx             context.Context
	dtRepo          *persistence.DifficultyTableRepository
	dtFetcher       *gateway.DifficultyTableFetcher
	songReader      *persistence.SongdataReader
	estimateUseCase *usecase.EstimateInstallLocationUseCase

	// 非同期一括更新の状態管理
	mu         sync.Mutex
	refreshing bool
	cancelFunc context.CancelFunc
	progress   struct{ current, total int }
}
```

- [ ] **Step 2: RefreshAllDifficultyTablesAsync メソッドを追加**

```go
// RefreshAllDifficultyTablesAsync は全難易度表を最大5並列で非同期更新する。
// 二重実行時はエラーを返す。進捗は dt:refresh-progress イベントで通知する。
func (h *DifficultyTableHandler) RefreshAllDifficultyTablesAsync() error {
	h.mu.Lock()
	if h.refreshing {
		h.mu.Unlock()
		return fmt.Errorf("既に更新中です")
	}
	h.refreshing = true
	h.mu.Unlock()

	tables, err := h.dtRepo.ListTables(h.ctx)
	if err != nil {
		h.mu.Lock()
		h.refreshing = false
		h.mu.Unlock()
		return err
	}

	ctx, cancel := context.WithCancel(h.ctx)
	h.mu.Lock()
	h.cancelFunc = cancel
	h.progress.current = 0
	h.progress.total = len(tables)
	h.mu.Unlock()

	go func() {
		defer func() {
			h.mu.Lock()
			h.refreshing = false
			h.cancelFunc = nil
			h.mu.Unlock()
		}()

		sem := make(chan struct{}, 5)
		var wg sync.WaitGroup
		var mu sync.Mutex
		results := make([]dto.DifficultyTableRefreshResult, len(tables))
		completed := 0

		for i, t := range tables {
			select {
			case <-ctx.Done():
				// キャンセルされた場合、残りのテーブルはスキップ
				mu.Lock()
				for j := i; j < len(tables); j++ {
					results[j] = dto.DifficultyTableRefreshResult{
						TableName: tables[j].Name, Error: "キャンセルされました",
					}
				}
				mu.Unlock()
				goto done
			case sem <- struct{}{}:
			}

			wg.Add(1)
			go func(idx int, tbl persistence.DifficultyTable) {
				defer func() { <-sem; wg.Done() }()
				result := h.refreshTable(tbl)
				mu.Lock()
				results[idx] = result
				completed++
				c := completed
				mu.Unlock()

				h.mu.Lock()
				h.progress.current = c
				h.mu.Unlock()

				wailsRuntime.EventsEmit(h.ctx, "dt:refresh-progress", map[string]any{
					"current":   c,
					"total":     len(tables),
					"tableName": tbl.Name,
					"success":   result.Success,
					"error":     result.Error,
				})
			}(i, t)
		}
		wg.Wait()

	done:
		wg.Wait()
		wailsRuntime.EventsEmit(h.ctx, "dt:refresh-done", map[string]any{
			"results": results,
		})
	}()

	return nil
}
```

- [ ] **Step 3: StopDifficultyTableRefresh, IsRefreshing, RefreshProgress メソッドを追加**

```go
// StopDifficultyTableRefresh は実行中の一括更新をキャンセルする
func (h *DifficultyTableHandler) StopDifficultyTableRefresh() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancelFunc != nil {
		h.cancelFunc()
	}
}

// IsRefreshing は一括更新が実行中かどうかを返す
func (h *DifficultyTableHandler) IsRefreshing() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.refreshing
}

// RefreshProgress は実行中の一括更新の進捗を返す
func (h *DifficultyTableHandler) RefreshProgress() map[string]int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return map[string]int{
		"current": h.progress.current,
		"total":   h.progress.total,
	}
}
```

- [ ] **Step 4: importに `fmt`, `sync`, `wailsRuntime` を追加**

既存のimportブロックを更新:

```go
import (
	"context"
	"fmt"
	"sync"

	"github.com/meta-BE/bms-elsa/internal/adapter/gateway"
	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
	"github.com/meta-BE/bms-elsa/internal/app/dto"
	"github.com/meta-BE/bms-elsa/internal/usecase"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)
```

- [ ] **Step 5: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 6: コミット**

```bash
git add internal/app/difficulty_table_handler.go
git commit -m "feat: 難易度表の非同期一括更新APIを追加（5並列、進捗通知、キャンセル対応）"
```

---

### Task 2: Wailsバインディング再生成

**Files:**
- 自動生成: `frontend/wailsjs/go/app/DifficultyTableHandler.js`
- 自動生成: `frontend/wailsjs/go/app/DifficultyTableHandler.d.ts`

- [ ] **Step 1: Wailsバインディングを再生成**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails generate module`
Expected: 新しいメソッド（`RefreshAllDifficultyTablesAsync`, `StopDifficultyTableRefresh`, `IsRefreshing`, `RefreshProgress`）が `DifficultyTableHandler.d.ts` に追加される

- [ ] **Step 2: 生成結果を確認**

`frontend/wailsjs/go/app/DifficultyTableHandler.d.ts` に以下が含まれることを確認:
```typescript
export function RefreshAllDifficultyTablesAsync():Promise<void>;
export function StopDifficultyTableRefresh():Promise<void>;
export function IsRefreshing():Promise<boolean>;
export function RefreshProgress():Promise<{[key: string]: number}>;
```

- [ ] **Step 3: コミット**

```bash
git add frontend/wailsjs/
git commit -m "chore: Wailsバインディング再生成（難易度表非同期更新API追加）"
```

---

### Task 3: フロントエンド — 設定ダイアログの一括更新を非同期化

**Files:**
- Modify: `frontend/src/settings/DifficultyTableSettings.svelte`

- [ ] **Step 1: import文を更新**

既存:
```typescript
import { ListDifficultyTables, AddDifficultyTable, RemoveDifficultyTable, RefreshAllDifficultyTables, ReorderDifficultyTables } from '../../wailsjs/go/app/DifficultyTableHandler'
```

変更後:
```typescript
import { ListDifficultyTables, AddDifficultyTable, RemoveDifficultyTable, RefreshAllDifficultyTablesAsync, StopDifficultyTableRefresh, IsRefreshing, RefreshProgress, ReorderDifficultyTables } from '../../wailsjs/go/app/DifficultyTableHandler'
import { onMount, onDestroy, createEventDispatcher } from 'svelte'
import { EventsOn } from '../../wailsjs/runtime/runtime'
```

- [ ] **Step 2: 状態変数とイベントリスナーを追加**

既存の `let refreshing = false` の後に進捗変数を追加し、`handleRefreshAll` を非同期版に置き換え、イベントリスナーを追加:

```typescript
  let refreshing = false
  let refreshProgress = { current: 0, total: 0 }

  let offProgress: (() => void) | null = null
  let offDone: (() => void) | null = null

  onMount(async () => {
    // ダイアログ再オープン時の進捗復元
    const running = await IsRefreshing()
    if (running) {
      refreshing = true
      const p = await RefreshProgress()
      refreshProgress = { current: p.current || 0, total: p.total || 0 }
    }

    offProgress = EventsOn('dt:refresh-progress', (data: { current: number; total: number }) => {
      if (!refreshing) return
      refreshProgress = { current: data.current, total: data.total }
    })
    offDone = EventsOn('dt:refresh-done', (data: { results: any[] }) => {
      refreshing = false
      refreshResults = data.results
      loadTables()
    })
  })

  onDestroy(() => {
    offProgress?.()
    offDone?.()
  })
```

- [ ] **Step 3: handleRefreshAll を非同期版に置き換え**

既存の `handleRefreshAll` 関数を以下に置き換え:

```typescript
  async function handleRefreshAll() {
    refreshing = true
    refreshResults = null
    refreshProgress = { current: 0, total: 0 }
    try {
      await RefreshAllDifficultyTablesAsync()
    } catch (e: any) {
      refreshing = false
      refreshResults = [{ tableName: '', success: false, error: e?.message || '更新に失敗しました' }]
    }
  }

  function handleStopRefresh() {
    StopDifficultyTableRefresh()
  }
```

- [ ] **Step 4: テンプレートの更新ボタンと進捗表示を変更**

既存の「全て更新」ボタン部分:
```svelte
    {#if tables.length > 0}
      <button class="btn btn-sm btn-outline mt-2" on:click={handleRefreshAll} disabled={refreshing}>
        {refreshing ? '更新中...' : '全て更新'}
      </button>
    {/if}
```

変更後:
```svelte
    {#if tables.length > 0}
      <div class="flex items-center gap-2 mt-2">
        {#if refreshing}
          <span class="text-sm text-base-content/70">更新中: {refreshProgress.current}/{refreshProgress.total} テーブル完了</span>
          <button class="btn btn-sm btn-error btn-outline" on:click={handleStopRefresh}>停止</button>
        {:else}
          <button class="btn btn-sm btn-outline" on:click={handleRefreshAll}>全て更新</button>
        {/if}
      </div>
    {/if}
```

- [ ] **Step 5: 動作確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails dev`

確認事項:
1. 難易度表設定ダイアログを開き「全て更新」をクリック
2. 「更新中: 0/N テーブル完了」→「1/N」→...と進捗がリアルタイム更新される
3. 完了後にチェックマーク/エラー表示の結果一覧が表示される
4. 更新中に「停止」をクリックすると中断される
5. ダイアログを閉じて再度開くと、更新中なら進捗が復元される

- [ ] **Step 6: コミット**

```bash
git add frontend/src/settings/DifficultyTableSettings.svelte
git commit -m "feat: 難易度表設定ダイアログの一括更新を非同期化（進捗表示・停止ボタン）"
```

---

### Task 4: フロントエンド — メインUIに個別更新ボタンを追加

**Files:**
- Modify: `frontend/src/views/DifficultyTableView.svelte`

- [ ] **Step 1: import文にRefreshDifficultyTableとIconを追加**

既存のimport文に追加:
```typescript
  import { ListDifficultyTables, ListDifficultyTableEntries, RefreshDifficultyTable } from '../../wailsjs/go/app/DifficultyTableHandler'
  import Icon from '../components/Icon.svelte'
```

- [ ] **Step 2: 更新状態変数を追加**

`let dtSettingsComponent: DifficultyTableSettings` の後に追加:

```typescript
  let refreshingSingle = false
  let refreshDoneMessage = ''
  let refreshDoneTimer: ReturnType<typeof setTimeout> | null = null
```

- [ ] **Step 3: 個別更新関数を追加**

`handleSettingsClose` 関数の後に追加:

```typescript
  async function handleRefreshCurrent() {
    if (!selectedTableId || refreshingSingle) return
    refreshingSingle = true
    refreshDoneMessage = ''
    if (refreshDoneTimer) { clearTimeout(refreshDoneTimer); refreshDoneTimer = null }
    try {
      const result = await RefreshDifficultyTable(selectedTableId)
      if (result.success) {
        refreshDoneMessage = `${result.entryCount}件更新`
      } else {
        refreshDoneMessage = result.error || '更新失敗'
      }
      await loadEntries(selectedTableId!)
      // テーブル一覧も更新（entryCountやfetchedAtが変わるため）
      tables = (await ListDifficultyTables()) || []
    } catch (e: any) {
      refreshDoneMessage = e?.message || '更新失敗'
    } finally {
      refreshingSingle = false
      refreshDoneTimer = setTimeout(() => { refreshDoneMessage = '' }, 3000)
    }
  }
```

- [ ] **Step 4: テンプレートにリフレッシュボタンを追加**

`<select>` の閉じタグ `</select>` と `</div>` の間にボタンを挿入。

既存:
```svelte
        <select
          class="select select-bordered select-sm"
          value={selectedTableId}
          on:change={handleTableChange}
          on:click|stopPropagation
        >
          {#each tables as t}
            <option value={t.id}>{t.symbol} / {t.name} ({t.entryCount})</option>
          {/each}
        </select>
      </div>
```

変更後:
```svelte
        <select
          class="select select-bordered select-sm"
          value={selectedTableId}
          on:change={handleTableChange}
          on:click|stopPropagation
        >
          {#each tables as t}
            <option value={t.id}>{t.symbol} / {t.name} ({t.entryCount})</option>
          {/each}
        </select>
        <button
          class="btn btn-ghost btn-xs"
          on:click|stopPropagation={handleRefreshCurrent}
          disabled={refreshingSingle}
          title="この難易度表を更新"
        >
          <Icon name="arrowPath" cls="w-4 h-4 {refreshingSingle ? 'animate-spin' : ''}" />
        </button>
        {#if refreshDoneMessage}
          <span class="text-xs text-success">{refreshDoneMessage}</span>
        {/if}
      </div>
```

- [ ] **Step 5: 動作確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails dev`

確認事項:
1. テーブルセレクタの右に回転矢印アイコンが表示される
2. クリックするとアイコンが回転し、完了後に「○件更新」が3秒間表示される
3. 更新後にエントリ一覧が自動リロードされる
4. 更新中はボタンがdisabledでクリック不可
5. エラー時はエラーメッセージが表示される

- [ ] **Step 6: コミット**

```bash
git add frontend/src/views/DifficultyTableView.svelte
git commit -m "feat: 難易度表メインUIに個別更新ボタンを追加（arrowPathアイコン）"
```

---

### Task 5: 最終確認

- [ ] **Step 1: go build 確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 2: wails dev で統合動作確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails dev`

全体の動作確認:
1. メインUIの個別更新ボタン — アイコン回転、完了メッセージ、エントリリロード
2. 設定ダイアログの一括更新 — 進捗表示、結果一覧、停止ボタン
3. 一括更新中にダイアログを閉じて再オープン — 進捗が復元される
4. 一括更新中に停止 — 処理が中断され、完了した分の結果が表示される
