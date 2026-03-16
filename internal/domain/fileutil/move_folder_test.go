package fileutil_test

import (
	"os"
	"path/filepath"
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
