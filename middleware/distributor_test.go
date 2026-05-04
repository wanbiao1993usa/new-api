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

func TestValidateImageModelForDistributionRejectsTextModelsBeforeChannelSelection(t *testing.T) {
	for _, modelName := range []string{"gpt-5.5", "gpt5.5", "gpt-5-mini", "gpt5-mini"} {
		if err := validateImageModelForDistribution("/v1/images/generations", modelName); err == nil {
			t.Fatalf("expected %s to be rejected on image generation path", modelName)
		}
	}
}

func TestValidateImageModelForDistributionAllowsImageModelsAndOtherPaths(t *testing.T) {
	if err := validateImageModelForDistribution("/v1/images/generations", "gpt-image-1"); err != nil {
		t.Fatalf("expected image model to be allowed, got %v", err)
	}
	if err := validateImageModelForDistribution("/v1/chat/completions", "gpt-5.5"); err != nil {
		t.Fatalf("expected text model to be allowed on chat path, got %v", err)
	}
}
