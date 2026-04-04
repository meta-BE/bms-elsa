# 起動時バックグラウンドタスク自動実行 実装計画

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** アプリ起動時にMinHashスキャンと難易度表一括更新を自動実行し、進捗を基本設定モーダルで確認できるようにする

**Architecture:** `app.startup()` に2行追加で自動起動。フロントエンドはChartListViewからMinHash UIを削除し、Settings.svelteにWailsイベント購読による進捗セクションを追加

**Tech Stack:** Go (Wails v2), Svelte 4, TypeScript, DaisyUI 5

---

## ファイル構成

| 操作 | ファイル | 責務 |
|------|---------|------|
| 修正 | `app.go` | startup()にバックグラウンドタスク自動起動を追加 |
| 修正 | `frontend/src/views/ChartListView.svelte` | MinHashスキャン関連のUI・ロジックを削除 |
| 修正 | `frontend/src/settings/Settings.svelte` | バックグラウンドタスク進捗セクションを追加 |

---

### Task 1: バックエンド — startup()に自動起動を追加

**Files:**
- Modify: `app.go:125-136`

- [ ] **Step 1: startup()にバックグラウンドタスク起動を追加**

`app.go` の `startup()` メソッド末尾に2行追加する:

```go
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.SongHandler.SetContext(ctx)
	a.IRHandler.SetContext(ctx)
	a.EventHandler.SetContext(ctx)
	a.RewriteHandler.SetContext(ctx)
	a.ChartHandler.SetContext(ctx)
	a.DifficultyTableHandler.SetContext(ctx)
	a.ScanHandler.SetContext(ctx)
	a.DiffImportHandler.SetContext(ctx)
	a.DuplicateHandler.SetContext(ctx)

	// バックグラウンドタスクを並列起動
	a.ScanHandler.StartMinHashScan()
	a.DifficultyTableHandler.RefreshAllDifficultyTablesAsync()
}
```

- [ ] **Step 2: ビルド確認**

Run: `go build ./...`
Expected: 成功（エラーなし）

- [ ] **Step 3: コミット**

```bash
git add app.go
git commit -m "feat: startup時にMinHashスキャンと難易度表更新を自動起動"
```

---

### Task 2: ChartListViewからMinHashスキャンUIを削除

**Files:**
- Modify: `frontend/src/views/ChartListView.svelte`

- [ ] **Step 1: import文からMinHashスキャン関連を削除**

削除する行:
```typescript
import { StartMinHashScan, StopMinHashScan } from '../../wailsjs/go/app/ScanHandler'
```

`EventsOn` のimportは他で使っていないなら削除。ただし他のイベント購読がないか確認する。→ このファイルでは `EventsOn` はMinHashスキャン用のみなので削除。

- [ ] **Step 2: MinHashスキャン関連の状態変数を削除**

削除する変数（行36-40）:
```typescript
let scanRunning = false
let scanProgress = { current: 0, total: 0 }
let scanDoneMessage = ''
let scanDoneTimer: ReturnType<typeof setTimeout> | null = null
```

- [ ] **Step 3: MinHashスキャン関連の関数を削除**

削除する関数（行60-73）:
```typescript
function startMinHashScan() { ... }
function stopMinHashScan() { ... }
```

- [ ] **Step 4: Wailsイベント購読・解除を削除**

削除する変数宣言（行183-184）:
```typescript
let offScanProgress: (() => void) | null = null
let offScanDone: (() => void) | null = null
```

onMount内から `offScanProgress` と `offScanDone` の設定を削除（行187-203）。

onDestroy内から以下を削除（行213-215）:
```typescript
offScanProgress?.()
offScanDone?.()
if (scanDoneTimer) clearTimeout(scanDoneTimer)
```

onDestroyの中身が空になった場合、onDestroy自体と `onDestroy` のimportも削除する。

- [ ] **Step 5: テンプレートからMinHashスキャンUIを削除**

ヘッダー部分（行260-269）の以下のブロックを削除:

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

- [ ] **Step 6: ビルド確認**

Run: `cd frontend && npm run build`
Expected: 成功（エラーなし）

- [ ] **Step 7: コミット**

```bash
git add frontend/src/views/ChartListView.svelte
git commit -m "feat: ChartListViewからMinHashスキャンUIを削除（起動時自動実行に移行）"
```

---

### Task 3: 基本設定モーダルにバックグラウンドタスク進捗セクションを追加

**Files:**
- Modify: `frontend/src/settings/Settings.svelte`

- [ ] **Step 1: import文を追加**

`Settings.svelte` の `<script>` ブロック先頭にimportを追加:

```typescript
import { onMount, onDestroy } from 'svelte'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { IsMinHashScanRunning } from '../../wailsjs/go/app/ScanHandler'
import { IsRefreshing, RefreshProgress } from '../../wailsjs/go/app/DifficultyTableHandler'
```

- [ ] **Step 2: 状態変数を追加**

既存の変数宣言の後に追加:

```typescript
// バックグラウンドタスクの状態
let scanState: 'running' | 'done' | 'error' = 'done'
let scanProgress = { current: 0, total: 0 }
let scanError = ''

let dtState: 'running' | 'done' | 'error' = 'done'
let dtProgress = { current: 0, total: 0 }
let dtError = ''
```

- [ ] **Step 3: open()にタスク状態取得を追加**

既存の `open()` 関数内、`dialog.showModal()` の前にバックグラウンドタスクの状態取得を追加:

```typescript
export async function open() {
  saved = false
  error = ''
  try {
    const cfg = await GetConfig()
    songdataDBPath = cfg.songdataDBPath || ''
    fileLog = cfg.fileLog || false
  } catch (e) {
    songdataDBPath = ''
  }

  // バックグラウンドタスクの現在状態を取得
  try {
    if (await IsMinHashScanRunning()) {
      scanState = 'running'
    }
  } catch {}
  try {
    if (await IsRefreshing()) {
      dtState = 'running'
      const p = await RefreshProgress()
      dtProgress = { current: p.current, total: p.total }
    }
  } catch {}

  dialog.showModal()
}
```

- [ ] **Step 4: Wailsイベント購読・解除を追加**

```typescript
let offScanProgress: (() => void) | null = null
let offScanDone: (() => void) | null = null
let offDtProgress: (() => void) | null = null
let offDtDone: (() => void) | null = null

onMount(() => {
  offScanProgress = EventsOn('scan:progress', (data: { current: number; total: number }) => {
    scanState = 'running'
    scanProgress = data
  })
  offScanDone = EventsOn('scan:done', (data: { total: number; computed: number; failed: number; cancelled: boolean }) => {
    if (data.failed > 0) {
      scanState = 'error'
      scanError = `${data.failed}件失敗`
    } else {
      scanState = 'done'
    }
    scanProgress = { current: data.total, total: data.total }
  })
  offDtProgress = EventsOn('dt:refresh-progress', (data: { current: number; total: number; tableName: string; success: boolean; error: string }) => {
    dtState = 'running'
    dtProgress = { current: data.current, total: data.total }
    if (data.error) {
      dtError = `${data.tableName}: ${data.error}`
    }
  })
  offDtDone = EventsOn('dt:refresh-done', (data: { results: Array<{ tableName: string; success: boolean; error: string }> }) => {
    const errors = data.results.filter(r => r.error)
    if (errors.length > 0) {
      dtState = 'error'
      dtError = errors.map(e => `${e.tableName}: ${e.error}`).join(', ')
    } else {
      dtState = 'done'
    }
  })
})

onDestroy(() => {
  offScanProgress?.()
  offScanDone?.()
  offDtProgress?.()
  offDtDone?.()
})
```

- [ ] **Step 5: テンプレートにバックグラウンドタスクセクションを追加**

`{#if error}` ブロックの直前（ファイルログのフォームコントロールの後）に追加:

```svelte
<!-- バックグラウンドタスク -->
<div class="divider text-xs text-base-content/50">バックグラウンドタスク</div>

<div class="space-y-3">
  <!-- MinHashスキャン -->
  <div>
    <div class="flex items-center justify-between text-sm mb-1">
      <span>MinHashスキャン</span>
      {#if scanState === 'running'}
        <span class="text-xs text-base-content/50">実行中...</span>
      {:else if scanState === 'error'}
        <span class="text-xs text-error">エラー</span>
      {:else}
        <span class="text-xs text-success">完了</span>
      {/if}
    </div>
    {#if scanState === 'running' && scanProgress.total > 0}
      <div class="flex items-center gap-2 text-xs">
        <progress class="progress progress-primary flex-1" value={scanProgress.current} max={scanProgress.total}></progress>
        <span class="text-base-content/50">{scanProgress.current.toLocaleString()}/{scanProgress.total.toLocaleString()}</span>
      </div>
    {/if}
    {#if scanState === 'error' && scanError}
      <p class="text-xs text-error mt-1">{scanError}</p>
    {/if}
  </div>

  <!-- 難易度表更新 -->
  <div>
    <div class="flex items-center justify-between text-sm mb-1">
      <span>難易度表更新</span>
      {#if dtState === 'running'}
        <span class="text-xs text-base-content/50">実行中...</span>
      {:else if dtState === 'error'}
        <span class="text-xs text-error">エラー</span>
      {:else}
        <span class="text-xs text-success">完了</span>
      {/if}
    </div>
    {#if dtState === 'running' && dtProgress.total > 0}
      <div class="flex items-center gap-2 text-xs">
        <progress class="progress progress-primary flex-1" value={dtProgress.current} max={dtProgress.total}></progress>
        <span class="text-base-content/50">{dtProgress.current}/{dtProgress.total}</span>
      </div>
    {/if}
    {#if dtState === 'error' && dtError}
      <p class="text-xs text-error mt-1">{dtError}</p>
    {/if}
  </div>
</div>
```

- [ ] **Step 6: ビルド確認**

Run: `cd frontend && npm run build`
Expected: 成功（エラーなし）

- [ ] **Step 7: コミット**

```bash
git add frontend/src/settings/Settings.svelte
git commit -m "feat: 基本設定モーダルにバックグラウンドタスク進捗表示を追加"
```

---

### Task 4: 全体ビルド確認

- [ ] **Step 1: Go + フロントエンド全体ビルド**

Run: `go build ./... && cd frontend && npm run build`
Expected: 両方成功

- [ ] **Step 2: 最終コミット（必要に応じて）**

ビルドエラーがあれば修正してコミット。
