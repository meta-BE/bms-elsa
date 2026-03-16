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
		if d != nil && !d.IsDir() && d.Type()&os.ModeSymlink == 0 {
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
