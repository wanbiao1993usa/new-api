package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func IsChannelEnabledForGroupModel(group string, modelName string, channelID int) bool {
	if group == "" || modelName == "" || channelID <= 0 {
		return false
	}
	if !common.MemoryCacheEnabled {
		return isChannelEnabledForGroupModelDB(group, modelName, channelID)
	}

	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	if group2model2channels == nil {
		return false
	}

	if isChannelIDInList(group2model2channels[group][modelName], channelID) {
		return true
	}
	normalized := ratio_setting.FormatMatchingModelName(modelName)
	if normalized != "" && normalized != modelName {
		return isChannelIDInList(group2model2channels[group][normalized], channelID)
	}
	return false
}

func IsChannelEnabledForAnyGroupModel(groups []string, modelName string, channelID int) bool {
	if len(groups) == 0 {
		return false
	}
	for _, g := range groups {
		if IsChannelEnabledForGroupModel(g, modelName, channelID) {
			return true
		}
	}
	return false
}

func isChannelEnabledForGroupModelDB(group string, modelName string, channelID int) bool {
	if isAbilityEnabledForGroupModelDB(group, modelName, channelID) {
		return true
	}
	normalized := ratio_setting.FormatMatchingModelName(modelName)
	if normalized != "" && normalized != modelName && isAbilityEnabledForGroupModelDB(group, normalized, channelID) {
		return true
	}
	if !ratio_setting.IsCompactModelName(modelName) {
		return false
	}
	baseModelName := ratio_setting.TrimCompactModelSuffix(modelName)
	if baseModelName == modelName || !isDBChannelEnabledForResponsesCompact(channelID) {
		return false
	}
	if isAbilityEnabledForGroupModelDB(group, baseModelName, channelID) {
		return true
	}
	normalizedBaseModelName := ratio_setting.FormatMatchingModelName(baseModelName)
	return normalizedBaseModelName != "" &&
		normalizedBaseModelName != baseModelName &&
		isAbilityEnabledForGroupModelDB(group, normalizedBaseModelName, channelID)
}

func isAbilityEnabledForGroupModelDB(group string, modelName string, channelID int) bool {
	var count int64
	err := DB.Model(&Ability{}).
		Where(commonGroupCol+" = ? and model = ? and channel_id = ? and enabled = ?", group, modelName, channelID, true).
		Count(&count).Error
	return err == nil && count > 0
}

func isDBChannelEnabledForResponsesCompact(channelID int) bool {
	if channelID <= 0 {
		return false
	}
	var channel Channel
	err := DB.Select("id", "type", "status").First(&channel, "id = ?", channelID).Error
	return err == nil &&
		channel.Status == common.ChannelStatusEnabled &&
		channelSupportsResponsesCompact(channel.Type)
}

func isChannelIDInList(list []int, channelID int) bool {
	for _, id := range list {
		if id == channelID {
			return true
		}
	}
	return false
}
