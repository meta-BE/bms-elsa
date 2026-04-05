package usecase

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/domain/similarity"
)

const defaultThreshold = 60

type ScanDuplicatesUseCase struct {
	songRepo model.SongRepository
}

func NewScanDuplicatesUseCase(songRepo model.SongRepository) *ScanDuplicatesUseCase {
	return &ScanDuplicatesUseCase{songRepo: songRepo}
}

func (u *ScanDuplicatesUseCase) Execute(ctx context.Context) ([]similarity.DuplicateGroup, error) {
	md5Pairs, err := u.songRepo.ListMD5DuplicateFolders(ctx)
	if err != nil {
		return nil, err
	}

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

	folderPairs := make([]similarity.FolderPair, len(md5Pairs))
	for i, p := range md5Pairs {
		folderPairs[i] = similarity.FolderPair{FolderA: p.FolderA, FolderB: p.FolderB}
	}

	return similarity.FindDuplicateGroups(songs, folderPairs, defaultThreshold), nil
}
