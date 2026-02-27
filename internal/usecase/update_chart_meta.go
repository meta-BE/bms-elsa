package usecase

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

// UpdateChartMetaUseCase は譜面IRメタデータ更新のユースケース
type UpdateChartMetaUseCase struct {
	metaRepo model.MetaRepository
}

func NewUpdateChartMetaUseCase(metaRepo model.MetaRepository) *UpdateChartMetaUseCase {
	return &UpdateChartMetaUseCase{metaRepo: metaRepo}
}

func (u *UpdateChartMetaUseCase) Execute(ctx context.Context, meta model.ChartIRMeta) error {
	return u.metaRepo.UpsertChartMeta(ctx, meta)
}
