package app

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/meta-BE/bms-elsa/internal/adapter/persistence"
	"github.com/meta-BE/bms-elsa/internal/domain/bms"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ScanHandler struct {
	ctx      context.Context
	elsaRepo *persistence.ElsaRepository

	mu         sync.Mutex
	running    bool
	cancelFunc context.CancelFunc
}

func NewScanHandler(elsaRepo *persistence.ElsaRepository) *ScanHandler {
	return &ScanHandler{elsaRepo: elsaRepo}
}

func (h *ScanHandler) SetContext(ctx context.Context) { h.ctx = ctx }

// StartMinHashScan はMinHash一括計算をバックグラウンドで開始する。二重起動不可。
func (h *ScanHandler) StartMinHashScan() error {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return nil
	}
	h.running = true
	h.mu.Unlock()

	targets, err := h.elsaRepo.ListChartsWithoutMinhash(h.ctx)
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

		total := len(targets)
		computed := 0
		skipped := 0
		failed := 0
		cancelled := false

		for i, tgt := range targets {
			select {
			case <-ctx.Done():
				cancelled = true
				goto done
			default:
			}

			// ファイル存在チェック
			if _, err := os.Stat(tgt.Path); err != nil {
				skipped++
				wailsRuntime.EventsEmit(h.ctx, "scan:progress", map[string]int{
					"current": i + 1, "total": total,
				})
				continue
			}

			// BMSパース
			wavFiles, err := bms.ParseWAVFiles(tgt.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "scan: parse error %s: %v\n", tgt.Path, err)
				failed++
				wailsRuntime.EventsEmit(h.ctx, "scan:progress", map[string]int{
					"current": i + 1, "total": total,
				})
				continue
			}

			// MinHash計算・保存
			sig := bms.ComputeMinHash(wavFiles)
			if err := h.elsaRepo.UpdateWavMinhash(h.ctx, tgt.MD5, sig.Bytes()); err != nil {
				fmt.Fprintf(os.Stderr, "scan: db error %s: %v\n", tgt.MD5, err)
				failed++
				wailsRuntime.EventsEmit(h.ctx, "scan:progress", map[string]int{
					"current": i + 1, "total": total,
				})
				continue
			}

			computed++
			wailsRuntime.EventsEmit(h.ctx, "scan:progress", map[string]int{
				"current": i + 1, "total": total,
			})
		}

	done:
		wailsRuntime.EventsEmit(h.ctx, "scan:done", map[string]interface{}{
			"total":     total,
			"computed":  computed,
			"skipped":   skipped,
			"failed":    failed,
			"cancelled": cancelled,
		})
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
