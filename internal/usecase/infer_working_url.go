package usecase

import (
	"context"
	"regexp"
	"strings"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

// InferWorkingURLResult は動作URL推定の結果
type InferWorkingURLResult struct {
	Applied int
	Skipped int
	Total   int
}

// InferWorkingURLUseCase は書き換えルールによる動作URL自動推定
type InferWorkingURLUseCase struct {
	metaRepo model.MetaRepository
}

func NewInferWorkingURLUseCase(metaRepo model.MetaRepository) *InferWorkingURLUseCase {
	return &InferWorkingURLUseCase{metaRepo: metaRepo}
}

// InferWorkingURLProgress は進捗情報
type InferWorkingURLProgress struct {
	Current int
	Total   int
}

func (u *InferWorkingURLUseCase) Execute(ctx context.Context, onProgress func(InferWorkingURLProgress)) (*InferWorkingURLResult, error) {
	rules, err := u.metaRepo.ListRewriteRules(ctx)
	if err != nil {
		return nil, err
	}

	charts, err := u.metaRepo.ListChartsForWorkingURLInference(ctx)
	if err != nil {
		return nil, err
	}

	result := &InferWorkingURLResult{Total: len(charts)}

	for i, c := range charts {
		bodyURL := applyRewriteRules(c.LR2IRBodyURL, rules)
		diffURL := applyRewriteRules(c.LR2IRDiffURL, rules)

		if bodyURL == "" && diffURL == "" {
			result.Skipped++
		} else {
			if err := u.metaRepo.UpdateWorkingURLs(ctx, c.MD5, bodyURL, diffURL); err != nil {
				return nil, err
			}
			result.Applied++
		}

		if onProgress != nil {
			onProgress(InferWorkingURLProgress{Current: i + 1, Total: len(charts)})
		}
	}

	return result, nil
}

// applyRewriteRules はルールリスト（priority降順を前提）を順に適用し、
// 最初にマッチしたルールの結果を返す。マッチなしなら空文字を返す。
func applyRewriteRules(url string, rules []model.RewriteRule) string {
	if url == "" {
		return ""
	}
	for _, rule := range rules {
		switch rule.RuleType {
		case "replace":
			if strings.Contains(url, rule.Pattern) {
				return strings.Replace(url, rule.Pattern, rule.Replacement, 1)
			}
		case "regex":
			re, err := regexp.Compile(rule.Pattern)
			if err != nil {
				continue
			}
			if re.MatchString(url) {
				return re.ReplaceAllString(url, rule.Replacement)
			}
		}
	}
	return ""
}
