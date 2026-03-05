package app

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
	"github.com/meta-BE/bms-elsa/internal/app/dto"
)

type ChartHandler struct {
	ctx        context.Context
	songReader *persistence.SongdataReader
	elsaRepo   *persistence.ElsaRepository
}

func NewChartHandler(
	songReader *persistence.SongdataReader,
	elsaRepo *persistence.ElsaRepository,
) *ChartHandler {
	return &ChartHandler{
		songReader: songReader,
		elsaRepo:   elsaRepo,
	}
}

func (h *ChartHandler) SetContext(ctx context.Context) { h.ctx = ctx }

func (h *ChartHandler) ListCharts() ([]dto.ChartListItemDTO, error) {
	charts, err := h.songReader.ListAllCharts(h.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.ChartListItemDTO, len(charts))
	for i, c := range charts {
		result[i] = dto.ChartListItemDTO{
			MD5:        c.MD5,
			Title:      c.Title,
			Subtitle:   c.Subtitle,
			Artist:     c.Artist,
			SubArtist:  c.SubArtist,
			Genre:      c.Genre,
			MinBPM:     c.MinBPM,
			MaxBPM:     c.MaxBPM,
			Difficulty: c.Difficulty,
			HasIRMeta:  c.HasIRMeta,
		}
		if c.EventName != nil {
			result[i].EventName = *c.EventName
		}
		if c.ReleaseYear != nil {
			result[i].ReleaseYear = *c.ReleaseYear
		}
	}
	return result, nil
}

func (h *ChartHandler) GetChartDetailByMD5(md5 string) (*dto.ChartDTO, error) {
	chart, err := h.songReader.GetChartByMD5(h.ctx, md5)
	if err != nil {
		return nil, err
	}
	if chart == nil {
		return nil, nil
	}
	result := dto.ChartToDTO(*chart)
	return &result, nil
}

// GetChartMetaByMD5 はchart_metaテーブルからIR情報のみを取得する（未導入譜面用）
func (h *ChartHandler) GetChartMetaByMD5(md5 string) (*dto.ChartIRMetaDTO, error) {
	meta, err := h.elsaRepo.GetChartMeta(h.ctx, md5)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, nil
	}
	result := dto.ChartIRMetaToDTO(*meta)
	return &result, nil
}
