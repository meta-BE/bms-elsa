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
