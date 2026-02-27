package usecase

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

// GetSongDetailUseCase は楽曲詳細取得のユースケース
type GetSongDetailUseCase struct {
	songRepo model.SongRepository
}

func NewGetSongDetailUseCase(songRepo model.SongRepository) *GetSongDetailUseCase {
	return &GetSongDetailUseCase{songRepo: songRepo}
}

func (u *GetSongDetailUseCase) Execute(ctx context.Context, folderHash string) (*model.Song, error) {
	return u.songRepo.GetSongByFolder(ctx, folderHash)
}
