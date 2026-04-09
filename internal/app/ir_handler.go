package app

import (
	"context"
	"strings"
	"sync"

	"github.com/meta-BE/bms-elsa/internal/app/dto"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type IRHandler struct {
	ctx         context.Context
	lookupIR    *usecase.LookupIRUseCase
	bulkFetchIR *usecase.BulkFetchIRUseCase
	metaRepo    model.MetaRepository

	mu         sync.Mutex
	running    bool
	cancelFunc context.CancelFunc
}

func NewIRHandler(
	li *usecase.LookupIRUseCase,
	bf *usecase.BulkFetchIRUseCase,
	mr model.MetaRepository,
) *IRHandler {
	return &IRHandler{lookupIR: li, bulkFetchIR: bf, metaRepo: mr}
}

func (h *IRHandler) SetContext(ctx context.Context) { h.ctx = ctx }

func (h *IRHandler) LookupByMD5(md5, sha256 string) (*dto.ChartDTO, error) {
	resp, err := h.lookupIR.Execute(h.ctx, md5, sha256)
	if err != nil {
		return nil, err
	}
	if !resp.Registered {
		return nil, nil
	}
	result := &dto.ChartDTO{
		MD5:          md5,
		SHA256:       sha256,
		HasIRMeta:    true,
		LR2IRTags:    strings.Join(resp.Tags, ","),
		LR2IRBodyURL: resp.BodyURL,
		LR2IRDiffURL: resp.DiffURL,
		LR2IRNotes:   resp.Notes,
	}
	return result, nil
}

// startBulkFetchWith はIR一括取得の共通処理。fetchMD5s で取得対象を差し替える。
func (h *IRHandler) startBulkFetchWith(fetchMD5s func(ctx context.Context) ([]string, error)) error {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return nil
	}
	h.running = true
	h.mu.Unlock()

	md5s, err := fetchMD5s(h.ctx)
	if err != nil {
		h.mu.Lock()
		h.running = false
		h.mu.Unlock()
		return err
	}

	ctx, cancel := context.WithCancel(h.ctx)
	h.mu.Lock()
	h.cancelFunc = cancel
	h.mu.Unlock()

	go func() {
		defer func() {
			h.mu.Lock()
			h.running = false
			h.cancelFunc = nil
			h.mu.Unlock()
		}()

		result, err := h.bulkFetchIR.Execute(ctx, md5s, func(p usecase.BulkFetchProgress) {
			wailsRuntime.EventsEmit(h.ctx, "ir:progress", map[string]int{
				"current": p.Current,
				"total":   p.Total,
			})
		})

		doneData := map[string]any{
			"cancelled": false,
			"error":     "",
		}
		if err != nil {
			doneData["error"] = err.Error()
		}
		if result != nil {
			doneData["total"] = result.Total
			doneData["fetched"] = result.Fetched
			doneData["notFound"] = result.NotFound
			doneData["failed"] = result.Failed
			doneData["cancelled"] = result.Cancelled
		}
		wailsRuntime.EventsEmit(h.ctx, "ir:done", doneData)
	}()

	return nil
}

// StartBulkFetch はIR一括取得をバックグラウンドで開始する。二重起動不可。
func (h *IRHandler) StartBulkFetch() error {
	return h.startBulkFetchWith(h.metaRepo.ListUnfetchedChartMD5s)
}

// StartDifficultyTableBulkFetch は難易度表エントリのIR一括取得をバックグラウンドで開始する。二重起動不可。
func (h *IRHandler) StartDifficultyTableBulkFetch(tableID int) error {
	return h.startBulkFetchWith(func(ctx context.Context) ([]string, error) {
		return h.metaRepo.ListUnfetchedDTEntryMD5s(ctx, tableID)
	})
}

// StopBulkFetch は実行中のIR一括取得を中断する
func (h *IRHandler) StopBulkFetch() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancelFunc != nil {
		h.cancelFunc()
	}
}

// IsBulkFetchRunning は一括取得が実行中かどうかを返す
func (h *IRHandler) IsBulkFetchRunning() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running
}
