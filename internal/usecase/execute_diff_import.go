package usecase

import "github.com/meta-BE/bms-elsa/internal/domain/fileutil"

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

type ExecuteDiffImportUseCase struct{}

func NewExecuteDiffImportUseCase() *ExecuteDiffImportUseCase {
	return &ExecuteDiffImportUseCase{}
}

// Execute は確定済み候補のファイル移動を実行する
func (u *ExecuteDiffImportUseCase) Execute(requests []ImportRequest) ImportResult {
	var result ImportResult
	for _, req := range requests {
		if err := fileutil.MoveFileToFolder(req.FilePath, req.DestFolder); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, err.Error())
		} else {
			result.Success++
		}
	}
	return result
}
