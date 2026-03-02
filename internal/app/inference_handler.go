package app

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/app/dto"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
)

type InferenceHandler struct {
	ctx       context.Context
	inferMeta *usecase.InferSongMetaUseCase
	metaRepo  model.MetaRepository
}

func NewInferenceHandler(inferMeta *usecase.InferSongMetaUseCase, metaRepo model.MetaRepository) *InferenceHandler {
	return &InferenceHandler{inferMeta: inferMeta, metaRepo: metaRepo}
}

func (h *InferenceHandler) SetContext(ctx context.Context) {
	h.ctx = ctx
}

func (h *InferenceHandler) RunAutoInference() (*dto.InferenceResultDTO, error) {
	result, err := h.inferMeta.RunAutoInference(h.ctx)
	if err != nil {
		return nil, err
	}
	unmatchedDTOs := make([]dto.SongIRURLsDTO, len(result.UnmatchedSongs))
	for i, s := range result.UnmatchedSongs {
		unmatchedDTOs[i] = dto.SongIRURLsDTO{
			FolderHash: s.FolderHash,
			Title:      s.Title,
			Artist:     s.Artist,
			Genre:      s.Genre,
			BodyURLs:   s.BodyURLs,
			ChartCount: s.ChartCount,
			IRCount:    s.IRCount,
		}
	}
	return &dto.InferenceResultDTO{
		AutoSetCount:   result.AutoSetCount,
		UnmatchedSongs: unmatchedDTOs,
		NoIRCount:      result.NoIRCount,
	}, nil
}

func (h *InferenceHandler) ListEventMappings() ([]dto.EventMappingDTO, error) {
	mappings, err := h.metaRepo.ListEventMappings(h.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.EventMappingDTO, len(mappings))
	for i, m := range mappings {
		result[i] = dto.EventMappingDTO{
			ID:          m.ID,
			URLPattern:  m.URLPattern,
			EventName:   m.EventName,
			ReleaseYear: m.ReleaseYear,
		}
	}
	return result, nil
}

func (h *InferenceHandler) UpsertEventMapping(urlPattern, eventName string, releaseYear int) error {
	return h.metaRepo.UpsertEventMapping(h.ctx, model.EventMapping{
		URLPattern:  urlPattern,
		EventName:   eventName,
		ReleaseYear: releaseYear,
	})
}

func (h *InferenceHandler) DeleteEventMapping(id int) error {
	return h.metaRepo.DeleteEventMapping(h.ctx, id)
}
