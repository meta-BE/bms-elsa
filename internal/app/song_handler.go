package app

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/app/dto"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

type SongHandler struct {
	ctx           context.Context
	listSongs     *usecase.ListSongsUseCase
	getSongDetail *usecase.GetSongDetailUseCase
	updateMeta    *usecase.UpdateSongMetaUseCase
}

func NewSongHandler(ls *usecase.ListSongsUseCase, gsd *usecase.GetSongDetailUseCase, um *usecase.UpdateSongMetaUseCase) *SongHandler {
	return &SongHandler{listSongs: ls, getSongDetail: gsd, updateMeta: um}
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

func (h *SongHandler) UpdateSongMeta(folderHash string, releaseYear *int, eventName *string) error {
	return h.updateMeta.Execute(h.ctx, model.SongMeta{
		FolderHash: folderHash, ReleaseYear: releaseYear, EventName: eventName,
	})
}
