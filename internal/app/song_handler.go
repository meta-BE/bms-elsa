package app

import (
	"context"
	"fmt"

	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
	"github.com/meta-BE/bms-elsa/internal/app/dto"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

type SongHandler struct {
	ctx            context.Context
	listSongs      *usecase.ListSongsUseCase
	getSongDetail  *usecase.GetSongDetailUseCase
	updateMeta     *usecase.UpdateSongMetaUseCase
	moveSongFolder *usecase.MoveSongFolderUseCase
}

func NewSongHandler(ls *usecase.ListSongsUseCase, gsd *usecase.GetSongDetailUseCase, um *usecase.UpdateSongMetaUseCase, msf *usecase.MoveSongFolderUseCase) *SongHandler {
	return &SongHandler{listSongs: ls, getSongDetail: gsd, updateMeta: um, moveSongFolder: msf}
}

func (h *SongHandler) SetContext(ctx context.Context) { h.ctx = ctx }

func (h *SongHandler) ListSongs(page, pageSize int, sortBy string, sortDesc bool, search string) (*dto.SongListDTO, error) {
	opts := model.ListOptions{Page: page, PageSize: pageSize, SortBy: sortBy, SortDesc: sortDesc, Search: search}
	songs, total, err := h.listSongs.Execute(h.ctx, opts)
	if err != nil {
		return nil, err
	}
	rows := make([]dto.SongRowDTO, len(songs))
	for i, s := range songs {
		rows[i] = dto.SongToRowDTO(s)
	}
	return &dto.SongListDTO{Songs: rows, TotalCount: total, Page: page, PageSize: pageSize}, nil
}

func (h *SongHandler) ListAllSongs() ([]dto.SongRowDTO, error) {
	songs, err := h.listSongs.ExecuteAll(h.ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]dto.SongRowDTO, len(songs))
	for i, s := range songs {
		rows[i] = dto.SongToRowDTO(s)
	}
	return rows, nil
}

func (h *SongHandler) GetSongDetail(folderHash string) (*dto.SongDetailDTO, error) {
	song, err := h.getSongDetail.Execute(h.ctx, folderHash)
	if err != nil {
		return nil, err
	}
	if song == nil {
		return nil, nil
	}
	result := dto.SongToDetailDTO(*song)
	return &result, nil
}

func (h *SongHandler) UpdateSongMeta(folderHash string, releaseYear *int, eventID *string) error {
	return h.updateMeta.Execute(h.ctx, model.SongMeta{
		FolderHash:  folderHash,
		ReleaseYear: releaseYear,
		EventID:     eventID,
	})
}

func (h *SongHandler) MoveSongFolder(folderHash, destParentDir string) (*dto.MoveSongFolderResultDTO, error) {
	song, err := h.getSongDetail.Execute(h.ctx, folderHash)
	if err != nil {
		return nil, err
	}
	if song == nil || len(song.Charts) == 0 {
		return nil, fmt.Errorf("楽曲が見つかりません: %s", folderHash)
	}

	// チャートのファイルパスからフォルダパスを導出（Windows\区切り対応）
	srcFolderPath := persistence.ParentDirOf(song.Charts[0].Path)

	result, err := h.moveSongFolder.Execute(h.ctx, srcFolderPath, destParentDir)
	if err != nil {
		return nil, err
	}

	return &dto.MoveSongFolderResultDTO{
		DestPath:  result.DestPath,
		FileCount: result.FileCount,
	}, nil
}
