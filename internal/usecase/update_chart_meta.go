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

func (u *UpdateChartMetaUseCase) Execute(ctx context.Context, md5, sha256, workingBodyURL, workingDiffURL string) error {
	return u.metaRepo.UpdateWorkingURLs(ctx, md5, sha256, workingBodyURL, workingDiffURL)
}
