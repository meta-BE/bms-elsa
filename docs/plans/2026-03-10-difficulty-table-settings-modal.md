# 難易度表設定モーダル独立化 実装計画

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 難易度表の追加・削除・更新を Settings モーダルから独立させ、難易度表画面に専用モーダルを設置する。モーダル close 時に親画面のデータを同期する。

**Architecture:** `Settings.svelte` から難易度表管理セクションを抽出して `DifficultyTableSettings.svelte` を新設。`DifficultyTableView.svelte` にボタンとモーダルを組み込み、close イベントでテーブル一覧＋エントリを再取得する。

**Tech Stack:** Svelte, DaisyUI, Wails v2 bindings

---

## Chunk 1: DifficultyTableSettings モーダル新設と統合

### Task 1: DifficultyTableSettings.svelte を新設

**Files:**
- Create: `frontend/src/settings/DifficultyTableSettings.svelte`

- [ ] **Step 1: Settings.svelte の難易度表管理ロジック＋UIを移植した新コンポーネントを作成**

以下の内容で `frontend/src/settings/DifficultyTableSettings.svelte` を作成する。
既存の `RewriteRuleManager.svelte` のモーダルパターン（dialog + open/close + mouseDownOnBackdrop）に準拠。

```svelte
<script lang="ts">
  import { ListDifficultyTables, AddDifficultyTable, RemoveDifficultyTable, RefreshAllDifficultyTables } from '../../wailsjs/go/app/DifficultyTableHandler'

  let dialog: HTMLDialogElement
  let mouseDownOnBackdrop = false
  let tables: any[] = []
  let newTableURL = ''
  let addError = ''
  let refreshResults: any[] | null = null
  let refreshing = false
  let adding = false

  export async function open() {
    addError = ''
    newTableURL = ''
    refreshResults = null
    await loadTables()
    dialog.showModal()
  }

  async function loadTables() {
    try {
      tables = await ListDifficultyTables() || []
    } catch (e) {
      tables = []
    }
  }

  async function handleAddTable() {
    if (!newTableURL.trim()) return
    addError = ''
    adding = true
    try {
      await AddDifficultyTable(newTableURL.trim())
      newTableURL = ''
      await loadTables()
    } catch (e: any) {
      addError = e?.message || '追加に失敗しました'
    } finally {
      adding = false
    }
  }

  async function handleRemoveTable(id: number) {
    await RemoveDifficultyTable(id)
    await loadTables()
  }

  async function handleRefreshAll() {
    refreshing = true
    refreshResults = null
    try {
      refreshResults = await RefreshAllDifficultyTables()
      await loadTables()
    } catch (e: any) {
      refreshResults = [{ tableName: '', success: false, error: e?.message || '更新に失敗しました' }]
    } finally {
      refreshing = false
    }
  }

  function handleClose() {
    dialog.close()
  }
</script>

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<dialog bind:this={dialog} class="modal"
  on:mousedown|self={() => mouseDownOnBackdrop = true}
  on:click|self={() => { if (mouseDownOnBackdrop) dialog.close(); mouseDownOnBackdrop = false }}>
  <div class="modal-box max-w-2xl">
    <h3 class="text-lg font-bold mb-4">難易度表設定</h3>

    {#if tables.length > 0}
      <div class="overflow-x-auto">
        <table class="table table-xs">
          <thead>
            <tr>
              <th>名前</th>
              <th>記号</th>
              <th>譜面数</th>
              <th>最終取得</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {#each tables as t}
              <tr>
                <td>{t.name}</td>
                <td>{t.symbol}</td>
                <td>{t.entryCount}</td>
                <td class="text-xs text-base-content/50">{t.fetchedAt || '未取得'}</td>
                <td>
                  <button class="btn btn-ghost btn-xs text-error" on:click={() => handleRemoveTable(t.id)}>削除</button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}
      <p class="text-sm text-base-content/50">難易度表が登録されていません</p>
    {/if}

    <div class="flex gap-2 mt-2">
      <input
        type="text"
        class="input input-bordered input-sm flex-1"
        bind:value={newTableURL}
        placeholder="https://stellabms.xyz/st/table.html"
        on:keydown={(e) => e.key === 'Enter' && handleAddTable()}
      />
      <button class="btn btn-sm btn-outline" on:click={handleAddTable} disabled={adding}>
        {adding ? '追加中...' : '追加'}
      </button>
    </div>
    {#if addError}
      <div class="alert alert-error mt-2 py-1 text-sm">{addError}</div>
    {/if}

    {#if tables.length > 0}
      <button class="btn btn-sm btn-outline mt-2" on:click={handleRefreshAll} disabled={refreshing}>
        {refreshing ? '更新中...' : '全て更新'}
      </button>
    {/if}

    {#if refreshResults}
      <div class="mt-2 text-sm space-y-1">
        {#each refreshResults as r}
          <div class="flex items-center gap-2">
            <span class={r.success ? 'text-success' : 'text-error'}>{r.success ? '✓' : '✗'}</span>
            <span>{r.tableName}</span>
            {#if r.success}
              <span class="text-base-content/50">{r.entryCount}件</span>
            {:else}
              <span class="text-error">{r.error}</span>
            {/if}
          </div>
        {/each}
      </div>
    {/if}

    <div class="modal-action">
      <button class="btn" on:click={handleClose}>閉じる</button>
    </div>
  </div>
</dialog>
```

---

### Task 2: DifficultyTableView にボタンとモーダルを組み込む

**Files:**
- Modify: `frontend/src/views/DifficultyTableView.svelte`

- [ ] **Step 1: import を追加**

`DifficultyTableView.svelte` の script 冒頭に以下を追加：

```typescript
import DifficultyTableSettings from '../settings/DifficultyTableSettings.svelte'
```

- [ ] **Step 2: モーダル参照用の変数を追加**

`let searchText = ''` の下あたりに追加：

```typescript
let dtSettingsComponent: DifficultyTableSettings
```

- [ ] **Step 3: close 時のリフレッシュ関数を追加**

`handleRowClick` 関数の後に追加：

```typescript
async function handleSettingsClose() {
  const prevId = selectedTableId
  tables = (await ListDifficultyTables()) || []
  if (tables.length === 0) {
    selectedTableId = null
    entries = []
    applyFilter()
    return
  }
  // 選択中テーブルがまだ存在するか確認
  const still = tables.find(t => t.id === prevId)
  if (still) {
    selectedTableId = prevId
  } else {
    selectedTableId = tables[0].id
  }
  await loadEntries(selectedTableId!)
}
```

- [ ] **Step 4: ヘッダーのIR取得ボタンの右に「難易度表設定」ボタンを追加**

`DifficultyTableView.svelte` のテンプレートで、BulkFetchButton と SearchInput の間にボタンを追加する。

変更前（212-218行目付近）：

```svelte
      <div class="flex items-center gap-2">
        <BulkFetchButton
          startFn={() => selectedTableId ? StartDifficultyTableBulkFetch(selectedTableId) : Promise.resolve()}
          stopFn={StopBulkFetch}
          on:done={() => selectedTableId && loadEntries(selectedTableId)}
        />
        <SearchInput bind:value={searchText} on:input={applyFilter} on:clear={applyFilter} />
      </div>
```

変更後：

```svelte
      <div class="flex items-center gap-2">
        <BulkFetchButton
          startFn={() => selectedTableId ? StartDifficultyTableBulkFetch(selectedTableId) : Promise.resolve()}
          stopFn={StopBulkFetch}
          on:done={() => selectedTableId && loadEntries(selectedTableId)}
        />
        <button class="btn btn-sm btn-ghost" on:click|stopPropagation={() => dtSettingsComponent.open()}>
          難易度表設定
        </button>
        <SearchInput bind:value={searchText} on:input={applyFilter} on:clear={applyFilter} />
      </div>
```

- [ ] **Step 5: テーブル0件時のメッセージを更新し、設定ボタンを追加**

変更前（195-196行目）：

```svelte
    {:else if tables.length === 0}
      <span class="text-sm text-base-content/50">Settings画面から難易度表を追加してください</span>
```

変更後：

```svelte
    {:else if tables.length === 0}
      <button class="btn btn-sm btn-ghost" on:click|stopPropagation={() => dtSettingsComponent.open()}>
        難易度表を追加
      </button>
```

- [ ] **Step 6: テンプレート末尾にモーダルコンポーネントを配置**

`</div>` の最後（ルート要素閉じタグ）の後に追加：

```svelte
<DifficultyTableSettings bind:this={dtSettingsComponent} on:close={handleSettingsClose} />
```

---

### Task 3: DifficultyTableSettings に close イベントの dispatch を追加

**Files:**
- Modify: `frontend/src/settings/DifficultyTableSettings.svelte`

- [ ] **Step 1: createEventDispatcher を追加**

script の import に追加：

```typescript
import { createEventDispatcher } from 'svelte'
```

変数宣言の先頭に追加：

```typescript
const dispatch = createEventDispatcher()
```

- [ ] **Step 2: handleClose で close イベントを dispatch**

```typescript
function handleClose() {
  dialog.close()
  dispatch('close')
}
```

- [ ] **Step 3: バックドロップクリック時も dispatch**

dialog の on:click|self を更新：

変更前：

```svelte
on:click|self={() => { if (mouseDownOnBackdrop) dialog.close(); mouseDownOnBackdrop = false }}
```

変更後：

```svelte
on:click|self={() => { if (mouseDownOnBackdrop) { dialog.close(); dispatch('close') } mouseDownOnBackdrop = false }}
```

---

### Task 4: Settings.svelte から難易度表セクションを削除

**Files:**
- Modify: `frontend/src/settings/Settings.svelte`

- [ ] **Step 1: 不要な import を削除**

変更前：

```typescript
import { GetConfig, SaveConfig, SelectFile } from '../../wailsjs/go/main/App'
import { ListDifficultyTables, AddDifficultyTable, RemoveDifficultyTable, RefreshAllDifficultyTables } from '../../wailsjs/go/app/DifficultyTableHandler'
```

変更後：

```typescript
import { GetConfig, SaveConfig, SelectFile } from '../../wailsjs/go/main/App'
```

- [ ] **Step 2: 難易度表関連の変数を削除**

以下の変数を削除：

```typescript
let tables: any[] = []
let newTableURL = ''
let addError = ''
let refreshResults: any[] | null = null
let refreshing = false
let adding = false
```

- [ ] **Step 3: open() から loadTables() 呼び出しを削除**

変更前：

```typescript
export async function open() {
  saved = false
  error = ''
  refreshResults = null
  try {
    const cfg = await GetConfig()
    songdataDBPath = cfg.songdataDBPath || ''
  } catch (e) {
    songdataDBPath = ''
  }
  await loadTables()
  dialog.showModal()
}
```

変更後：

```typescript
export async function open() {
  saved = false
  error = ''
  try {
    const cfg = await GetConfig()
    songdataDBPath = cfg.songdataDBPath || ''
  } catch (e) {
    songdataDBPath = ''
  }
  dialog.showModal()
}
```

- [ ] **Step 4: 難易度表関連の関数を全て削除**

以下の関数を削除：
- `loadTables()`
- `handleAddTable()`
- `handleRemoveTable()`
- `handleRefreshAll()`

- [ ] **Step 5: テンプレートから難易度表セクションを削除**

divider（`<div class="divider"></div>`）から `{/if}` （refreshResults の閉じタグ、203行目）までを削除。
具体的には Settings.svelte の133行目〜203行目を削除する。

---

### Task 5: 動作確認とコミット

- [ ] **Step 1: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && cd frontend && npm run check`
Expected: エラーなし

- [ ] **Step 2: コミット**

```bash
git add frontend/src/settings/DifficultyTableSettings.svelte frontend/src/views/DifficultyTableView.svelte frontend/src/settings/Settings.svelte
git commit -m "refactor: 難易度表設定をSettingsから独立モーダルに分離"
```
