package app

import (
	"context"
	"sync"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ScanHandler struct {
	ctx         context.Context
	metaRepo    model.MetaRepository
	scanMinHash *usecase.ScanMinHashUseCase

	mu         sync.Mutex
	running    bool
	cancelFunc context.CancelFunc
}

func NewScanHandler(metaRepo model.MetaRepository, scanMinHash *usecase.ScanMinHashUseCase) *ScanHandler {
	return &ScanHandler{metaRepo: metaRepo, scanMinHash: scanMinHash}
}

func (h *ScanHandler) SetContext(ctx context.Context) { h.ctx = ctx }

// StartMinHashScan はMinHash一括計算をバックグラウンドで開始する。二重起動不可。
func (h *ScanHandler) StartMinHashScan(onDone func()) error {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return nil
	}
	h.running = true
	h.mu.Unlock()

	targets, err := h.metaRepo.ListChartsWithoutMinhash(h.ctx)
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

		result := h.scanMinHash.Execute(ctx, targets, func(p usecase.ScanMinHashProgress) {
			wailsRuntime.EventsEmit(h.ctx, "scan:progress", map[string]int{
				"current": p.Current,
				"total":   p.Total,
			})
		})

		wailsRuntime.EventsEmit(h.ctx, "scan:done", map[string]any{
			"total":     result.Total,
			"computed":  result.Computed,
			"skipped":   result.Skipped,
			"failed":    result.Failed,
			"cancelled": result.Cancelled,
		})

		if onDone != nil {
			onDone()
		}
	}()

	return nil
}

// StopMinHashScan は実行中のMinHashスキャンを中断する
func (h *ScanHandler) StopMinHashScan() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancelFunc != nil {
		h.cancelFunc()
	}
}

// IsMinHashScanRunning はMinHashスキャンが実行中かどうかを返す
func (h *ScanHandler) IsMinHashScanRunning() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running
}
