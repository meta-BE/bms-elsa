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
