package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func channelSupportsResponsesCompact(channelType int) bool {
	apiType, _ := common.ChannelType2APIType(channelType)
	return apiType == constant.APITypeOpenAI || apiType == constant.APITypeCodex
}

func shouldExposeResponsesCompactModel(model string) bool {
	model = strings.TrimSpace(model)
	if model == "" {
		return false
	}
	if strings.HasSuffix(model, ratio_setting.CompactModelSuffix) {
		return true
	}
	return strings.HasPrefix(model, "gpt-5")
}

func expandChannelModelsForResponsesCompact(models []string, channelType int) []string {
	if !channelSupportsResponsesCompact(channelType) {
		return models
	}

	seen := make(map[string]struct{}, len(models)*2)
	expanded := make([]string, 0, len(models)*2)
	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		if _, ok := seen[modelName]; !ok {
			seen[modelName] = struct{}{}
			expanded = append(expanded, modelName)
		}
		if shouldExposeResponsesCompactModel(modelName) {
			compactModelName := ratio_setting.WithCompactModelSuffix(modelName)
			if _, ok := seen[compactModelName]; !ok {
				seen[compactModelName] = struct{}{}
				expanded = append(expanded, compactModelName)
			}
		}
	}
	return expanded
}
