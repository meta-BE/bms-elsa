package app

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/domain/similarity"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

// MergeFoldersResultDTO はフロントエンドに返すマージ結果
type MergeFoldersResultDTO struct {
	Success  bool   `json:"success"`
	Moved    int    `json:"moved"`
	Replaced int    `json:"replaced"`
	Skipped  int    `json:"skipped"`
	Errors   int    `json:"errors"`
	ErrorMsg string `json:"errorMsg"`
}

type DuplicateHandler struct {
	ctx            context.Context
	scanDuplicates *usecase.ScanDuplicatesUseCase
	mergeFolders   *usecase.MergeFoldersUseCase
}

func NewDuplicateHandler(
	scanDuplicates *usecase.ScanDuplicatesUseCase,
	mergeFolders *usecase.MergeFoldersUseCase,
) *DuplicateHandler {
	return &DuplicateHandler{
		scanDuplicates: scanDuplicates,
		mergeFolders:   mergeFolders,
	}
}

func (h *DuplicateHandler) SetContext(ctx context.Context) { h.ctx = ctx }

func (h *DuplicateHandler) ScanDuplicates() ([]similarity.DuplicateGroup, error) {
	return h.scanDuplicates.Execute(h.ctx)
}

// MergeFolders は srcDir を destDir にマージする
func (h *DuplicateHandler) MergeFolders(srcDir, destDir string) (*MergeFoldersResultDTO, error) {
	result, err := h.mergeFolders.Execute(h.ctx, srcDir, destDir)
	if err != nil {
		return &MergeFoldersResultDTO{Success: false, ErrorMsg: err.Error()}, nil
	}
	return &MergeFoldersResultDTO{
		Success:  result.Errors == 0,
		Moved:    result.Moved,
		Replaced: result.Replaced,
		Skipped:  result.Skipped,
		Errors:   result.Errors,
		ErrorMsg: result.ErrorMsg,
	}, nil
}
