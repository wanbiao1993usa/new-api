package ratio_setting

import "testing"

func TestCompactModelSuffixHelpers(t *testing.T) {
	modelName := "gpt-5.5"
	compactModelName := WithCompactModelSuffix(modelName)

	if compactModelName != "gpt-5.5-openai-compact" {
		t.Fatalf("expected compact suffix to be appended, got %q", compactModelName)
	}
	if WithCompactModelSuffix(compactModelName) != compactModelName {
		t.Fatal("expected compact suffix to be appended only once")
	}
	if !IsCompactModelName(compactModelName) {
		t.Fatal("expected compact model name to be recognized")
	}
	if TrimCompactModelSuffix(compactModelName) != modelName {
		t.Fatal("expected compact suffix to be trimmed")
	}
}
