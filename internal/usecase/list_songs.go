package usecase

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

// ListSongsUseCase は楽曲一覧取得のユースケース
type ListSongsUseCase struct {
	songRepo model.SongRepository
}

func NewListSongsUseCase(songRepo model.SongRepository) *ListSongsUseCase {
	return &ListSongsUseCase{songRepo: songRepo}
}

func (u *ListSongsUseCase) Execute(ctx context.Context, opts model.ListOptions) ([]model.Song, int, error) {
	return u.songRepo.ListSongs(ctx, opts)
}

func (u *ListSongsUseCase) ExecuteAll(ctx context.Context) ([]model.Song, error) {
	return u.songRepo.ListAllSongs(ctx)
}
