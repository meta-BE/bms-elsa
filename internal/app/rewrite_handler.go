package app

import (
	"context"

	"github.com/meta-BE/bms-elsa/internal/app/dto"
	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

type RewriteHandler struct {
	ctx      context.Context
	metaRepo model.MetaRepository
}

func NewRewriteHandler(metaRepo model.MetaRepository) *RewriteHandler {
	return &RewriteHandler{metaRepo: metaRepo}
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
		ID: id, RuleType: ruleType, Pattern: pattern, Replacement: replacement, Priority: priority,
	})
}

func (h *RewriteHandler) DeleteRewriteRule(id int) error {
	return h.metaRepo.DeleteRewriteRule(h.ctx, id)
}
