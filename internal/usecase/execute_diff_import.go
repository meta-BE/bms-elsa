package usecase

import (
	"fmt"

	"github.com/meta-BE/bms-elsa/internal/domain/fileutil"
	"github.com/meta-BE/bms-elsa/internal/port"
)

// ImportRequest はファイル移動リクエスト
type ImportRequest struct {
	FilePath   string
	DestFolder string
}

// ImportResult はファイル移動結果
type ImportResult struct {
	Success int
	Failed  int
	Errors  []string
}

type ExecuteDiffImportUseCase struct {
	logger  port.Logger
	fileLog bool
}

func NewExecuteDiffImportUseCase(logger port.Logger, fileLog bool) *ExecuteDiffImportUseCase {
	return &ExecuteDiffImportUseCase{logger: logger, fileLog: fileLog}
}

// Execute は確定済み候補のファイル移動を実行する
func (u *ExecuteDiffImportUseCase) Execute(requests []ImportRequest) ImportResult {
	var result ImportResult
	destCounts := map[string]int{}

	for _, req := range requests {
		if err := fileutil.MoveFileToFolder(req.FilePath, req.DestFolder); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, err.Error())
			if u.fileLog {
				u.logger.Log(fmt.Sprintf("  ! MOVE %s → %s: %s", req.FilePath, req.DestFolder, err))
			}
		} else {
			result.Success++
			destCounts[req.DestFolder]++
			if u.fileLog {
				u.logger.Log(fmt.Sprintf("  MOVE %s → %s", req.FilePath, req.DestFolder))
			}
		}
	}

	if !u.fileLog {
		for dest, count := range destCounts {
			u.logger.Log(fmt.Sprintf("IMPORT %d files → %s (success:%d, failed:%d)",
				count, dest, count, 0))
		}
		if result.Failed > 0 {
			u.logger.Log(fmt.Sprintf("IMPORT failed: %d files", result.Failed))
		}
	}

	return result
}
