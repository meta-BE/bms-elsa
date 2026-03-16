package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MergeResult はマージ操作の結果
type MergeResult struct {
	Moved    []string // 新規移動したファイル（相対パス）
	Replaced []string // 上書きしたファイル（移動元が新しい）
	Skipped  []string // スキップしたファイル（移動先が新しいor同一）
	Errors   []MergeError
}

// MergeError はファイル単位のエラー
type MergeError struct {
	FileName string
	Err      error
}

// MergeFolders は srcDir 内の全ファイルを destDir に移動し、成功後に srcDir を削除する。
// サブディレクトリも再帰的に処理する。競合時はファイルの作成日時を比較し、新しい方を残す。
// 1ファイルでもエラーがあった場合、srcDirの削除はスキップ。
func MergeFolders(srcDir, destDir string) (*MergeResult, error) {
	srcDir, err := filepath.Abs(srcDir)
	if err != nil {
		return nil, fmt.Errorf("srcDir の絶対パス変換に失敗: %w", err)
	}
	destDir, err = filepath.Abs(destDir)
	if err != nil {
		return nil, fmt.Errorf("destDir の絶対パス変換に失敗: %w", err)
	}

	if err := validateMergePaths(srcDir, destDir); err != nil {
		return nil, err
	}

	result := &MergeResult{}

	// srcDir が空かチェック
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("srcDir の読み込みに失敗: %w", err)
	}
	if len(entries) == 0 {
		os.RemoveAll(srcDir)
		return result, nil
	}

	// 再帰的にファイルを処理（WalkDirでシンボリックリンクを正しく検出）
	err = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // エラーはスキップ
		}
		if d.IsDir() {
			return nil // ディレクトリはスキップ（ファイルのみ処理）
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil // シンボリックリンクはスキップ
		}

		rel, _ := filepath.Rel(srcDir, path)
		destPath := filepath.Join(destDir, rel)

		// 移動先ディレクトリを作成
		destSubDir := filepath.Dir(destPath)
		if err := os.MkdirAll(destSubDir, 0755); err != nil {
			result.Errors = append(result.Errors, MergeError{FileName: rel, Err: err})
			return nil
		}

		action, err := mergeOneFile(path, destPath)
		if err != nil {
			result.Errors = append(result.Errors, MergeError{FileName: rel, Err: err})
			return nil
		}

		switch action {
		case actionMoved:
			result.Moved = append(result.Moved, rel)
		case actionReplaced:
			result.Replaced = append(result.Replaced, rel)
		case actionSkipped:
			result.Skipped = append(result.Skipped, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("srcDir の走査に失敗: %w", err)
	}

	// エラーがなければ srcDir を削除
	if len(result.Errors) == 0 {
		os.RemoveAll(srcDir)
	}

	return result, nil
}

type mergeAction int

const (
	actionMoved mergeAction = iota
	actionReplaced
	actionSkipped
)

func mergeOneFile(srcPath, destPath string) (mergeAction, error) {
	_, err := os.Stat(destPath)
	if os.IsNotExist(err) {
		// 移動先に存在しない → そのまま移動
		if err := copyFile(srcPath, destPath); err != nil {
			return 0, fmt.Errorf("コピー失敗: %w", err)
		}
		os.Remove(srcPath)
		return actionMoved, nil
	}
	if err != nil {
		return 0, err
	}

	// 移動先に存在 → 作成日時を比較
	srcTime, err := fileCreationTime(srcPath)
	if err != nil {
		return 0, fmt.Errorf("移動元の作成日時取得に失敗: %w", err)
	}
	destTime, err := fileCreationTime(destPath)
	if err != nil {
		return 0, fmt.Errorf("移動先の作成日時取得に失敗: %w", err)
	}

	if srcTime.After(destTime) {
		// 移動元の方が新しい → 上書き
		if err := copyFile(srcPath, destPath); err != nil {
			return 0, fmt.Errorf("上書きコピー失敗: %w", err)
		}
		os.Remove(srcPath)
		return actionReplaced, nil
	}

	// 移動先の方が新しい or 同一 → スキップ（移動元は削除）
	os.Remove(srcPath)
	return actionSkipped, nil
}

func validateMergePaths(srcDir, destDir string) error {
	if srcDir == destDir {
		return fmt.Errorf("移動元と移動先が同じです: %s", srcDir)
	}

	// 親子関係チェック
	srcWithSep := srcDir + string(filepath.Separator)
	destWithSep := destDir + string(filepath.Separator)
	if strings.HasPrefix(destWithSep, srcWithSep) {
		return fmt.Errorf("移動先が移動元のサブディレクトリです: %s → %s", srcDir, destDir)
	}
	if strings.HasPrefix(srcWithSep, destWithSep) {
		return fmt.Errorf("移動元が移動先のサブディレクトリです: %s → %s", srcDir, destDir)
	}

	if _, err := os.Stat(srcDir); err != nil {
		return fmt.Errorf("移動元フォルダが存在しません: %s (%w)", srcDir, err)
	}
	if _, err := os.Stat(destDir); err != nil {
		return fmt.Errorf("移動先フォルダが存在しません: %s (%w)", destDir, err)
	}

	return nil
}
