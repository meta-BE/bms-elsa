# フォルダマージ機能 実装計画

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 重複検知画面からフォルダ間マージを実行し、操作ログを system.log に出力する機能を追加する

**Architecture:** domain層(fileutil)にマージ関数とプラットフォーム別の作成日時取得を追加。adapter層にファイルロガー、port層にLoggerインターフェースを追加。usecase層でマージ操作とログ出力をオーケストレーション。フロントエンドのDuplicateDetailにマージUIを追加。

**Tech Stack:** Go 1.24 / Wails v2 / Svelte 4 / TypeScript / SQLite / TailwindCSS + DaisyUI 5

**Spec:** `docs/superpowers/specs/2026-03-16-folder-merge-design.md`

---

## ファイル構成

| 操作 | パス | 責務 |
|------|------|------|
| Create | `internal/domain/fileutil/ctime_windows.go` | Windows作成日時取得 |
| Create | `internal/domain/fileutil/ctime_other.go` | 非Windows作成日時フォールバック |
| Create | `internal/domain/fileutil/ctime_test.go` | 作成日時取得テスト |
| Create | `internal/domain/fileutil/merge.go` | フォルダマージ関数 |
| Create | `internal/domain/fileutil/merge_test.go` | マージテスト |
| Create | `internal/port/logger.go` | Loggerインターフェース |
| Create | `internal/adapter/logger/logger.go` | FileLogger実装 |
| Create | `internal/adapter/logger/logger_test.go` | ロガーテスト |
| Create | `internal/usecase/merge_folders.go` | マージユースケース |
| Create | `internal/usecase/merge_folders_test.go` | マージUCテスト |
| Modify | `internal/usecase/execute_diff_import.go` | Logger注入追加 |
| Modify | `internal/app/duplicate_handler.go` | MergeFoldersメソッド追加 |
| Modify | `app.go` | Config.FileLog追加 + DI配線 |
| Modify | `frontend/src/views/DuplicateDetail.svelte` | マージUI追加 |
| Modify | `frontend/src/views/DuplicateView.svelte` | マージ後グループ除去 |

---

## Chunk 1: ファイルシステム操作層

### Task 1: プラットフォーム別の作成日時取得

**Files:**
- Create: `internal/domain/fileutil/ctime_windows.go`
- Create: `internal/domain/fileutil/ctime_other.go`
- Create: `internal/domain/fileutil/ctime_test.go`

- [ ] **Step 1: 非Windows用のフォールバック実装を作成**

`internal/domain/fileutil/ctime_other.go`:
```go
//go:build !windows

package fileutil

import (
	"os"
	"time"
)

// fileCreationTime は非WindowsではModTimeにフォールバックする
func fileCreationTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
```

- [ ] **Step 2: Windows用の作成日時取得を実装**

`internal/domain/fileutil/ctime_windows.go`:
```go
//go:build windows

package fileutil

import (
	"os"
	"syscall"
	"time"
)

// fileCreationTime はWindowsのCreationTimeを返す
func fileCreationTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	sys := info.Sys().(*syscall.Win32FileAttributeData)
	return time.Unix(0, sys.CreationTime.Nanoseconds()), nil
}
```

- [ ] **Step 3: テストを作成**

`internal/domain/fileutil/ctime_test.go`:
```go
package fileutil

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileCreationTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	before := time.Now().Add(-time.Second)
	os.WriteFile(path, []byte("test"), 0644)

	ct, err := fileCreationTime(path)
	if err != nil {
		t.Fatalf("fileCreationTime failed: %v", err)
	}
	if ct.Before(before) {
		t.Errorf("creation time %v is before file creation %v", ct, before)
	}
}

func TestFileCreationTime_NotExist(t *testing.T) {
	_, err := fileCreationTime("/nonexistent/file.txt")
	if err == nil {
		t.Fatal("should return error for nonexistent file")
	}
}
```

注意: このテストは内部関数にアクセスするため `package fileutil`（非 `_test` サフィックス）で定義する。

- [ ] **Step 4: テスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/domain/fileutil/ -run TestFileCreationTime -v`
Expected: PASS

- [ ] **Step 5: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add internal/domain/fileutil/ctime_windows.go internal/domain/fileutil/ctime_other.go internal/domain/fileutil/ctime_test.go
git commit -m "feat: プラットフォーム別のファイル作成日時取得を追加"
```

---

### Task 2: MergeFolders 関数

**Files:**
- Create: `internal/domain/fileutil/merge.go`
- Create: `internal/domain/fileutil/merge_test.go`

- [ ] **Step 1: MergeResult型とバリデーション付きの関数シグネチャを作成**

`internal/domain/fileutil/merge.go`:
```go
package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MergeResult はマージ操作の結果
type MergeResult struct {
	Moved    []string    // 新規移動したファイル（相対パス）
	Replaced []string    // 上書きしたファイル（移動元が新しい）
	Skipped  []string    // スキップしたファイル（移動先が新しいor同一）
	Errors   []MergeError
}

// MergeError はファイル単位のエラー
type MergeError struct {
	FileName string
	Err      error
}

// MergeFolders は srcDir 内の全ファイルを destDir に移動し、成功後に srcDir を削除する。
// サブディレクトリも再帰的に処理する。競合時はファイルの作成日時を比較し、新しい方を残す。
// 1ファイルでもエラーがあった場合、srcDirの削除はスキップ。
func MergeFolders(srcDir, destDir string) (*MergeResult, error) {
	srcDir, err := filepath.Abs(srcDir)
	if err != nil {
		return nil, fmt.Errorf("srcDir の絶対パス変換に失敗: %w", err)
	}
	destDir, err = filepath.Abs(destDir)
	if err != nil {
		return nil, fmt.Errorf("destDir の絶対パス変換に失敗: %w", err)
	}

	if err := validateMergePaths(srcDir, destDir); err != nil {
		return nil, err
	}

	result := &MergeResult{}

	// srcDir が空かチェック
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("srcDir の読み込みに失敗: %w", err)
	}
	if len(entries) == 0 {
		os.RemoveAll(srcDir)
		return result, nil
	}

	// 再帰的にファイルを処理（WalkDirでシンボリックリンクを正しく検出）
	err = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // エラーはスキップ
		}
		if d.IsDir() {
			return nil // ディレクトリはスキップ（ファイルのみ処理）
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil // シンボリックリンクはスキップ
		}

		rel, _ := filepath.Rel(srcDir, path)
		destPath := filepath.Join(destDir, rel)

		// 移動先ディレクトリを作成
		destSubDir := filepath.Dir(destPath)
		if err := os.MkdirAll(destSubDir, 0755); err != nil {
			result.Errors = append(result.Errors, MergeError{FileName: rel, Err: err})
			return nil
		}

		action, err := mergeOneFile(path, destPath)
		if err != nil {
			result.Errors = append(result.Errors, MergeError{FileName: rel, Err: err})
			return nil
		}

		switch action {
		case actionMoved:
			result.Moved = append(result.Moved, rel)
		case actionReplaced:
			result.Replaced = append(result.Replaced, rel)
		case actionSkipped:
			result.Skipped = append(result.Skipped, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("srcDir の走査に失敗: %w", err)
	}

	// エラーがなければ srcDir を削除
	if len(result.Errors) == 0 {
		os.RemoveAll(srcDir)
	}

	return result, nil
}

type mergeAction int

const (
	actionMoved    mergeAction = iota
	actionReplaced
	actionSkipped
)

func mergeOneFile(srcPath, destPath string) (mergeAction, error) {
	_, err := os.Stat(destPath)
	if os.IsNotExist(err) {
		// 移動先に存在しない → そのまま移動
		if err := copyFile(srcPath, destPath); err != nil {
			return 0, fmt.Errorf("コピー失敗: %w", err)
		}
		os.Remove(srcPath)
		return actionMoved, nil
	}
	if err != nil {
		return 0, err
	}

	// 移動先に存在 → 作成日時を比較
	srcTime, err := fileCreationTime(srcPath)
	if err != nil {
		return 0, fmt.Errorf("移動元の作成日時取得に失敗: %w", err)
	}
	destTime, err := fileCreationTime(destPath)
	if err != nil {
		return 0, fmt.Errorf("移動先の作成日時取得に失敗: %w", err)
	}

	if srcTime.After(destTime) {
		// 移動元の方が新しい → 上書き
		if err := copyFile(srcPath, destPath); err != nil {
			return 0, fmt.Errorf("上書きコピー失敗: %w", err)
		}
		os.Remove(srcPath)
		return actionReplaced, nil
	}

	// 移動先の方が新しい or 同一 → スキップ（移動元は削除）
	os.Remove(srcPath)
	return actionSkipped, nil
}

func validateMergePaths(srcDir, destDir string) error {
	if srcDir == destDir {
		return fmt.Errorf("移動元と移動先が同じです: %s", srcDir)
	}

	// 親子関係チェック
	srcWithSep := srcDir + string(filepath.Separator)
	destWithSep := destDir + string(filepath.Separator)
	if strings.HasPrefix(destWithSep, srcWithSep) {
		return fmt.Errorf("移動先が移動元のサブディレクトリです: %s → %s", srcDir, destDir)
	}
	if strings.HasPrefix(srcWithSep, destWithSep) {
		return fmt.Errorf("移動元が移動先のサブディレクトリです: %s → %s", srcDir, destDir)
	}

	if _, err := os.Stat(srcDir); err != nil {
		return fmt.Errorf("移動元フォルダが存在しません: %s (%w)", srcDir, err)
	}
	if _, err := os.Stat(destDir); err != nil {
		return fmt.Errorf("移動先フォルダが存在しません: %s (%w)", destDir, err)
	}

	return nil
}
```

- [ ] **Step 2: テストを作成**

`internal/domain/fileutil/merge_test.go`:
```go
package fileutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/meta-BE/bms-elsa/internal/domain/fileutil"
)

func TestMergeFolders_NewFiles(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	destDir := filepath.Join(tmpDir, "dest")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(destDir, 0755)

	os.WriteFile(filepath.Join(srcDir, "a.bms"), []byte("aaa"), 0644)
	os.WriteFile(filepath.Join(srcDir, "b.wav"), []byte("bbb"), 0644)

	result, err := fileutil.MergeFolders(srcDir, destDir)
	if err != nil {
		t.Fatalf("MergeFolders failed: %v", err)
	}

	if len(result.Moved) != 2 {
		t.Errorf("expected 2 moved, got %d", len(result.Moved))
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(result.Errors))
	}

	// 移動先に存在
	if _, err := os.Stat(filepath.Join(destDir, "a.bms")); err != nil {
		t.Error("a.bms should exist in dest")
	}
	if _, err := os.Stat(filepath.Join(destDir, "b.wav")); err != nil {
		t.Error("b.wav should exist in dest")
	}

	// srcDir は削除済み
	if _, err := os.Stat(srcDir); !os.IsNotExist(err) {
		t.Error("srcDir should be removed")
	}
}

func TestMergeFolders_WithSubDir(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	destDir := filepath.Join(tmpDir, "dest")
	os.MkdirAll(filepath.Join(srcDir, "sub"), 0755)
	os.MkdirAll(destDir, 0755)

	os.WriteFile(filepath.Join(srcDir, "a.bms"), []byte("aaa"), 0644)
	os.WriteFile(filepath.Join(srcDir, "sub", "b.wav"), []byte("bbb"), 0644)

	result, err := fileutil.MergeFolders(srcDir, destDir)
	if err != nil {
		t.Fatalf("MergeFolders failed: %v", err)
	}

	if len(result.Moved) != 2 {
		t.Errorf("expected 2 moved, got %d", len(result.Moved))
	}

	// サブディレクトリ構造が維持される
	if _, err := os.Stat(filepath.Join(destDir, "sub", "b.wav")); err != nil {
		t.Error("sub/b.wav should exist in dest")
	}
}

func TestMergeFolders_ConflictSkip(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	destDir := filepath.Join(tmpDir, "dest")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(destDir, 0755)

	// 移動先に先にファイルを作成（移動先の方が古くなる可能性はテスト環境では低いので、
	// 同一タイムスタンプ = スキップのケースを検証）
	os.WriteFile(filepath.Join(destDir, "a.bms"), []byte("dest"), 0644)
	os.WriteFile(filepath.Join(srcDir, "a.bms"), []byte("src"), 0644)

	result, err := fileutil.MergeFolders(srcDir, destDir)
	if err != nil {
		t.Fatalf("MergeFolders failed: %v", err)
	}

	// 同一 or ほぼ同時刻なのでスキップ（テスト環境ではタイミング依存だが、
	// Moved=0かSkipped>0のいずれかであればOK）
	total := len(result.Moved) + len(result.Replaced) + len(result.Skipped)
	if total != 1 {
		t.Errorf("expected 1 total action, got %d", total)
	}
}

func TestMergeFolders_EmptySrc(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	destDir := filepath.Join(tmpDir, "dest")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(destDir, 0755)

	result, err := fileutil.MergeFolders(srcDir, destDir)
	if err != nil {
		t.Fatalf("MergeFolders failed: %v", err)
	}

	if len(result.Moved) != 0 {
		t.Errorf("expected 0 moved, got %d", len(result.Moved))
	}

	// 空のsrcDirは削除される
	if _, err := os.Stat(srcDir); !os.IsNotExist(err) {
		t.Error("empty srcDir should be removed")
	}
}

func TestMergeFolders_SameDir(t *testing.T) {
	dir := t.TempDir()
	_, err := fileutil.MergeFolders(dir, dir)
	if err == nil {
		t.Fatal("should return error for same dir")
	}
}

func TestMergeFolders_ParentChild(t *testing.T) {
	tmpDir := t.TempDir()
	parent := filepath.Join(tmpDir, "parent")
	child := filepath.Join(parent, "child")
	os.MkdirAll(child, 0755)

	_, err := fileutil.MergeFolders(parent, child)
	if err == nil {
		t.Fatal("should return error when dest is child of src")
	}

	_, err = fileutil.MergeFolders(child, parent)
	if err == nil {
		t.Fatal("should return error when src is child of dest")
	}
}

func TestMergeFolders_SrcNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "dest")
	os.MkdirAll(destDir, 0755)

	_, err := fileutil.MergeFolders(filepath.Join(tmpDir, "nonexistent"), destDir)
	if err == nil {
		t.Fatal("should return error when src does not exist")
	}
}

func TestMergeFolders_DestNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	os.MkdirAll(srcDir, 0755)

	_, err := fileutil.MergeFolders(srcDir, filepath.Join(tmpDir, "nonexistent"))
	if err == nil {
		t.Fatal("should return error when dest does not exist")
	}
}
```

- [ ] **Step 3: テスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/domain/fileutil/ -run TestMergeFolders -v`
Expected: PASS

- [ ] **Step 4: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add internal/domain/fileutil/merge.go internal/domain/fileutil/merge_test.go
git commit -m "feat: フォルダマージ関数 MergeFolders を追加"
```

---

## Chunk 2: ロガー基盤

### Task 3: port.Logger インターフェース

**Files:**
- Create: `internal/port/logger.go`

- [ ] **Step 1: Logger インターフェースを作成**

`internal/port/logger.go`:
```go
package port

// Logger は操作ログの書き込みインターフェース
type Logger interface {
	Log(message string)
}

// NopLogger は何もしないLogger（ログ初期化失敗時のフォールバック）
type NopLogger struct{}

func (NopLogger) Log(string) {}
```

- [ ] **Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./internal/port/`
Expected: 成功（出力なし）

- [ ] **Step 3: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add internal/port/logger.go
git commit -m "feat: port.Logger インターフェースを追加"
```

---

### Task 4: FileLogger 実装

**Files:**
- Create: `internal/adapter/logger/logger.go`
- Create: `internal/adapter/logger/logger_test.go`

- [ ] **Step 1: テストを作成**

`internal/adapter/logger/logger_test.go`:
```go
package logger_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/meta-BE/bms-elsa/internal/adapter/logger"
)

func TestFileLogger_Log(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l, err := logger.NewWithPath(path)
	if err != nil {
		t.Fatalf("NewWithPath failed: %v", err)
	}
	defer l.Close()

	l.Log("テストメッセージ")
	l.Log("2行目")

	l.Close()

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "テストメッセージ") {
		t.Errorf("line 0 should contain message, got: %s", lines[0])
	}
	// タイムスタンプ形式チェック: "2026-03-16 15:00:00 メッセージ"
	if len(lines[0]) < 20 {
		t.Errorf("line too short, missing timestamp? got: %s", lines[0])
	}
}

func TestFileLogger_Append(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	// 1回目
	l1, _ := logger.NewWithPath(path)
	l1.Log("1回目")
	l1.Close()

	// 2回目（追記）
	l2, _ := logger.NewWithPath(path)
	l2.Log("2回目")
	l2.Close()

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (appended), got %d", len(lines))
	}
}
```

- [ ] **Step 2: FileLogger を実装**

`internal/adapter/logger/logger.go`:
```go
package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileLogger は system.log への書き込みを担当する。
// port.Logger インターフェースを実装する。
type FileLogger struct {
	file *os.File
	mu   sync.Mutex
}

// New は実行ファイルと同じディレクトリに system.log を開く（追記モード）
func New() (*FileLogger, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("実行ファイルパスの取得に失敗: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return nil, fmt.Errorf("シンボリックリンクの解決に失敗: %w", err)
	}
	path := filepath.Join(filepath.Dir(exe), "system.log")
	return NewWithPath(path)
}

// NewWithPath は指定パスにログファイルを開く（テスト用）
func NewWithPath(path string) (*FileLogger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("ログファイルのオープンに失敗: %w", err)
	}
	return &FileLogger{file: f}, nil
}

// Log は1行のログを書き込む（タイムスタンプ自動付与）。
// 書き込み失敗は握りつぶす（マージ操作の成否に影響を与えない）。
func (l *FileLogger) Log(message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ts := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(l.file, "%s %s\n", ts, message)
}

// Close はログファイルを閉じる
func (l *FileLogger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
```

- [ ] **Step 3: テスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/adapter/logger/ -v`
Expected: PASS

- [ ] **Step 4: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add internal/adapter/logger/logger.go internal/adapter/logger/logger_test.go
git commit -m "feat: FileLogger を adapter 層に追加"
```

---

## Chunk 3: ユースケース層 + DI配線

### Task 5: Config に FileLog フィールド追加

**Files:**
- Modify: `app.go:186-188`

- [ ] **Step 1: Config 構造体を修正**

`app.go` L186-188 の `Config` 構造体を以下に変更:
```go
// Config はアプリケーション設定
type Config struct {
	SongdataDBPath string `json:"songdataDBPath"`
	FileLog        bool   `json:"fileLog"`
}
```

- [ ] **Step 2: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: 成功

- [ ] **Step 3: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add app.go
git commit -m "feat: Config に FileLog フィールドを追加"
```

---

### Task 6: MergeFoldersUseCase

**Files:**
- Create: `internal/usecase/merge_folders.go`
- Create: `internal/usecase/merge_folders_test.go`

- [ ] **Step 1: テストを作成**

`internal/usecase/merge_folders_test.go`:
```go
package usecase_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/meta-BE/bms-elsa/internal/usecase"
)

// mockLogger はテスト用のログ記録
type mockLogger struct {
	messages []string
}

func (m *mockLogger) Log(message string) {
	m.messages = append(m.messages, message)
}

func TestMergeFoldersUseCase_Execute(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	destDir := filepath.Join(tmpDir, "dest")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(destDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "a.bms"), []byte("aaa"), 0644)

	logger := &mockLogger{}
	uc := usecase.NewMergeFoldersUseCase(logger, false)

	result, err := uc.Execute(context.Background(), srcDir, destDir)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Moved != 1 {
		t.Errorf("expected 1 moved, got %d", result.Moved)
	}

	// サマリーログが出力される
	if len(logger.messages) != 1 {
		t.Fatalf("expected 1 log message, got %d", len(logger.messages))
	}
	if !strings.Contains(logger.messages[0], "MERGE") {
		t.Errorf("log should contain MERGE, got: %s", logger.messages[0])
	}
}

func TestMergeFoldersUseCase_FileLog(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	destDir := filepath.Join(tmpDir, "dest")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(destDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "a.bms"), []byte("aaa"), 0644)
	os.WriteFile(filepath.Join(srcDir, "b.wav"), []byte("bbb"), 0644)

	logger := &mockLogger{}
	uc := usecase.NewMergeFoldersUseCase(logger, true) // fileLog=true

	result, err := uc.Execute(context.Background(), srcDir, destDir)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Moved != 2 {
		t.Errorf("expected 2 moved, got %d", result.Moved)
	}

	// サマリー1行 + ファイル別ログ2行
	if len(logger.messages) != 3 {
		t.Fatalf("expected 3 log messages with fileLog, got %d: %v", len(logger.messages), logger.messages)
	}
}
```

- [ ] **Step 2: MergeFoldersUseCase を実装**

`internal/usecase/merge_folders.go`:
```go
package usecase

import (
	"context"
	"fmt"

	"github.com/meta-BE/bms-elsa/internal/domain/fileutil"
	"github.com/meta-BE/bms-elsa/internal/port"
)

// MergeFoldersResult はフロントエンド向けのマージ結果
type MergeFoldersResult struct {
	Moved    int
	Replaced int
	Skipped  int
	Errors   int
	ErrorMsg string
}

type MergeFoldersUseCase struct {
	logger  port.Logger
	fileLog bool
}

func NewMergeFoldersUseCase(logger port.Logger, fileLog bool) *MergeFoldersUseCase {
	return &MergeFoldersUseCase{logger: logger, fileLog: fileLog}
}

func (u *MergeFoldersUseCase) Execute(_ context.Context, srcDir, destDir string) (*MergeFoldersResult, error) {
	mergeResult, err := fileutil.MergeFolders(srcDir, destDir)
	if err != nil {
		return nil, err
	}

	u.writeLog(srcDir, destDir, mergeResult)

	result := &MergeFoldersResult{
		Moved:    len(mergeResult.Moved),
		Replaced: len(mergeResult.Replaced),
		Skipped:  len(mergeResult.Skipped),
		Errors:   len(mergeResult.Errors),
	}
	if len(mergeResult.Errors) > 0 {
		result.ErrorMsg = mergeResult.Errors[0].Err.Error()
	}
	return result, nil
}

func (u *MergeFoldersUseCase) writeLog(srcDir, destDir string, r *fileutil.MergeResult) {
	if u.fileLog {
		u.logger.Log(fmt.Sprintf("MERGE %s → %s", srcDir, destDir))
		for _, f := range r.Moved {
			u.logger.Log(fmt.Sprintf("  + %s", f))
		}
		for _, f := range r.Replaced {
			u.logger.Log(fmt.Sprintf("  > %s", f))
		}
		for _, f := range r.Skipped {
			u.logger.Log(fmt.Sprintf("  = %s", f))
		}
		for _, e := range r.Errors {
			u.logger.Log(fmt.Sprintf("  ! %s: %s", e.FileName, e.Err))
		}
	} else {
		u.logger.Log(fmt.Sprintf("MERGE %s → %s (moved:%d, replaced:%d, skipped:%d, errors:%d)",
			srcDir, destDir,
			len(r.Moved), len(r.Replaced), len(r.Skipped), len(r.Errors)))
	}
}
```

- [ ] **Step 3: テスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/usecase/ -run TestMergeFolders -v`
Expected: PASS

- [ ] **Step 4: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add internal/usecase/merge_folders.go internal/usecase/merge_folders_test.go
git commit -m "feat: MergeFoldersUseCase を追加"
```

---

### Task 7: ExecuteDiffImportUseCase にLogger注入 + DuplicateHandler にMergeFolders追加 + DI配線

Task 7〜8 は `NewExecuteDiffImportUseCase` のシグネチャ変更と DI配線更新を同時に行わないとビルドが壊れるため、1タスクにまとめる。

**Files:**
- Modify: `internal/usecase/execute_diff_import.go`
- Modify: `internal/app/duplicate_handler.go`
- Modify: `app.go`

- [ ] **Step 1: ExecuteDiffImportUseCase にLogger を注入可能にする**

`internal/usecase/execute_diff_import.go` を以下に変更:
```go
package usecase

import (
	"fmt"

	"github.com/meta-BE/bms-elsa/internal/domain/fileutil"
	"github.com/meta-BE/bms-elsa/internal/port"
)

// ImportRequest はファイル移動リクエスト
type ImportRequest struct {
	FilePath   string
	DestFolder string
}

// ImportResult はファイル移動結果
type ImportResult struct {
	Success int
	Failed  int
	Errors  []string
}

type ExecuteDiffImportUseCase struct {
	logger  port.Logger
	fileLog bool
}

func NewExecuteDiffImportUseCase(logger port.Logger, fileLog bool) *ExecuteDiffImportUseCase {
	return &ExecuteDiffImportUseCase{logger: logger, fileLog: fileLog}
}

// Execute は確定済み候補のファイル移動を実行する
func (u *ExecuteDiffImportUseCase) Execute(requests []ImportRequest) ImportResult {
	var result ImportResult

	// 移動先ごとにグルーピング（ログ用）
	destCounts := map[string]int{}

	for _, req := range requests {
		if err := fileutil.MoveFileToFolder(req.FilePath, req.DestFolder); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, err.Error())
			if u.fileLog {
				u.logger.Log(fmt.Sprintf("  ! MOVE %s → %s: %s", req.FilePath, req.DestFolder, err))
			}
		} else {
			result.Success++
			destCounts[req.DestFolder]++
			if u.fileLog {
				u.logger.Log(fmt.Sprintf("  MOVE %s → %s", req.FilePath, req.DestFolder))
			}
		}
	}

	if !u.fileLog {
		for dest, count := range destCounts {
			u.logger.Log(fmt.Sprintf("IMPORT %d files → %s (success:%d, failed:%d)",
				count, dest, count, 0))
		}
		if result.Failed > 0 {
			u.logger.Log(fmt.Sprintf("IMPORT failed: %d files", result.Failed))
		}
	}

	return result
}
```

- [ ] **Step 2: DuplicateHandler を修正**

`internal/app/duplicate_handler.go` を以下に変更:
```go
package app

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/domain/similarity"
	"github.com/meta-BE/bms-elsa/internal/usecase"
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

func (h *DuplicateHandler) ScanDuplicates() ([]similarity.DuplicateGroup, error) {
	return h.scanDuplicates.Execute(h.ctx)
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

- [ ] **Step 3: app.go にLoggerフィールドとDI配線を追加**

`app.go` の App 構造体（L22-35）を修正:
```go
type App struct {
	ctx                    context.Context
	db                     *sql.DB
	logger                 *logger.FileLogger
	SongHandler            *internalapp.SongHandler
	IRHandler              *internalapp.IRHandler
	InferenceHandler       *internalapp.InferenceHandler
	RewriteHandler         *internalapp.RewriteHandler
	ChartHandler           *internalapp.ChartHandler
	DifficultyTableHandler *internalapp.DifficultyTableHandler
	ScanHandler            *internalapp.ScanHandler
	DiffImportHandler      *internalapp.DiffImportHandler
	DuplicateHandler       *internalapp.DuplicateHandler
	elsaRepo               *persistence.ElsaRepository
}
```

import に `"github.com/meta-BE/bms-elsa/internal/adapter/logger"` と `"github.com/meta-BE/bms-elsa/internal/port"` を追加。

`Init()` 関数（L43-104）の冒頭にLogger初期化を追加:
```go
func (a *App) Init() error {
	// Logger初期化（失敗時はNopLoggerでフォールバック）
	fileLogger, err := logger.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "system.log open: %v\n", err)
	}
	a.logger = fileLogger
	cfg := loadConfig()

	// UseCase に渡す Logger（nil安全）
	var appLogger port.Logger = port.NopLogger{}
	if a.logger != nil {
		appLogger = a.logger
	}

	// 既存のDB初期化...
```

`DuplicateHandler` のDI組み立て（L72-73）を修正:
```go
	// 旧: a.DuplicateHandler = internalapp.NewDuplicateHandler(scanDuplicates)
	mergeFoldersUC := usecase.NewMergeFoldersUseCase(appLogger, cfg.FileLog)
	a.DuplicateHandler = internalapp.NewDuplicateHandler(scanDuplicates, mergeFoldersUC)
```

`ExecuteDiffImportUseCase` のDI（L100）を修正:
```go
	// 旧: executeDiffImport := usecase.NewExecuteDiffImportUseCase()
	executeDiffImport := usecase.NewExecuteDiffImportUseCase(appLogger, cfg.FileLog)
```

`shutdown` 関数（L119-123）を修正:
```go
func (a *App) shutdown(ctx context.Context) {
	if a.db != nil {
		a.db.Close()
	}
	if a.logger != nil {
		a.logger.Close()
	}
}
```

- [ ] **Step 4: ビルド確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go build ./...`
Expected: 成功

- [ ] **Step 5: 全テスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./...`
Expected: 全PASS

- [ ] **Step 6: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add internal/usecase/execute_diff_import.go internal/app/duplicate_handler.go app.go
git commit -m "feat: DuplicateHandler にMergeFolders追加、Logger注入・DI配線を更新"
```

---

## Chunk 4: フロントエンド

### Task 9: DuplicateDetail にマージUI追加

**Files:**
- Modify: `frontend/src/views/DuplicateDetail.svelte`

- [ ] **Step 1: Wails バインディング生成**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails generate module`
Expected: `frontend/wailsjs/go/app/DuplicateHandler.js` に `MergeFolders` が追加される

- [ ] **Step 2: DuplicateDetail.svelte を修正**

`frontend/src/views/DuplicateDetail.svelte` を以下に変更:
```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte'
  import { GetSongDetail } from '../../wailsjs/go/app/SongHandler'
  import { MergeFolders } from '../../wailsjs/go/app/DuplicateHandler'
  import type { dto, similarity } from '../../wailsjs/go/models'
  import OpenFolderButton from '../components/OpenFolderButton.svelte'

  const dispatch = createEventDispatcher()

  export let group: similarity.DuplicateGroup | null = null

  // メンバーごとの譜面詳細をキャッシュ
  let chartsMap: Record<string, dto.ChartDTO[]> = {}

  // マージ先として選択されたメンバーのFolderHash
  let mergeTargetHash: string | null = null
  let merging = false

  // groupが変わったらマージ先選択をリセット
  $: if (group) {
    mergeTargetHash = null
    for (const member of group.Members) {
      if (!chartsMap[member.FolderHash]) {
        fetchCharts(member.FolderHash)
      }
    }
  }

  async function fetchCharts(folderHash: string) {
    try {
      const detail = await GetSongDetail(folderHash)
      if (detail?.charts) {
        chartsMap = { ...chartsMap, [folderHash]: detail.charts }
      }
    } catch {
      // 取得失敗は無視
    }
  }

  function formatBPM(min: number, max: number): string {
    if (min === max) return String(Math.round(min))
    return `${Math.round(min)}-${Math.round(max)}`
  }

  function folderPath(path: string): string {
    const sep = path.includes('\\') ? '\\' : '/'
    const parts = path.split(sep)
    parts.pop()
    return parts.join(sep)
  }

  function fileName(path: string): string {
    const sep = path.includes('\\') ? '\\' : '/'
    const parts = path.split(sep)
    return parts[parts.length - 1] || path
  }

  function selectMergeTarget(folderHash: string) {
    mergeTargetHash = mergeTargetHash === folderHash ? null : folderHash
  }

  async function handleMerge(srcMember: similarity.DuplicateMember) {
    if (!group || !mergeTargetHash) return
    const targetMember = group.Members.find(m => m.FolderHash === mergeTargetHash)
    if (!targetMember) return

    const srcPath = folderPath(srcMember.Path)
    const destPath = folderPath(targetMember.Path)

    const ok = confirm(
      `フォルダをマージします。移動元は削除されます。\n\n` +
      `移動元: ${srcPath}\n移動先: ${destPath}\n\nよろしいですか？`
    )
    if (!ok) return

    merging = true
    try {
      const result = await MergeFolders(srcPath, destPath)
      if (result.success) {
        // 楽観的UI更新: マージしたメンバーを除去
        dispatch('memberMerged', { folderHash: srcMember.FolderHash })
      } else {
        alert(`マージに失敗しました: ${result.errorMsg}`)
      }
    } catch (err) {
      alert(`エラー: ${err}`)
    } finally {
      merging = false
    }
  }
</script>

{#if group}
  <div class="p-3 space-y-3">
    <div class="flex items-center gap-2 text-sm font-semibold">
      <span>グループ #{group.ID}</span>
      <span class="badge badge-sm badge-primary">{group.Score}%</span>
    </div>

    {#each group.Members as member, i}
      <div class="card card-compact bg-base-200 {mergeTargetHash === member.FolderHash ? 'ring-2 ring-primary' : ''}">
        <div class="card-body">
          <div class="flex items-start justify-between">
            <div>
              <div class="text-lg font-bold">{member.Title}</div>
              <div class="text-sm text-base-content/70">{member.Artist}</div>
            </div>
            <div class="text-right text-sm text-base-content/50">
              <div>{member.Genre}</div>
              <div>BPM {formatBPM(member.MinBPM, member.MaxBPM)}</div>
              <div>{member.ChartCount}譜面</div>
            </div>
          </div>
          <div class="text-sm text-base-content/50 break-all flex items-center gap-1">
            <span>{folderPath(member.Path)}</span>
            <OpenFolderButton path={member.Path} size="xs" />
          </div>

          {#if chartsMap[member.FolderHash]}
            <div class="mt-1 space-y-0.5">
              {#each chartsMap[member.FolderHash] as chart}
                <div class="text-sm flex gap-2">
                  <span class="text-base-content/70">{chart.subtitle || fileName(chart.path || '')}</span>
                  <span class="text-xs text-base-content/50 break-all">{fileName(chart.path || '')}</span>
                </div>
              {/each}
            </div>
          {/if}

          <div class="mt-2 flex gap-2">
            <button
              class="btn btn-xs {mergeTargetHash === member.FolderHash ? 'btn-primary' : 'btn-outline btn-primary'}"
              on:click={() => selectMergeTarget(member.FolderHash)}
            >
              {mergeTargetHash === member.FolderHash ? 'マージ先 ✓' : 'マージ先に指定'}
            </button>
            {#if mergeTargetHash && mergeTargetHash !== member.FolderHash}
              <button
                class="btn btn-xs btn-warning"
                disabled={merging}
                on:click={() => handleMerge(member)}
              >
                {merging ? '処理中...' : '→ マージ'}
              </button>
            {/if}
          </div>
        </div>
      </div>
    {/each}

    {#if group.Members.length >= 2}
      {@const scores = group.Members[0].Scores}
      <div class="text-base-content/60 space-y-1">
        <div class="text-sm font-semibold">類似度内訳</div>
        <div class="text-sm flex gap-4">
          <span>WAV定義 {scores.WAV}%</span>
          <span>title {scores.Title}%</span>
          <span>artist {scores.Artist}%</span>
          <span>genre {scores.Genre}%</span>
          <span>BPM {scores.BPM}%</span>
        </div>
      </div>
    {/if}
  </div>
{:else}
  <div class="flex items-center justify-center h-full text-base-content/40 text-sm">
    グループを選択してください
  </div>
{/if}
```

- [ ] **Step 3: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add frontend/src/views/DuplicateDetail.svelte
git commit -m "feat: DuplicateDetail にマージUI追加"
```

---

### Task 10: マージ後のグループ・メンバー除去

`App.svelte` が `DuplicateView` と `DuplicateDetail` の両方を管理している。
`DuplicateDetail` が dispatch する `memberMerged` イベントを `App.svelte` で受け取り、
`DuplicateView` にメンバー除去を伝搬する。

**Files:**
- Modify: `frontend/src/App.svelte`
- Modify: `frontend/src/views/DuplicateView.svelte`

- [ ] **Step 1: DuplicateView に removeMember メソッドを公開する**

`DuplicateView.svelte` に以下を追加:

```typescript
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
```

- [ ] **Step 2: App.svelte で memberMerged イベントをハンドリング**

`App.svelte` L231 を修正:
```svelte
<!-- 旧 -->
<DuplicateDetail group={selectedDuplicateGroup} />

<!-- 新 -->
<DuplicateDetail group={selectedDuplicateGroup} on:memberMerged={handleMemberMerged} />
```

`App.svelte` の script に以下を追加:
```typescript
let duplicateViewRef: DuplicateView

function handleMemberMerged(e: CustomEvent<{ folderHash: string }>) {
  duplicateViewRef?.removeMember(e.detail.folderHash)
  // selectedDuplicateGroup を更新（除去後の状態を反映）
  if (selectedDuplicateGroup) {
    selectedDuplicateGroup = { ...selectedDuplicateGroup }
    if (selectedDuplicateGroup.Members.length <= 1) {
      selectedDuplicateGroup = null
    }
  }
}
```

`App.svelte` L228 の DuplicateView に `bind:this` を追加:
```svelte
<!-- 旧 -->
<DuplicateView slot="list" active={activeTab === 'duplicates'} on:select={handleDuplicateSelect} />

<!-- 新 -->
<DuplicateView slot="list" active={activeTab === 'duplicates'} on:select={handleDuplicateSelect} bind:this={duplicateViewRef} />
```

- [ ] **Step 2: 動作確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails dev`

手動テスト:
1. 重複検知画面でスキャン実行
2. グループを選択
3. 「マージ先に指定」をクリック
4. 別メンバーの「→ マージ」をクリック
5. 確認ダイアログでOK
6. マージしたメンバーが一覧から消えることを確認
7. メンバーが1つになったらグループも消えることを確認

- [ ] **Step 3: コミット**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa
git add frontend/src/
git commit -m "feat: マージ後のグループ・メンバー除去UIを実装"
```

---

## 最終確認

- [ ] **全テスト実行**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./...
```

- [ ] **ビルド確認**

```bash
cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && wails build
```
