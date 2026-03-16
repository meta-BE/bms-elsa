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
