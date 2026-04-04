package app

import (
	"context"
	"sync"

	"github.com/meta-BE/bms-elsa/internal/app/dto"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
	"github.com/meta-BE/bms-elsa/internal/usecase"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type RewriteHandler struct {
	ctx              context.Context
	inferWorkingURLs *usecase.InferWorkingURLUseCase
	metaRepo         model.MetaRepository

	mu      sync.Mutex
	running bool
}

func NewRewriteHandler(inferWorkingURLs *usecase.InferWorkingURLUseCase, metaRepo model.MetaRepository) *RewriteHandler {
	return &RewriteHandler{inferWorkingURLs: inferWorkingURLs, metaRepo: metaRepo}
}

func (h *RewriteHandler) SetContext(ctx context.Context) {
	h.ctx = ctx
}

func (h *RewriteHandler) ListRewriteRules() ([]dto.RewriteRuleDTO, error) {
	rules, err := h.metaRepo.ListRewriteRules(h.ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.RewriteRuleDTO, len(rules))
	for i, r := range rules {
		result[i] = dto.RewriteRuleDTO{
			ID:          r.ID,
			RuleType:    r.RuleType,
			Pattern:     r.Pattern,
			Replacement: r.Replacement,
			Priority:    r.Priority,
		}
	}
	return result, nil
}

func (h *RewriteHandler) UpsertRewriteRule(id int, ruleType, pattern, replacement string, priority int) error {
	return h.metaRepo.UpsertRewriteRule(h.ctx, model.RewriteRule{
		ID:          id,
		RuleType:    ruleType,
		Pattern:     pattern,
		Replacement: replacement,
		Priority:    priority,
	})
}

func (h *RewriteHandler) DeleteRewriteRule(id int) error {
	return h.metaRepo.DeleteRewriteRule(h.ctx, id)
}

func (h *RewriteHandler) InferWorkingURLs() (*dto.InferWorkingURLResultDTO, error) {
	result, err := h.inferWorkingURLs.Execute(h.ctx)
	if err != nil {
		return nil, err
	}
	return &dto.InferWorkingURLResultDTO{
		Applied: result.Applied,
		Skipped: result.Skipped,
		Total:   result.Total,
	}, nil
}

// StartInferWorkingURLs は動作URL推定をバックグラウンドで実行する
func (h *RewriteHandler) StartInferWorkingURLs() {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return
	}
	h.running = true
	h.mu.Unlock()

	go func() {
		defer func() {
			h.mu.Lock()
			h.running = false
			h.mu.Unlock()
		}()

		result, err := h.inferWorkingURLs.Execute(h.ctx)

		doneData := map[string]any{
			"error": "",
		}
		if err != nil {
			doneData["error"] = err.Error()
		} else {
			doneData["applied"] = result.Applied
			doneData["skipped"] = result.Skipped
			doneData["total"] = result.Total
		}
		wailsRuntime.EventsEmit(h.ctx, "rewrite:done", doneData)
	}()
}

// IsInferring は動作URL推定が実行中かどうかを返す
func (h *RewriteHandler) IsInferring() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.running
}
