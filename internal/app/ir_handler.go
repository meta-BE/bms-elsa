package app

import (
	"context"
	"strings"

	"github.com/meta-BE/bms-elsa/internal/app/dto"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

type IRHandler struct {
	ctx         context.Context
	lookupIR    *usecase.LookupIRUseCase
	updateChart *usecase.UpdateChartMetaUseCase
}

func NewIRHandler(li *usecase.LookupIRUseCase, uc *usecase.UpdateChartMetaUseCase) *IRHandler {
	return &IRHandler{lookupIR: li, updateChart: uc}
}

func (h *IRHandler) SetContext(ctx context.Context) { h.ctx = ctx }

func (h *IRHandler) LookupByMD5(md5, sha256 string) (*dto.ChartDTO, error) {
	resp, err := h.lookupIR.Execute(h.ctx, md5, sha256)
	if err != nil {
		return nil, err
	}
	if !resp.Registered {
		return nil, nil // 未登録
	}
	result := &dto.ChartDTO{
		MD5:          md5,
		SHA256:       sha256,
		HasIRMeta:    true,
		LR2IRTags:    strings.Join(resp.Tags, ","),
		LR2IRBodyURL: resp.BodyURL,
		LR2IRDiffURL: resp.DiffURL,
		LR2IRNotes:   resp.Notes,
	}
	return result, nil
}

func (h *IRHandler) UpdateChartMeta(md5, sha256, workingBodyURL, workingDiffURL string) error {
	meta := model.ChartIRMeta{
		MD5:            md5,
		SHA256:         sha256,
		WorkingBodyURL: workingBodyURL,
		WorkingDiffURL: workingDiffURL,
	}
	return h.updateChart.Execute(h.ctx, meta)
}
