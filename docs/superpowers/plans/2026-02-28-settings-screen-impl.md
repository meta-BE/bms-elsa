# 設定画面 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** config.jsonをGUIから編集できる設定画面を追加する

**Architecture:** App構造体に設定の読み書きメソッドを追加し、Wails Bindに登録。フロントエンドはDaisyUIモーダルで設定画面を実装。

**Tech Stack:** Go 1.24, Wails v2 (runtime.OpenFileDialog), Svelte 4, DaisyUI v5

---

### Task 1: バックエンドにGetConfig / SaveConfig / SelectFileを追加

**Files:**
- Modify: `app.go`
- Modify: `main.go`

**Step 1: app.goにメソッドを追加**

`app.go` に以下の3メソッドを追加する:

```go
// GetConfig は現在のconfig.jsonを読んで返す
func (a *App) GetConfig() Config {
	return loadConfig()
}

// SaveConfig はconfig.jsonに設定を書き込む
func (a *App) SaveConfig(cfg Config) error {
	path := filepath.Join(appDir(), "config.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("config write: %w", err)
	}
	return nil
}

// SelectFile はOSネイティブのファイル選択ダイアログを開き、選択されたパスを返す
func (a *App) SelectFile() (string, error) {
	return wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "songdata.db を選択",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "SQLite Database", Pattern: "*.db"},
		},
	})
}
```

importに追加:
```go
wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
```

**Step 2: main.goのBindにAppを追加**

`main.go` の Bind に `app` 自体を追加:

```go
Bind: []interface{}{
	app,
	app.SongHandler,
	app.IRHandler,
},
```

**Step 3: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: ビルド成功

**Step 4: コミット**

```bash
git add app.go main.go
git commit -m "設定の読み書きとファイル選択ダイアログのAPIを追加"
```

### Task 2: Settings.svelteを作成

**Files:**
- Create: `frontend/src/Settings.svelte`

**Step 1: Settings.svelteを作成**

```svelte
<script lang="ts">
  import { GetConfig, SaveConfig, SelectFile } from '../wailsjs/go/main/App'

  let dialog: HTMLDialogElement
  let songdataDBPath = ''
  let saved = false
  let error = ''

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

  async function handleBrowse() {
    try {
      const path = await SelectFile()
      if (path) {
        songdataDBPath = path
      }
    } catch (e) {
      // キャンセル時は何もしない
    }
  }

  async function handleSave() {
    error = ''
    try {
      await SaveConfig({ songdataDBPath })
      saved = true
    } catch (e: any) {
      error = e?.message || '保存に失敗しました'
    }
  }

  function handleClose() {
    dialog.close()
  }
</script>

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<dialog bind:this={dialog} class="modal" on:click|self={handleClose}>
  <div class="modal-box">
    <h3 class="text-lg font-bold mb-4">設定</h3>

    <div class="form-control w-full">
      <label class="label" for="songdata-path">
        <span class="label-text">songdata.db のパス</span>
      </label>
      <div class="flex gap-2">
        <input
          id="songdata-path"
          type="text"
          class="input input-bordered flex-1"
          bind:value={songdataDBPath}
          placeholder="/path/to/beatoraja/songdata.db"
        />
        <button class="btn btn-outline" on:click={handleBrowse}>参照</button>
      </div>
      <label class="label" for="songdata-path">
        <span class="label-text-alt text-base-content/50">
          未指定の場合は ~/.beatoraja/songdata.db → ~/beatoraja/songdata.db の順で自動検出
        </span>
      </label>
    </div>

    {#if saved}
      <div class="alert alert-success mt-4">
        <span>保存しました。設定を反映するにはアプリを再起動してください。</span>
      </div>
    {/if}

    {#if error}
      <div class="alert alert-error mt-4">
        <span>{error}</span>
      </div>
    {/if}

    <div class="modal-action">
      <button class="btn" on:click={handleClose}>閉じる</button>
      <button class="btn btn-primary" on:click={handleSave}>保存</button>
    </div>
  </div>
</dialog>
```

**Step 2: コミット**

```bash
git add frontend/src/Settings.svelte
git commit -m "設定画面モーダルコンポーネントを追加"
```

### Task 3: App.svelteにナビバー歯車アイコンを追加

**Files:**
- Modify: `frontend/src/App.svelte`

**Step 1: App.svelteを編集**

scriptセクションに追加:
```typescript
import Settings from './Settings.svelte'
let settingsComponent: Settings
```

ナビバーのdiv（`<div class="flex-1">` の兄弟要素として）に歯車ボタンを追加:
```svelte
<div class="navbar bg-base-200 px-4 shrink-0">
  <div class="flex-1">
    <span class="text-xl font-bold">BMS ELSA</span>
  </div>
  <button class="btn btn-ghost btn-sm" on:click={() => settingsComponent.open()}>
    <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
    </svg>
  </button>
</div>
```

テンプレートの末尾（`</div>` の前）にSettingsコンポーネントを配置:
```svelte
<Settings bind:this={settingsComponent} />
```

**Step 2: Wails バインディング生成確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails dev` を起動して `frontend/wailsjs/go/main/App.js` が自動生成されることを確認。
`GetConfig`, `SaveConfig`, `SelectFile` が含まれていること。

**Step 3: コミット**

```bash
git add frontend/src/App.svelte
git commit -m "ナビバーに設定画面への歯車アイコンを追加"
```

### Task 4: 動作確認

**Step 1: wails devで動作確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails dev`

確認項目:
1. ナビバーに歯車アイコンが表示される
2. クリックで設定モーダルが開く
3. 現在のsongdataDBPathが表示される
4. 「参照」ボタンでファイル選択ダイアログが開く
5. ファイルを選択するとパスが入力欄に反映される
6. 「保存」ボタンで保存成功メッセージが表示される
7. config.jsonにパスが書き込まれている
8. 「閉じる」ボタンでモーダルが閉じる
