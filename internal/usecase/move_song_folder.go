package usecase

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/meta-BE/bms-elsa/internal/domain/fileutil"
	"github.com/meta-BE/bms-elsa/internal/port"
)

// MoveSongFolderResult はフォルダ移動の結果
type MoveSongFolderResult struct {
	DestPath  string
	FileCount int
}

type MoveSongFolderUseCase struct {
	logger port.Logger
}

func NewMoveSongFolderUseCase(logger port.Logger) *MoveSongFolderUseCase {
	return &MoveSongFolderUseCase{logger: logger}
}

// Execute は srcFolderPath を destParentDir/フォルダ名 に移動する。
func (u *MoveSongFolderUseCase) Execute(_ context.Context, srcFolderPath, destParentDir string) (*MoveSongFolderResult, error) {
	folderName := filepath.Base(srcFolderPath)
	destPath := filepath.Join(destParentDir, folderName)

	count, err := fileutil.MoveFolder(srcFolderPath, destPath)
	if err != nil {
		return nil, err
	}

	u.logger.Log(fmt.Sprintf("MOVE %s → %s (%d files)", srcFolderPath, destPath, count))

	return &MoveSongFolderResult{
		DestPath:  destPath,
		FileCount: count,
	}, nil
}
