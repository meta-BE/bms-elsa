package fileutil_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/meta-BE/bms-elsa/internal/domain/fileutil"
)

func TestMoveFileToFolder(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	destDir := filepath.Join(tmpDir, "dest")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(destDir, 0755)

	// テストファイル作成
	srcPath := filepath.Join(srcDir, "test.bms")
	os.WriteFile(srcPath, []byte("test content"), 0644)

	// 移動実行
	err := fileutil.MoveFileToFolder(srcPath, destDir)
	if err != nil {
		t.Fatalf("MoveFileToFolder failed: %v", err)
	}

	// 移動先に存在することを確認
	destPath := filepath.Join(destDir, "test.bms")
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("file should exist at dest: %v", err)
	}

	// 移動元から消えていることを確認
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Error("file should not exist at src")
	}
}

func TestMoveFileToFolder_DestFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	destDir := filepath.Join(tmpDir, "dest")
	os.MkdirAll(srcDir, 0755)
	os.MkdirAll(destDir, 0755)

	srcPath := filepath.Join(srcDir, "test.bms")
	os.WriteFile(srcPath, []byte("new"), 0644)

	// 移動先に同名ファイルが既に存在
	destPath := filepath.Join(destDir, "test.bms")
	os.WriteFile(destPath, []byte("existing"), 0644)

	err := fileutil.MoveFileToFolder(srcPath, destDir)
	if err == nil {
		t.Fatal("should return error when dest file exists")
	}
}

func TestMoveFileToFolder_DestDirNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "test.bms")
	os.WriteFile(srcPath, []byte("test"), 0644)

	err := fileutil.MoveFileToFolder(srcPath, filepath.Join(tmpDir, "nonexistent"))
	if err == nil {
		t.Fatal("should return error when dest dir does not exist")
	}
}
