package app

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/meta-BE/bms-elsa/internal/usecase"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// DiffImportCandidateDTO はフロントエンドに返す推定結果
type DiffImportCandidateDTO struct {
	FilePath    string  `json:"filePath"`
	FileName    string  `json:"fileName"`
	Title       string  `json:"title"`
	Subtitle    string  `json:"subtitle"`
	Artist      string  `json:"artist"`
	Subartist   string  `json:"subartist"`
	DestFolder  string  `json:"destFolder"`
	Score       float64 `json:"score"`
	MatchMethod string  `json:"matchMethod"`
}

// DiffImportResultDTO はフロントエンドに返す導入結果
type DiffImportResultDTO struct {
	Success int      `json:"success"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors"`
}

type DiffImportHandler struct {
	ctx        context.Context
	estimateUC *usecase.EstimateDiffInstallUseCase
	executeUC  *usecase.ExecuteDiffImportUseCase

	mu         sync.Mutex
	running    bool
	cancelFunc context.CancelFunc
}

func NewDiffImportHandler(
	estimateUC *usecase.EstimateDiffInstallUseCase,
	executeUC *usecase.ExecuteDiffImportUseCase,
) *DiffImportHandler {
	return &DiffImportHandler{
		estimateUC: estimateUC,
		executeUC:  executeUC,
	}
}

func (h *DiffImportHandler) SetContext(ctx context.Context) { h.ctx = ctx }

// isBMSFile はBMS/BME/BMLファイルかどうかを判定する
func isBMSFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".bms" || ext == ".bme" || ext == ".bml"
}

// collectBMSFiles はパスリストからBMSファイルを収集する（フォルダの場合は再帰的に）
func collectBMSFiles(paths []string) []string {
	var result []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if !d.IsDir() && isBMSFile(path) {
					result = append(result, path)
				}
				return nil
			})
		} else if isBMSFile(p) {
			result = append(result, p)
		}
	}
	return result
}

// ParseAndEstimate はD&D時に呼ばれ、パース→推定を一括実行する
func (h *DiffImportHandler) ParseAndEstimate(filePaths []string) ([]DiffImportCandidateDTO, error) {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return nil, nil
	}
	h.running = true
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		h.running = false
		h.cancelFunc = nil
		h.mu.Unlock()
	}()

	ctx, cancel := context.WithCancel(h.ctx)
	h.mu.Lock()
	h.cancelFunc = cancel
	h.mu.Unlock()

	bmsFiles := collectBMSFiles(filePaths)
	total := len(bmsFiles)
	var results []DiffImportCandidateDTO

	for i, fp := range bmsFiles {
		select {
		case <-ctx.Done():
			return results, nil
		default:
		}

		candidate, err := h.estimateUC.EstimateOne(ctx, fp)
		if err != nil {
			// パースエラーはスキップして続行
			wailsRuntime.EventsEmit(h.ctx, "diff-import:progress", map[string]int{
				"current": i + 1, "total": total,
			})
			continue
		}

		results = append(results, DiffImportCandidateDTO{
			FilePath:    candidate.FilePath,
			FileName:    candidate.FileName,
			Title:       candidate.Title,
			Subtitle:    candidate.Subtitle,
			Artist:      candidate.Artist,
			Subartist:   candidate.Subartist,
			DestFolder:  candidate.DestFolder,
			Score:       candidate.Score,
			MatchMethod: candidate.MatchMethod,
		})

		wailsRuntime.EventsEmit(h.ctx, "diff-import:progress", map[string]int{
			"current": i + 1, "total": total,
		})
	}

	wailsRuntime.EventsEmit(h.ctx, "diff-import:done", nil)
	return results, nil
}

// ExecuteImport は確定済み候補のファイル移動を実行する
func (h *DiffImportHandler) ExecuteImport(candidates []DiffImportCandidateDTO) DiffImportResultDTO {
	var requests []usecase.ImportRequest
	for _, c := range candidates {
		if c.DestFolder != "" {
			requests = append(requests, usecase.ImportRequest{
				FilePath:   c.FilePath,
				DestFolder: c.DestFolder,
			})
		}
	}
	result := h.executeUC.Execute(requests)
	return DiffImportResultDTO{
		Success: result.Success,
		Failed:  result.Failed,
		Errors:  result.Errors,
	}
}

// StopEstimate は実行中の推定処理を中断する
func (h *DiffImportHandler) StopEstimate() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancelFunc != nil {
		h.cancelFunc()
	}
}

// IsEstimateRunning は推定処理が実行中かどうかを返す
func (h *DiffImportHandler) IsEstimateRunning() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running
}
