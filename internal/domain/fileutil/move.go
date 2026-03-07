package fileutil

import (
	"fmt"
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

	return os.Rename(srcPath, destPath)
}
