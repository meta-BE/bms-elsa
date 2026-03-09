package usecase

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/domain/similarity"
)

type ScanDuplicatesUseCase struct {
	songRepo model.SongRepository
}

func NewScanDuplicatesUseCase(songRepo model.SongRepository) *ScanDuplicatesUseCase {
	return &ScanDuplicatesUseCase{songRepo: songRepo}
}

func (u *ScanDuplicatesUseCase) Execute(ctx context.Context) ([]similarity.DuplicateGroup, error) {
	groups, err := u.songRepo.ListSongGroupsForDuplicateScan(ctx)
	if err != nil {
		return nil, err
	}

	songs := make([]similarity.SongInfo, len(groups))
	for i, g := range groups {
		songs[i] = similarity.SongInfo{
			FolderHash: g.FolderHash,
			Title:      g.Title,
			Artist:     g.Artist,
			Genre:      g.Genre,
			MinBPM:     g.MinBPM,
			MaxBPM:     g.MaxBPM,
			ChartCount: g.ChartCount,
			Path:       g.Path,
			WavMinHash: g.WavMinHash,
		}
	}

	return similarity.FindDuplicateGroups(songs, 60), nil
}
