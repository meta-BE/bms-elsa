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

	// 移動先に先にファイルを作成（同一タイムスタンプ = スキップのケースを検証）
	os.WriteFile(filepath.Join(destDir, "a.bms"), []byte("dest"), 0644)
	os.WriteFile(filepath.Join(srcDir, "a.bms"), []byte("src"), 0644)

	result, err := fileutil.MergeFolders(srcDir, destDir)
	if err != nil {
		t.Fatalf("MergeFolders failed: %v", err)
	}

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
