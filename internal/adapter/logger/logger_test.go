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
