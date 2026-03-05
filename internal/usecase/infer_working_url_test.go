package usecase

import (
	"testing"

	"github.com/meta-BE/bms-elsa/internal/domain/model"
)

func TestApplyRewriteRules(t *testing.T) {
	rules := []model.RewriteRule{
		{ID: 1, RuleType: "replace", Pattern: "old-host.com/bms", Replacement: "new-host.com/bms", Priority: 10},
		{ID: 2, RuleType: "regex", Pattern: `example\.com/dl/(\d+)`, Replacement: "mirror.com/download?id=$1", Priority: 5},
	}

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"replaceマッチ", "http://old-host.com/bms/song123", "http://new-host.com/bms/song123"},
		{"regexマッチ", "http://example.com/dl/456", "http://mirror.com/download?id=456"},
		{"マッチなし", "http://other-host.com/file", ""},
		{"空URL", "", ""},
		{"priority順（高い方が優先）", "http://old-host.com/bms/example.com/dl/789", "http://new-host.com/bms/example.com/dl/789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyRewriteRules(tt.url, rules)
			if result != tt.expected {
				t.Errorf("applyRewriteRules(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestApplyRewriteRules_EmptyRules(t *testing.T) {
	result := applyRewriteRules("http://example.com", nil)
	if result != "" {
		t.Errorf("空ルールなのに結果が返った: %q", result)
	}
}
