# 差分導入バグ修正 実装計画

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 差分導入のクロスドライブ移動失敗とBMSパーサーの文字化けを修正する

**Architecture:** move.goの`os.Rename`を`io.Copy`+`os.Remove`に置き換え、parser.goにShift-JIS→UTF-8変換を追加

**Tech Stack:** Go標準ライブラリ(`io`, `os`)、`golang.org/x/text/encoding/japanese`（既存依存）

---

### Task 1: MoveFileToFolder を copy+delete 方式に変更

**Files:**
- Modify: `internal/domain/fileutil/move.go`
- Test: `internal/domain/fileutil/move_test.go`（既存テスト3件で検証）

**Step 1: move.go を修正**

`os.Rename` を `copyFile` + `os.Remove` に置き換える。

```go
package fileutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// MoveFileToFolder はファイルを指定フォルダに移動する。
// 移動先に同名ファイルが存在する場合、移動先フォルダが存在しない場合はエラーを返す。
func MoveFileToFolder(srcPath, destFolder string) error {
	if _, err := os.Stat(destFolder); err != nil {
		return fmt.Errorf("移動先フォルダが存在しません: %s (%w)", destFolder, err)
	}

	filename := filepath.Base(srcPath)
	destPath := filepath.Join(destFolder, filename)

	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("移動先に同名ファイルが既に存在します: %s", destPath)
	}

	if err := copyFile(srcPath, destPath); err != nil {
		os.Remove(destPath)
		return fmt.Errorf("ファイルコピーに失敗: %w", err)
	}

	if err := os.Remove(srcPath); err != nil {
		return fmt.Errorf("コピー元の削除に失敗: %w", err)
	}

	return nil
}

func copyFile(src, dst string) error {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sf.Close()

	df, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(df, sf); err != nil {
		df.Close()
		return err
	}

	return df.Close()
}
```

**Step 2: 既存テストを実行して全てパスすることを確認**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/domain/fileutil/ -v`
Expected: 3件すべてPASS

**Step 3: コミット**

```bash
git add internal/domain/fileutil/move.go
git commit -m "fix: MoveFileToFolderをcopy+delete方式に変更しクロスドライブ移動に対応"
```

---

### Task 2: BMSパーサーにShift-JIS→UTF-8変換を追加

**Files:**
- Modify: `internal/domain/bms/parser.go`
- Test: `internal/domain/bms/parser_test.go`

**Step 1: parser.go にエンコーディング変換を追加**

`os.ReadFile` 後、UTF-8バリデーション → 非UTF-8ならShift-JISデコード。
MD5はオリジナルバイト列から計算し、パースはUTF-8変換後のデータで行う。

```go
// importに追加
"unicode/utf8"
"golang.org/x/text/encoding/japanese"
"golang.org/x/text/transform"
```

`ParseBMSFile` 関数内、`data, err := os.ReadFile(path)` の直後に以下を追加:

```go
	hash := md5.Sum(data)

	// Shift-JIS → UTF-8 変換（BMSの事実上の標準エンコーディング）
	if !utf8.Valid(data) {
		decoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(data), japanese.ShiftJIS.NewDecoder()))
		if err == nil {
			data = decoded
		}
	}

	result := &ParsedBMS{
```

既存の `hash := md5.Sum(data)` の行を上記の位置に移動し、変換前のdataでMD5を計算する。

**Step 2: Shift-JISテスト用BMSファイルを作成してテスト**

`internal/domain/bms/testdata/sjis_test.bms` をShift-JISエンコーディングで作成する。

```go
// parser_test.go に追加
func TestParseBMSFile_ShiftJIS(t *testing.T) {
	// Shift-JISエンコードのテストファイルを動的に作成
	dir := t.TempDir()
	path := filepath.Join(dir, "sjis_test.bms")

	encoder := japanese.ShiftJIS.NewEncoder()
	content := "#TITLE テスト楽曲\n#ARTIST テストアーティスト\n#WAV01 test.wav\n"
	sjisBytes, err := io.ReadAll(transform.NewReader(strings.NewReader(content), encoder))
	if err != nil {
		t.Fatalf("Shift-JIS encode failed: %v", err)
	}
	os.WriteFile(path, sjisBytes, 0644)

	parsed, err := bms.ParseBMSFile(path)
	if err != nil {
		t.Fatalf("ParseBMSFile failed: %v", err)
	}

	if parsed.Title != "テスト楽曲" {
		t.Errorf("Title = %q, want %q", parsed.Title, "テスト楽曲")
	}
	if parsed.Artist != "テストアーティスト" {
		t.Errorf("Artist = %q, want %q", parsed.Artist, "テストアーティスト")
	}
}
```

テストファイルのimportに以下を追加:
```go
"io"
"strings"
"golang.org/x/text/encoding/japanese"
"golang.org/x/text/transform"
```

**Step 3: テスト実行**

Run: `cd /Users/yudai.kuroki/src/github.com/meta-BE/bms-elsa && go test ./internal/domain/bms/ -v -run TestParseBMSFile`
Expected: 新規テスト含め全てPASS

**Step 4: コミット**

```bash
git add internal/domain/bms/parser.go internal/domain/bms/parser_test.go
git commit -m "fix: BMSパーサーにShift-JIS→UTF-8変換を追加し文字化けを解消"
```
