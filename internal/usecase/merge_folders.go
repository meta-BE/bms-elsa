package usecase

import (
	"context"
	"fmt"

	"github.com/meta-BE/bms-elsa/internal/domain/fileutil"
	"github.com/meta-BE/bms-elsa/internal/port"
)

// MergeFoldersResult はフロントエンド向けのマージ結果
type MergeFoldersResult struct {
	Moved    int
	Replaced int
	Skipped  int
	Errors   int
	ErrorMsg string
}

type MergeFoldersUseCase struct {
	logger  port.Logger
	fileLog bool
}

func NewMergeFoldersUseCase(logger port.Logger, fileLog bool) *MergeFoldersUseCase {
	return &MergeFoldersUseCase{logger: logger, fileLog: fileLog}
}

func (u *MergeFoldersUseCase) Execute(_ context.Context, srcDir, destDir string) (*MergeFoldersResult, error) {
	mergeResult, err := fileutil.MergeFolders(srcDir, destDir)
	if err != nil {
		return nil, err
	}

	u.writeLog(srcDir, destDir, mergeResult)

	result := &MergeFoldersResult{
		Moved:    len(mergeResult.Moved),
		Replaced: len(mergeResult.Replaced),
		Skipped:  len(mergeResult.Skipped),
		Errors:   len(mergeResult.Errors),
	}
	if len(mergeResult.Errors) > 0 {
		result.ErrorMsg = mergeResult.Errors[0].Err.Error()
	}
	return result, nil
}

func (u *MergeFoldersUseCase) writeLog(srcDir, destDir string, r *fileutil.MergeResult) {
	if u.fileLog {
		u.logger.Log(fmt.Sprintf("MERGE %s → %s", srcDir, destDir))
		for _, f := range r.Moved {
			u.logger.Log(fmt.Sprintf("  + %s", f))
		}
		for _, f := range r.Replaced {
			u.logger.Log(fmt.Sprintf("  > %s", f))
		}
		for _, f := range r.Skipped {
			u.logger.Log(fmt.Sprintf("  = %s", f))
		}
		for _, e := range r.Errors {
			u.logger.Log(fmt.Sprintf("  ! %s: %s", e.FileName, e.Err))
		}
	} else {
		u.logger.Log(fmt.Sprintf("MERGE %s → %s (moved:%d, replaced:%d, skipped:%d, errors:%d)",
			srcDir, destDir,
			len(r.Moved), len(r.Replaced), len(r.Skipped), len(r.Errors)))
	}
}
