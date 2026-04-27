package middleware

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func TestIsModelAllowedByTokenLimitAllowsCompactAliasFromBaseModel(t *testing.T) {
	tokenModelLimit := map[string]bool{
		"gpt-5.5": true,
	}

	if !isModelAllowedByTokenLimit(ratio_setting.WithCompactModelSuffix("gpt-5.5"), tokenModelLimit) {
		t.Fatal("expected compact alias to be allowed by base model limit")
	}
}

func TestIsModelAllowedByTokenLimitRequiresBaseModelForCompactAlias(t *testing.T) {
	tokenModelLimit := map[string]bool{
		"gpt-5": true,
	}

	if isModelAllowedByTokenLimit(ratio_setting.WithCompactModelSuffix("gpt-5.5"), tokenModelLimit) {
		t.Fatal("expected compact alias to be denied when base model is not allowed")
	}
}

func TestIsModelAllowedByTokenLimitKeepsDirectModelMatching(t *testing.T) {
	tokenModelLimit := map[string]bool{
		"gpt-4o": true,
	}

	if !isModelAllowedByTokenLimit("gpt-4o", tokenModelLimit) {
		t.Fatal("expected direct model to be allowed")
	}
	if isModelAllowedByTokenLimit("gpt-4.1", tokenModelLimit) {
		t.Fatal("expected unrelated model to be denied")
	}
}
