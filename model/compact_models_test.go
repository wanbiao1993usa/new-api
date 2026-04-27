package model

import (
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func TestExpandChannelModelsForResponsesCompact(t *testing.T) {
	got := expandChannelModelsForResponsesCompact(
		[]string{"gpt-5.5", " gpt-4o ", "gpt-5.5"},
		constant.ChannelTypeOpenAI,
	)
	want := []string{
		"gpt-5.5",
		ratio_setting.WithCompactModelSuffix("gpt-5.5"),
		"gpt-4o",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestExpandChannelModelsForResponsesCompactSkipsUnsupportedChannel(t *testing.T) {
	got := expandChannelModelsForResponsesCompact([]string{"gpt-5.5"}, constant.ChannelTypeOpenRouter)
	want := []string{"gpt-5.5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestExpandChannelModelsForResponsesCompactDoesNotDuplicateSuffix(t *testing.T) {
	modelName := ratio_setting.WithCompactModelSuffix("gpt-5.5")
	got := expandChannelModelsForResponsesCompact([]string{modelName}, constant.ChannelTypeOpenAI)
	want := []string{modelName}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}
