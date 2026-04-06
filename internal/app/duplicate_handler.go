package app

import (
	"context"
	"sync"

	"github.com/meta-BE/bms-elsa/internal/domain/similarity"
	"github.com/meta-BE/bms-elsa/internal/usecase"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// MergeFoldersResultDTO はフロントエンドに返すマージ結果
type MergeFoldersResultDTO struct {
	Success  bool   `json:"success"`
	Moved    int    `json:"moved"`
	Replaced int    `json:"replaced"`
	Skipped  int    `json:"skipped"`
	Errors   int    `json:"errors"`
	ErrorMsg string `json:"errorMsg"`
}

type DuplicateHandler struct {
	ctx            context.Context
	scanDuplicates *usecase.ScanDuplicatesUseCase
	mergeFolders   *usecase.MergeFoldersUseCase

	mu      sync.Mutex
	running bool
	results []similarity.DuplicateGroup
}

func NewDuplicateHandler(
	scanDuplicates *usecase.ScanDuplicatesUseCase,
	mergeFolders *usecase.MergeFoldersUseCase,
) *DuplicateHandler {
	return &DuplicateHandler{
		scanDuplicates: scanDuplicates,
		mergeFolders:   mergeFolders,
	}
}

func (h *DuplicateHandler) SetContext(ctx context.Context) { h.ctx = ctx }

// StartScanDuplicates はバックグラウンドで重複検知スキャンを開始する。二重起動不可。
func (h *DuplicateHandler) StartScanDuplicates() {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return
	}
	h.running = true
	h.mu.Unlock()

	wailsRuntime.EventsEmit(h.ctx, "dup:progress", map[string]int{
		"current": 0,
		"total":   1,
	})

	go func() {
		defer func() {
			h.mu.Lock()
			h.running = false
			h.mu.Unlock()
		}()

		groups, err := h.scanDuplicates.Execute(h.ctx)
		if err != nil {
			wailsRuntime.EventsEmit(h.ctx, "dup:done", map[string]any{
				"groups": 0,
				"error":  err.Error(),
			})
			return
		}

		h.mu.Lock()
		h.results = groups
		h.mu.Unlock()

		wailsRuntime.EventsEmit(h.ctx, "dup:progress", map[string]int{
			"current": 1,
			"total":   1,
		})
		wailsRuntime.EventsEmit(h.ctx, "dup:done", map[string]any{
			"groups": len(groups),
			"error":  "",
		})
	}()
}

// GetDuplicateGroups はキャッシュ済みのスキャン結果を返す
func (h *DuplicateHandler) GetDuplicateGroups() []similarity.DuplicateGroup {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.results
}

// IsDuplicateScanRunning はスキャンが実行中かどうかを返す
func (h *DuplicateHandler) IsDuplicateScanRunning() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running
}

// MergeFolders は srcDir を destDir にマージする
func (h *DuplicateHandler) MergeFolders(srcDir, destDir string) (*MergeFoldersResultDTO, error) {
	result, err := h.mergeFolders.Execute(h.ctx, srcDir, destDir)
	if err != nil {
		return &MergeFoldersResultDTO{Success: false, ErrorMsg: err.Error()}, nil
	}
	return &MergeFoldersResultDTO{
		Success:  result.Errors == 0,
		Moved:    result.Moved,
		Replaced: result.Replaced,
		Skipped:  result.Skipped,
		Errors:   result.Errors,
		ErrorMsg: result.ErrorMsg,
	}, nil
}
