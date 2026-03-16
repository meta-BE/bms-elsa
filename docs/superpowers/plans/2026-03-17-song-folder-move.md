# 楽曲フォルダ移動 実装計画

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 楽曲詳細パネルから楽曲フォルダを別ディレクトリに移動する機能を追加する

**Architecture:** ドメイン層に `MoveFolder` を追加し、ユースケース層でフォルダパス構築＋ログ、ハンドラー層で folderHash→パス解決＋DTO変換。フロントエンドは SongDetail に移動ボタン＋確認/結果ダイアログ、SongTable/App でセッション中のみ「移動済み」黄色表示。

**Tech Stack:** Go 1.24, Wails v2 (`runtime.OpenDirectoryDialog`), Svelte 4, DaisyUI 5

**Spec:** `docs/superpowers/specs/2026-03-17-song-folder-move-design.md`

---

## ファイル構成

| 操作 | パス | 責務 |
|------|------|------|
| Create | `internal/domain/fileutil/move_folder.go` | `MoveFolder` ドメイン関数 |
| Create | `internal/domain/fileutil/move_folder_test.go` | `MoveFolder` テスト |
| Create | `internal/usecase/move_song_folder.go` | `MoveSongFolderUseCase` |
| Modify | `internal/adapter/persistence/songdata_reader.go` | `parentDirOf` をエクスポート（`ParentDirOf`） |
| Modify | `internal/app/song_handler.go` | `MoveSongFolder` メソッド追加 |
| Modify | `internal/app/dto/dto.go` | `MoveSongFolderResultDTO` 追加 |
| Modify | `app.go` | `SelectDirectory` 追加、`MoveSongFolderUseCase` DI配線 |
| Modify | `frontend/src/utils/icons.ts` | フォルダ移動用アイコン追加 |
| Modify | `frontend/src/views/SongDetail.svelte` | 移動ボタン＋確認/結果ダイアログ |
| Modify | `frontend/src/views/SongTable.svelte` | 移動済み行の黄色表示 |
| Modify | `frontend/src/App.svelte` | `movedFolderHashes` 状態管理、`moved` イベント処理 |

---

## Chunk 1: バックエンド

### Task 1: `fileutil.MoveFolder` ドメイン関数

**Files:**
- Create: `internal/domain/fileutil/move_folder.go`
- Create: `internal/domain/fileutil/move_folder_test.go`

**コンテキスト:**
- 既存 `move.go` に `MoveFileToFolder`（ファイル単位移動）と `copyFile`（プライベートヘルパー）がある
- 既存 `merge.go` にバリデーション（`validateMergePaths`）、`filepath.WalkDir` による再帰処理のパターンがある
- `copyFile` は `move.go` で定義済みなので同パッケージから直接呼べる

- [ ] **Step 1: テストを書く**

`internal/domain/fileutil/move_folder_test.go`:

```go
package fileutil_test

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/meta-BE/bms-elsa/internal/domain/fileutil"
)

func TestMoveFolder_Rename(t *testing.T) {
	// 同一ファイルシステム内 → os.Rename で移動
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "a.bms"), []byte("aaa"), 0644)
	os.WriteFile(filepath.Join(srcDir, "b.wav"), []byte("bbb"), 0644)
	os.MkdirAll(filepath.Join(srcDir, "sub"), 0755)
	os.WriteFile(filepath.Join(srcDir, "sub", "c.bms"), []byte("ccc"), 0644)

	destDir := filepath.Join(tmpDir, "dest")

	count, err := fileutil.MoveFolder(srcDir, destDir)
	if err != nil {
		t.Fatalf("MoveFolder failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 files, got %d", count)
	}

	// 移動先にファイルが存在
	for _, rel := range []string{"a.bms", "b.wav", "sub/c.bms"} {
		if _, err := os.Stat(filepath.Join(destDir, rel)); err != nil {
			t.Errorf("file %s should exist at dest: %v", rel, err)
		}
	}

	// 移動元が消えている
	if _, err := os.Stat(srcDir); !os.IsNotExist(err) {
		t.Error("srcDir should not exist")
	}
}

func TestMoveFolder_DestExists(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	destDir := filepath.Join(tmpDir, "dest")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(destDir, 0755)

	_, err := fileutil.MoveFolder(srcDir, destDir)
	if err == nil {
		t.Fatal("should return error when destDir already exists")
	}
}

func TestMoveFolder_SrcNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := fileutil.MoveFolder(filepath.Join(tmpDir, "nonexistent"), filepath.Join(tmpDir, "dest"))
	if err == nil {
		t.Fatal("should return error when srcDir does not exist")
	}
}

func TestMoveFolder_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	os.MkdirAll(srcDir, 0755)
	destDir := filepath.Join(tmpDir, "dest")

	count, err := fileutil.MoveFolder(srcDir, destDir)
	if err != nil {
		t.Fatalf("MoveFolder failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 files, got %d", count)
	}
	if _, err := os.Stat(srcDir); !os.IsNotExist(err) {
		t.Error("srcDir should not exist after move")
	}
}
```

- [ ] **Step 2: テスト実行して失敗を確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/domain/fileutil/ -run TestMoveFolder -v`
Expected: FAIL（`MoveFolder` が未定義）

- [ ] **Step 3: 実装**

`internal/domain/fileutil/move_folder.go`:

```go
package fileutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// MoveFolder は srcDir を destDir に移動する。
// destDir は移動先の完全パス（存在してはならない）。
// 同一ファイルシステムなら os.Rename でアトミックに移動し、
// クロスファイルシステム（EXDEV）の場合は再帰コピー＋元ディレクトリ削除にフォールバックする。
// 戻り値は移動したファイル数。
func MoveFolder(srcDir, destDir string) (int, error) {
	if _, err := os.Stat(srcDir); err != nil {
		return 0, fmt.Errorf("移動元フォルダが存在しません: %s (%w)", srcDir, err)
	}
	if _, err := os.Stat(destDir); err == nil {
		return 0, fmt.Errorf("移動先に同名のフォルダが既に存在します: %s", destDir)
	}

	// ファイル数を事前カウント
	fileCount := 0
	filepath.WalkDir(srcDir, func(_ string, d fs.DirEntry, _ error) error {
		if d != nil && !d.IsDir() {
			fileCount++
		}
		return nil
	})

	// 同一FS → rename
	err := os.Rename(srcDir, destDir)
	if err == nil {
		return fileCount, nil
	}

	// EXDEV以外のエラーはそのまま返す
	var linkErr *os.LinkError
	if !errors.As(err, &linkErr) || !errors.Is(linkErr.Err, syscall.EXDEV) {
		return 0, fmt.Errorf("フォルダの移動に失敗: %w", err)
	}

	// クロスFS → コピー＋削除
	count, copyErr := copyDir(srcDir, destDir)
	if copyErr != nil {
		// コピー失敗時はクリーンアップ
		os.RemoveAll(destDir)
		return 0, fmt.Errorf("フォルダのコピーに失敗: %w", copyErr)
	}

	if err := os.RemoveAll(srcDir); err != nil {
		return count, fmt.Errorf("移動元の削除に失敗（コピーは完了）: %w", err)
	}

	return count, nil
}

// copyDir は srcDir 配下のファイル・サブディレクトリを destDir に再帰コピーする。
func copyDir(srcDir, destDir string) (int, error) {
	count := 0
	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(srcDir, path)
		destPath := filepath.Join(destDir, rel)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		// シンボリックリンクはスキップ
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		if err := copyFile(path, destPath); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}
```

- [ ] **Step 4: テスト実行して通ることを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/domain/fileutil/ -run TestMoveFolder -v`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
git add internal/domain/fileutil/move_folder.go internal/domain/fileutil/move_folder_test.go
git commit -m "feat: fileutil.MoveFolder を追加（rename優先、EXDEVフォールバック）"
```

---

### Task 2: `MoveSongFolderUseCase` ユースケース

**Files:**
- Create: `internal/usecase/move_song_folder.go`

**コンテキスト:**
- 既存パターン: `MergeFoldersUseCase`（`internal/usecase/merge_folders.go`）— `port.Logger` を受け取り、ドメイン関数を呼び出し、ログ出力、結果をDTO変換
- `GetSongDetailUseCase`（`internal/usecase/get_song_detail.go`）— `model.SongRepository` を受け取り、`GetSongByFolder` を呼び出す
- `parentDirOf` は `internal/adapter/persistence/songdata_reader.go` のプライベート関数（unexported）。ユースケース層からは使えないので、ハンドラー層でパス解決する

- [ ] **Step 1: 実装**

`internal/usecase/move_song_folder.go`:

```go
package usecase

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/meta-BE/bms-elsa/internal/domain/fileutil"
	"github.com/meta-BE/bms-elsa/internal/port"
)

// MoveSongFolderResult はフォルダ移動の結果
type MoveSongFolderResult struct {
	DestPath  string
	FileCount int
}

type MoveSongFolderUseCase struct {
	logger port.Logger
}

func NewMoveSongFolderUseCase(logger port.Logger) *MoveSongFolderUseCase {
	return &MoveSongFolderUseCase{logger: logger}
}

// Execute は srcFolderPath を destParentDir/フォルダ名 に移動する。
func (u *MoveSongFolderUseCase) Execute(_ context.Context, srcFolderPath, destParentDir string) (*MoveSongFolderResult, error) {
	folderName := filepath.Base(srcFolderPath)
	destPath := filepath.Join(destParentDir, folderName)

	count, err := fileutil.MoveFolder(srcFolderPath, destPath)
	if err != nil {
		return nil, err
	}

	u.logger.Log(fmt.Sprintf("MOVE %s → %s (%d files)", srcFolderPath, destPath, count))

	return &MoveSongFolderResult{
		DestPath:  destPath,
		FileCount: count,
	}, nil
}
```

- [ ] **Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./internal/usecase/`
Expected: 成功

- [ ] **Step 3: コミット**

```bash
git add internal/usecase/move_song_folder.go
git commit -m "feat: MoveSongFolderUseCase を追加"
```

---

### Task 3: `parentDirOf` エクスポート + DTO追加 + SongHandler.MoveSongFolder + App.SelectDirectory + DI配線

**Files:**
- Modify: `internal/adapter/persistence/songdata_reader.go` — `parentDirOf` → `ParentDirOf` にリネーム
- Modify: `internal/app/dto/dto.go` — `MoveSongFolderResultDTO` 追加
- Modify: `internal/app/song_handler.go` — `MoveSongFolder` メソッド、`moveSongFolder` フィールド追加
- Modify: `app.go` — `SelectDirectory` メソッド追加、`MoveSongFolderUseCase` DI配線

**コンテキスト:**
- `SongHandler` は `usecase.*UseCase` をフィールドに持ち、`NewSongHandler` で受け取る
- `app.go` の `Init()` で全 UseCase を生成し Handler に渡す。`startup()` で `SetContext` を呼ぶ
- `SongHandler.GetSongDetail` は `GetSongByFolder` で `*model.Song` を取得する。`Song.Charts[0].Path` が代表チャートのファイルパス
- フォルダパスの導出: `Song.Charts[0].Path` から親ディレクトリを得る必要がある
  - `filepath.Dir` はmacOS上で Windows `\` 区切りパスを正しく処理できない
  - `parentDirOf`（`songdata_reader.go`）は `strings.LastIndexAny(p, "/\\")` で両方に対応済み
  - これをエクスポートして `persistence.ParentDirOf` としてハンドラーから使う

- [ ] **Step 1: `parentDirOf` をエクスポート**

`internal/adapter/persistence/songdata_reader.go` の `parentDirOf` を `ParentDirOf` にリネーム:

変更前:
```go
func parentDirOf(p string) string {
```

変更後:
```go
// ParentDirOf はファイルパスから親ディレクトリを返す。
// songdata.dbのパスはWindows形式（\区切り）の場合があるため、両方のセパレータを考慮する。
func ParentDirOf(p string) string {
```

同ファイル内の全呼び出し箇所（`FindChartFoldersByTitle`, `FindChartFoldersByBodyURL`, `FindChartFoldersByArtist`）も `parentDirOf` → `ParentDirOf` に更新する。

- [ ] **Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build .`
Expected: 成功

- [ ] **Step 3: DTO追加**

`internal/app/dto/dto.go` の末尾に追加:

```go
type MoveSongFolderResultDTO struct {
	DestPath  string `json:"destPath"`
	FileCount int    `json:"fileCount"`
}
```

- [ ] **Step 2: SongHandler にフィールドとメソッドを追加**

`internal/app/song_handler.go`:

フィールド追加:
```go
type SongHandler struct {
	ctx           context.Context
	listSongs     *usecase.ListSongsUseCase
	getSongDetail *usecase.GetSongDetailUseCase
	updateMeta    *usecase.UpdateSongMetaUseCase
	moveSongFolder *usecase.MoveSongFolderUseCase  // 追加
}
```

コンストラクタ変更:
```go
func NewSongHandler(ls *usecase.ListSongsUseCase, gsd *usecase.GetSongDetailUseCase, um *usecase.UpdateSongMetaUseCase, msf *usecase.MoveSongFolderUseCase) *SongHandler {
	return &SongHandler{listSongs: ls, getSongDetail: gsd, updateMeta: um, moveSongFolder: msf}
}
```

メソッド追加:
```go
func (h *SongHandler) MoveSongFolder(folderHash, destParentDir string) (*dto.MoveSongFolderResultDTO, error) {
	song, err := h.getSongDetail.Execute(h.ctx, folderHash)
	if err != nil {
		return nil, err
	}
	if song == nil || len(song.Charts) == 0 {
		return nil, fmt.Errorf("楽曲が見つかりません: %s", folderHash)
	}

	// チャートのファイルパスからフォルダパスを導出（Windows\区切り対応）
	srcFolderPath := persistence.ParentDirOf(song.Charts[0].Path)

	result, err := h.moveSongFolder.Execute(h.ctx, srcFolderPath, destParentDir)
	if err != nil {
		return nil, err
	}

	return &dto.MoveSongFolderResultDTO{
		DestPath:  result.DestPath,
		FileCount: result.FileCount,
	}, nil
}
```

import追加: `"fmt"`, `"github.com/meta-BE/bms-elsa/internal/adapter/persistence"`

- [ ] **Step 3: `app.go` に `SelectDirectory` 追加**

`app.go` の `SelectFile()` の直後に追加:

```go
// SelectDirectory はOSネイティブのディレクトリ選択ダイアログを開き、選択されたパスを返す
func (a *App) SelectDirectory() (string, error) {
	return wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "移動先フォルダを選択",
	})
}
```

- [ ] **Step 4: `app.go` の DI配線を更新**

`app.go` の `Init()` 内で `MoveSongFolderUseCase` を生成し、`NewSongHandler` に渡す:

変更前（100行目付近）:
```go
a.SongHandler = internalapp.NewSongHandler(listSongs, getSongDetail, updateSongMeta)
```

変更後:
```go
moveSongFolder := usecase.NewMoveSongFolderUseCase(appLogger)
a.SongHandler = internalapp.NewSongHandler(listSongs, getSongDetail, updateSongMeta, moveSongFolder)
```

- [ ] **Step 5: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build .`
Expected: 成功

- [ ] **Step 6: コミット**

```bash
git add internal/adapter/persistence/songdata_reader.go internal/app/dto/dto.go internal/app/song_handler.go app.go
git commit -m "feat: SongHandler.MoveSongFolder と App.SelectDirectory を追加"
```

---

## Chunk 2: フロントエンド

### Task 4: SongDetail.svelte — 移動ボタン＋確認/結果ダイアログ

**Files:**
- Possibly modify: `frontend/src/utils/icons.ts` — アイコン追加
- Modify: `frontend/src/views/SongDetail.svelte`

**コンテキスト:**
- 現在のヘッダー部分（80-88行目）に `OpenFolderButton` と閉じるボタンがある
- `DuplicateDetail.svelte` で確認ダイアログのパターンがある: `<dialog>` + `showModal()` / `close()` + backdrop処理
- `AlertModal` コンポーネントがエラー表示に使える
- `MoveSongFolder` は `../../wailsjs/go/app/SongHandler` からインポートする
- `SelectDirectory` は `../../wailsjs/go/main/App` からインポートする
- イベント: `close` に加えて `moved` を追加（`{ folderHash: string }`）
- `moved` を props で受け取り、移動済みならボタンを無効化

- [ ] **Step 1: アイコン確認＋追加**

`frontend/src/utils/icons.ts` を確認し、`arrowRight` が定義されているか確認する。
なければ Heroicons の `arrow-right-start-on-rectangle` 等の適切なアイコンの SVG パスデータを `arrowRight` として追加する。
（既存アイコンのパターンに合わせて `icons` オブジェクトにエントリを追加）

- [ ] **Step 2: Wails バインディングを生成**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails generate module`
Expected: `frontend/wailsjs/` 配下に `MoveSongFolder`, `SelectDirectory` のバインディングが生成される

- [ ] **Step 3: SongDetail.svelte を更新**

`frontend/src/views/SongDetail.svelte` の変更:

**script部分**:

インポート追加:
```typescript
import { MoveSongFolder } from '../../wailsjs/go/app/SongHandler'
import { SelectDirectory } from '../../wailsjs/go/main/App'
import AlertModal from '../components/AlertModal.svelte'
```

dispatch の型変更:
```typescript
const dispatch = createEventDispatcher<{ close: void; moved: { folderHash: string } }>()
```

props 追加:
```typescript
export let moved = false
```

変数追加:
```typescript
let confirmDialog: HTMLDialogElement
let resultDialog: HTMLDialogElement
let alertModal: AlertModal
let moving = false
let moveDestParent = ''
let moveResult: { destPath: string; fileCount: number } | null = null
let mouseDownOnBackdrop = false
```

移動関連関数を追加:
```typescript
async function startMove() {
  if (!detail) return
  try {
    const dir = await SelectDirectory()
    if (!dir) return
    moveDestParent = dir
    confirmDialog.showModal()
  } catch (e) {
    // キャンセル
  }
}

function cancelMove() {
  confirmDialog.close()
}

async function executeMove() {
  if (!detail) return
  moving = true
  try {
    const result = await MoveSongFolder(detail.folderHash, moveDestParent)
    confirmDialog.close()
    moveResult = result
    resultDialog.showModal()
  } catch (err) {
    confirmDialog.close()
    alertModal.open(String(err))
  } finally {
    moving = false
  }
}

function closeResult() {
  resultDialog.close()
  dispatch('moved', { folderHash: folderHash })
}
```

**テンプレート部分**:

ヘッダーのボタン群（80-88行目付近）を変更。`OpenFolderButton` の後に移動ボタン追加:

```svelte
<div class="flex items-center shrink-0 ml-2">
  <OpenFolderButton path={detail.charts[0]?.path} title="インストール先フォルダを開く" />
  <button
    class="btn btn-ghost btn-xs"
    title="フォルダ移動"
    on:click|stopPropagation={startMove}
    disabled={moved}
  >
    <Icon name="arrowRight" cls="h-4 w-4" />
  </button>
  <button
    class="btn btn-ghost btn-xs"
    on:click={() => dispatch('close')}
  >
    <Icon name="close" cls="h-4 w-4" />
  </button>
</div>
```

ファイル末尾（`{/if}` の後）に確認ダイアログ・結果ダイアログ・AlertModal を追加:

```svelte
<!-- 移動確認ダイアログ -->
<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<dialog bind:this={confirmDialog} class="modal"
  on:mousedown|self={() => mouseDownOnBackdrop = true}
  on:click|self={() => { if (mouseDownOnBackdrop) cancelMove(); mouseDownOnBackdrop = false }}>
  <div class="modal-box max-w-2xl">
    <h3 class="text-lg font-bold mb-4">フォルダ移動の確認</h3>
    <div class="space-y-2 text-sm">
      <p>楽曲フォルダを移動します。移動元フォルダは削除されます。</p>
      <div class="bg-base-200 rounded p-2 space-y-1">
        <div><span class="text-base-content/50">楽曲:</span> <span class="break-all">{detail?.title}</span></div>
        <div><span class="text-base-content/50">移動先:</span> <span class="break-all">{moveDestParent}</span></div>
      </div>
    </div>
    <div class="modal-action">
      <button class="btn" on:click={cancelMove}>キャンセル</button>
      <button class="btn btn-warning" on:click={executeMove} disabled={moving}>
        {moving ? '移動中...' : '移動実行'}
      </button>
    </div>
  </div>
</dialog>

<!-- 移動結果ダイアログ -->
<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-noninteractive-element-interactions -->
<dialog bind:this={resultDialog} class="modal"
  on:mousedown|self={() => mouseDownOnBackdrop = true}
  on:click|self={() => { if (mouseDownOnBackdrop) closeResult(); mouseDownOnBackdrop = false }}>
  <div class="modal-box max-w-2xl">
    <h3 class="text-lg font-bold mb-4">移動完了</h3>
    {#if moveResult}
      <div class="space-y-2 text-sm">
        <div><span class="text-base-content/50">移動先:</span> <span class="break-all">{moveResult.destPath}</span></div>
        <div><span class="text-base-content/50">ファイル数:</span> {moveResult.fileCount}</div>
      </div>
    {/if}
    <div class="modal-action">
      <button class="btn" on:click={closeResult}>OK</button>
    </div>
  </div>
</dialog>

<AlertModal bind:this={alertModal} />
```

- [ ] **Step 4: フロントエンドビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run build`
Expected: 成功

- [ ] **Step 5: コミット**

```bash
git add frontend/src/utils/icons.ts frontend/src/views/SongDetail.svelte
git commit -m "feat: 楽曲詳細に移動ボタン＋確認/結果ダイアログを追加"
```

---

### Task 5: App.svelte + SongTable.svelte — 移動済み状態管理＋表示

**Files:**
- Modify: `frontend/src/App.svelte`
- Modify: `frontend/src/views/SongTable.svelte`

**コンテキスト:**
- `App.svelte`:
  - 既存パターン: `handleMemberMerged` で `duplicateViewRef?.removeMember()` → UI楽観的更新
  - `SongDetail` は 200行目: `<SongDetail folderHash={selectedFolderHash} on:close={handleClose} />`
  - `SongTable` は 197行目: `<SongTable slot="list" selected={selectedFolderHash} active={activeTab === 'songs'} on:select={handleSelect} on:deselect={handleDeselect} />`
- `SongTable.svelte`:
  - 行の表示は 234-253行目の仮想スクロール内
  - 選択行は `bg-primary/20`、非選択は `hover:bg-base-200`
  - 移動済み行: `bg-warning/20` 背景＋「移動済み」バッジを追加

- [ ] **Step 1: App.svelte を更新**

`handleSongMoved` ハンドラを追加（`handleMemberMerged` の後に配置）:

```typescript
// 楽曲フォルダ移動済みの状態（セッション中のみ）
let movedFolderHashes: Set<string> = new Set()

function handleSongMoved(e: CustomEvent<{ folderHash: string }>) {
  movedFolderHashes = new Set([...movedFolderHashes, e.detail.folderHash])
  selectedFolderHash = null
}
```

SongTable に `movedHashes` props を追加:
```svelte
<SongTable slot="list" selected={selectedFolderHash} movedHashes={movedFolderHashes} active={activeTab === 'songs'} on:select={handleSelect} on:deselect={handleDeselect} />
```

SongDetail に `moved` props と `on:moved` イベントを追加:
```svelte
<SongDetail folderHash={selectedFolderHash} moved={movedFolderHashes.has(selectedFolderHash)} on:close={handleClose} on:moved={handleSongMoved} />
```

- [ ] **Step 2: SongTable.svelte を更新**

props 追加:
```typescript
export let movedHashes: Set<string> = new Set()
```

行のクラスを変更（238行目付近）。現在:
```svelte
class="flex absolute w-full border-b border-base-300/50 items-center px-2 cursor-pointer
  {selected === row.original.folderHash ? 'bg-primary/20' : 'hover:bg-base-200'}"
```

変更後:
```svelte
class="flex absolute w-full border-b border-base-300/50 items-center px-2 cursor-pointer
  {selected === row.original.folderHash ? 'bg-primary/20' : movedHashes.has(row.original.folderHash) ? 'bg-warning/20' : 'hover:bg-base-200'}"
```

行の先頭セル表示の直前（`{#each row.getVisibleCells() as cell}` の前）に移動済みバッジを追加:
```svelte
{#if movedHashes.has(row.original.folderHash)}
  <span class="badge badge-warning badge-xs shrink-0">移動済み</span>
{/if}
```

- [ ] **Step 3: フロントエンドビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa/frontend && npm run build`
Expected: 成功

- [ ] **Step 4: コミット**

```bash
git add frontend/src/App.svelte frontend/src/views/SongTable.svelte
git commit -m "feat: 楽曲一覧に移動済み表示（黄色背景＋バッジ）を追加"
```

---

### Task 6: 最終ビルド確認

- [ ] **Step 1: Wails フルビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build`
Expected: 成功
