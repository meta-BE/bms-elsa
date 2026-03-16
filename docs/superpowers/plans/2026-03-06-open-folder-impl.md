# インストール先フォルダを開く 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 詳細パネルのヘッダーから譜面ファイルのインストール先フォルダをOSファイルマネージャで開けるようにする

**Architecture:** Go側に `OpenFolder(filePath)` メソッドを追加し、`filepath.Dir()` で親ディレクトリを算出後、OS別コマンド（open/explorer/xdg-open）で開く。フロントエンドは3つの詳細パネル（SongDetail, ChartDetail, EntryDetail）のヘッダーにフォルダアイコンボタンを追加し、Wailsバインディング経由で呼び出す。

**Tech Stack:** Go (Wails v2), Svelte, TypeScript

---

### Task 1: バックエンド - OpenFolder メソッド追加

**Files:**
- Modify: `app.go:108-118`（OpenURL の直後に追加）

**Step 1: `OpenFolder` メソッドを `app.go` に追加**

`OpenURL` メソッドの直後（118行目の後）に以下を追加:

```go
// OpenFolder は譜面ファイルのパスを受け取り、その親ディレクトリをOSのファイルマネージャで開く
func (a *App) OpenFolder(filePath string) error {
	dir := filepath.Dir(filePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("フォルダが存在しません: %s", dir)
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", dir).Start()
	case "windows":
		return exec.Command("explorer", dir).Start()
	default:
		return exec.Command("xdg-open", dir).Start()
	}
}
```

**Step 2: Wailsバインディングを生成**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails generate module`

期待: `frontend/wailsjs/go/main/App.js` に `OpenFolder` が追加される

**Step 3: コミット**

```bash
git add app.go frontend/wailsjs/
git commit -m "feat: フォルダを開くバックエンドAPI (OpenFolder) を追加"
```

---

### Task 2: SongDetail - フォルダを開くボタン追加

**Files:**
- Modify: `frontend/src/SongDetail.svelte`

**Step 1: import に OpenFolder を追加**

`SongDetail.svelte` の script タグ内、既存の import 群の後に追加:

```typescript
import { OpenFolder } from '../wailsjs/go/main/App'
```

**Step 2: ヘッダーにフォルダボタンを追加**

ヘッダーの `✕` ボタンの前（84行目付近）にフォルダボタンを挿入。
`detail.charts[0]?.path` が存在する場合のみ表示:

変更前:
```svelte
        <button
          class="btn btn-ghost btn-xs shrink-0 ml-2"
          on:click={() => dispatch('close')}
        >✕</button>
```

変更後:
```svelte
        <div class="flex items-center shrink-0 ml-2">
          {#if detail.charts[0]?.path}
            <button
              class="btn btn-ghost btn-xs"
              title="インストール先フォルダを開く"
              on:click={() => OpenFolder(detail.charts[0].path)}
            >📁</button>
          {/if}
          <button
            class="btn btn-ghost btn-xs"
            on:click={() => dispatch('close')}
          >✕</button>
        </div>
```

**Step 3: コミット**

```bash
git add frontend/src/SongDetail.svelte
git commit -m "feat: SongDetail ヘッダーにフォルダを開くボタンを追加"
```

---

### Task 3: ChartDetail - フォルダを開くボタン追加

**Files:**
- Modify: `frontend/src/ChartDetail.svelte`

**Step 1: import に OpenFolder を追加**

既存の import 群の後に追加:

```typescript
import { OpenFolder } from '../wailsjs/go/main/App'
```

**Step 2: ヘッダーにフォルダボタンを追加**

ヘッダーの `✕` ボタン（65-68行目）を変更:

変更前:
```svelte
        <button
          class="btn btn-ghost btn-xs shrink-0 ml-2"
          on:click={() => dispatch('close')}
        >✕</button>
```

変更後:
```svelte
        <div class="flex items-center shrink-0 ml-2">
          {#if chart?.path}
            <button
              class="btn btn-ghost btn-xs"
              title="インストール先フォルダを開く"
              on:click={() => OpenFolder(chart.path)}
            >📁</button>
          {/if}
          <button
            class="btn btn-ghost btn-xs"
            on:click={() => dispatch('close')}
          >✕</button>
        </div>
```

**Step 3: コミット**

```bash
git add frontend/src/ChartDetail.svelte
git commit -m "feat: ChartDetail ヘッダーにフォルダを開くボタンを追加"
```

---

### Task 4: EntryDetail - フォルダを開くボタン追加（導入済みのみ）

**Files:**
- Modify: `frontend/src/EntryDetail.svelte`

**Step 1: import に OpenFolder を追加**

既存の import 群の後に追加:

```typescript
import { OpenFolder } from '../wailsjs/go/main/App'
```

**Step 2: ヘッダーにフォルダボタンを追加（chart が存在する場合のみ）**

ヘッダーの `✕` ボタン（88-91行目）を変更:

変更前:
```svelte
        <button
          class="btn btn-ghost btn-xs shrink-0 ml-2"
          on:click={() => dispatch('close')}
        >✕</button>
```

変更後:
```svelte
        <div class="flex items-center shrink-0 ml-2">
          {#if chart?.path}
            <button
              class="btn btn-ghost btn-xs"
              title="インストール先フォルダを開く"
              on:click={() => OpenFolder(chart.path)}
            >📁</button>
          {/if}
          <button
            class="btn btn-ghost btn-xs"
            on:click={() => dispatch('close')}
          >✕</button>
        </div>
```

**Step 3: コミット**

```bash
git add frontend/src/EntryDetail.svelte
git commit -m "feat: EntryDetail ヘッダーにフォルダを開くボタンを追加（導入済みのみ）"
```

---

### Task 5: ビルド確認

**Step 1: フロントエンドビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && cd frontend && npm run build`

期待: エラーなし

**Step 2: Goビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`

期待: ビルド成功
