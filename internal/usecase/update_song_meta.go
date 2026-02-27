package usecase

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

// UpdateSongMetaUseCase は楽曲メタデータ更新のユースケース
type UpdateSongMetaUseCase struct {
	metaRepo model.MetaRepository
}

func NewUpdateSongMetaUseCase(metaRepo model.MetaRepository) *UpdateSongMetaUseCase {
	return &UpdateSongMetaUseCase{metaRepo: metaRepo}
}

func (u *UpdateSongMetaUseCase) Execute(ctx context.Context, meta model.SongMeta) error {
	return u.metaRepo.UpsertSongMeta(ctx, meta)
}
