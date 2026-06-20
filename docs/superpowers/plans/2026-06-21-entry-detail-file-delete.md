# 難易度表詳細 ファイル削除機能 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 難易度表エントリ詳細の重複時ファイルパス一覧から、各BMSファイルをゴミ箱アイコン＋確認ダイアログで完全削除できるようにする。

**Architecture:** Goハンドラ `ChartHandler` に `os.Remove` ベースの `DeleteChartFile` を追加し、Wailsバインディング経由でフロントへ公開。`EntryDetail.svelte` のパス行に `trash` アイコンボタンを追加し、`DuplicateDetail.svelte` と同じ `<dialog class="modal">` 確認パターンと `AlertModal` を用いて削除を実行する。削除後のUI更新は行わない。

**Tech Stack:** Go / Wails v2 / Svelte / daisyUI(Tailwind)

設計書: `docs/superpowers/specs/2026-06-21-entry-detail-file-delete-design.md`

---

### Task 1: バックエンドに DeleteChartFile を追加

**Files:**
- Modify: `internal/app/chart_handler.go`
- Test: `internal/app/chart_handler_test.go` (Create)

- [ ] **Step 1: 失敗するテストを書く**

`internal/app/chart_handler_test.go` を新規作成:

```go
package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeleteChartFile_RemovesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.bms")
	if err := os.WriteFile(path, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("setup write failed: %v", err)
	}

	h := &ChartHandler{}
	if err := h.DeleteChartFile(path); err != nil {
		t.Fatalf("DeleteChartFile returned error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file still exists after delete: stat err=%v", err)
	}
}

func TestDeleteChartFile_NoErrorWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.bms")

	h := &ChartHandler{}
	if err := h.DeleteChartFile(path); err != nil {
		t.Fatalf("DeleteChartFile on missing file returned error: %v", err)
	}
}
```

- [ ] **Step 2: テストを実行して失敗を確認**

Run: `go test ./internal/app/ -run TestDeleteChartFile -v`
Expected: コンパイルエラー（`h.DeleteChartFile undefined`）で FAIL

- [ ] **Step 3: 最小実装を書く**

`internal/app/chart_handler.go` の import に `"os"` を追加:

```go
import (
	"context"
	"os"

	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
	"github.com/meta-BE/bms-elsa/internal/app/dto"
)
```

ファイル末尾（`ListChartPathsByMD5` の後）にメソッドを追加:

```go
// DeleteChartFile は指定パスのBMSファイルを削除する。
// ファイルが既に存在しない場合は何もせず成功扱いとする。
func (h *ChartHandler) DeleteChartFile(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
```

- [ ] **Step 4: テストを実行して成功を確認**

Run: `go test ./internal/app/ -run TestDeleteChartFile -v`
Expected: PASS（2件）

- [ ] **Step 5: コンパイル確認**

Run: `go build ./...`
Expected: エラーなし

- [ ] **Step 6: コミット**

```bash
git add internal/app/chart_handler.go internal/app/chart_handler_test.go
git commit -m "feat(app): ChartHandlerにDeleteChartFileを追加"
```

---

### Task 2: Wailsバインディングを再生成

**Files:**
- Modify: `frontend/wailsjs/go/app/ChartHandler.js`
- Modify: `frontend/wailsjs/go/app/ChartHandler.d.ts`

- [ ] **Step 1: バインディングを再生成**

Run: `wails generate module`
Expected: エラーなく完了し、`frontend/wailsjs/go/app/ChartHandler.js` / `.d.ts` に `DeleteChartFile` が追加される

- [ ] **Step 2: 生成結果を確認**

Run: `grep -n "DeleteChartFile" frontend/wailsjs/go/app/ChartHandler.js frontend/wailsjs/go/app/ChartHandler.d.ts`
Expected: `.js` に以下、`.d.ts` に `export function DeleteChartFile(arg1:string):Promise<void>;` が存在する

```js
export function DeleteChartFile(arg1) {
  return window['go']['app']['ChartHandler']['DeleteChartFile'](arg1);
}
```

備考: `wails generate module` が使えない場合は上記 `.js` のエクスポートと、`.d.ts` への `export function DeleteChartFile(arg1:string):Promise<void>;` を手動で追記する（既存エクスポートのアルファベット順／既存並びに合わせる）。

- [ ] **Step 3: コミット**

```bash
git add frontend/wailsjs/go/app/ChartHandler.js frontend/wailsjs/go/app/ChartHandler.d.ts
git commit -m "chore(bindings): DeleteChartFileのWailsバインディングを生成"
```

---

### Task 3: EntryDetail にゴミ箱ボタン・確認ダイアログ・削除処理を追加

**Files:**
- Modify: `frontend/src/views/EntryDetail.svelte`

- [ ] **Step 1: import と状態を追加**

`frontend/src/views/EntryDetail.svelte` の3行目の import を変更し、`DeleteChartFile` を追加:

```ts
  import { GetChartDetailByMD5, GetChartMetaByMD5, ListChartPathsByMD5, DeleteChartFile } from '../../wailsjs/go/app/ChartHandler'
```

`Icon` の import 行（17行目付近）の直後に `AlertModal` の import を追加:

```ts
  import AlertModal from '../components/AlertModal.svelte'
```

`let dupPaths: string[] = []`（30行目付近）の直後に削除用の状態を追加:

```ts
  // ファイル削除
  let deleteDialog: HTMLDialogElement
  let mouseDownOnBackdrop = false
  let pendingDeletePath = ''
  let alertModal: AlertModal
```

- [ ] **Step 2: 削除処理関数を追加**

`unlinkBMSSearch` 関数（88行目付近、`</script>` の直前）の後に3関数を追加:

```ts
  function requestDelete(path: string) {
    pendingDeletePath = path
    deleteDialog.showModal()
  }

  async function executeDelete() {
    const path = pendingDeletePath
    deleteDialog.close()
    try {
      await DeleteChartFile(path)
    } catch (err) {
      alertModal.open(String(err))
    } finally {
      pendingDeletePath = ''
    }
  }

  function cancelDelete() {
    deleteDialog.close()
    pendingDeletePath = ''
  }
```

- [ ] **Step 3: パス行にゴミ箱ボタンを追加**

ファイルパス一覧の各行（150-155行目付近）を以下に置き換える:

置換前:
```svelte
          {#each dupPaths as p}
            <div class="text-xs text-base-content/70 break-all flex items-center gap-1">
              <OpenFolderButton path={p} size="xs" title="フォルダを開く" />
              <span>{p}</span>
            </div>
          {/each}
```

置換後:
```svelte
          {#each dupPaths as p}
            <div class="text-xs text-base-content/70 flex items-center gap-1">
              <OpenFolderButton path={p} size="xs" title="フォルダを開く" />
              <span class="flex-1 break-all">{p}</span>
              <button
                class="btn btn-ghost btn-xs shrink-0"
                title="ファイルを削除"
                on:click={() => requestDelete(p)}
              >
                <Icon name="trash" cls="h-3 w-3" />
              </button>
            </div>
          {/each}
```

- [ ] **Step 4: 確認ダイアログと AlertModal を追加**

ファイル末尾の `{/if}`（180行目付近、`{:else if entryData}` ブロックを閉じる行）の直後に追加:

```svelte

<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<dialog bind:this={deleteDialog} class="modal"
  on:mousedown|self={() => mouseDownOnBackdrop = true}
  on:click|self={() => { if (mouseDownOnBackdrop) cancelDelete(); mouseDownOnBackdrop = false }}>
  <div class="modal-box max-w-2xl">
    <h3 class="text-lg font-bold mb-4">ファイル削除の確認</h3>
    <div class="space-y-2 text-sm">
      <p>このファイルを完全に削除します。元に戻せません。</p>
      <div class="bg-base-200 rounded p-3 break-all">{pendingDeletePath}</div>
    </div>
    <div class="modal-action">
      <button class="btn" on:click={cancelDelete}>キャンセル</button>
      <button class="btn btn-error" on:click={executeDelete}>削除</button>
    </div>
  </div>
</dialog>

<AlertModal bind:this={alertModal} />
```

- [ ] **Step 5: フロントエンドのビルド/型チェック**

Run: `cd frontend && npm run build`
Expected: ビルド成功（型エラー・Svelteコンパイルエラーなし）

- [ ] **Step 6: コミット**

```bash
git add frontend/src/views/EntryDetail.svelte
git commit -m "feat(frontend): 難易度表詳細のパス一覧にファイル削除機能を追加"
```

---

### Task 4: マニュアルを更新

**Files:**
- Modify: `docs/manual.md`

- [ ] **Step 1: 該当セクションを特定**

Run: `grep -n "重複時\|ファイルパス一覧\|難易度表" docs/manual.md`
Expected: 難易度表詳細の重複時パス一覧に関する記述（直前コミット 468cd1b で追記）が見つかる

- [ ] **Step 2: 削除機能の説明を追記**

Step 1 で見つけたファイルパス一覧の説明箇所に、各パス行のゴミ箱アイコンから対象BMSファイルを完全削除できること（確認ダイアログあり・元に戻せない・削除後に一覧は自動更新されない）を1〜2文で追記する。周辺の記述スタイル・見出しレベルに合わせること。

- [ ] **Step 3: コミット**

```bash
git add docs/manual.md
git commit -m "docs(manual): 難易度表詳細のファイル削除機能を追記"
```

---

## Self-Review メモ

- **Spec coverage:** ゴミ箱アイコン(Task3 S3) / 確認(Task3 S4) / 単一ファイル削除・os.Remove(Task1) / 不在時no-op(Task1 S1 2件目テスト) / UI更新なし(Task3 S2 executeDeleteは再読込しない) / エラー時AlertModal(Task3 S2,S4) / マニュアル(Task4) — すべて対応。
- **Placeholder scan:** コードステップは全て実コードを記載。Task4 S2のみ既存マニュアル文体に合わせる記述指示（既存ドキュメント編集のため許容）。
- **Type consistency:** `DeleteChartFile`/`requestDelete`/`executeDelete`/`cancelDelete`/`pendingDeletePath`/`deleteDialog`/`alertModal`/`mouseDownOnBackdrop` を全タスクで一貫使用。`DuplicateDetail.svelte` の確認ダイアログパターンに準拠。
