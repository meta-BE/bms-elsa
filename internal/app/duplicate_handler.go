package app

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/domain/similarity"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

type DuplicateHandler struct {
	ctx            context.Context
	scanDuplicates *usecase.ScanDuplicatesUseCase
}

func NewDuplicateHandler(scanDuplicates *usecase.ScanDuplicatesUseCase) *DuplicateHandler {
	return &DuplicateHandler{scanDuplicates: scanDuplicates}
}

func (h *DuplicateHandler) SetContext(ctx context.Context) { h.ctx = ctx }

func (h *DuplicateHandler) ScanDuplicates() ([]similarity.DuplicateGroup, error) {
	return h.scanDuplicates.Execute(h.ctx)
}
