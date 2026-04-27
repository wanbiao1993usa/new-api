package model

import (
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
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

func TestGetResponsesCompactFallbackAbilitiesUsesBaseModelRows(t *testing.T) {
	truncateTables(t)

	priority := int64(10)
	supported := insertCompactFallbackChannel(t, constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)
	unsupported := insertCompactFallbackChannel(t, constant.ChannelTypeOpenRouter, common.ChannelStatusEnabled)
	disabled := insertCompactFallbackChannel(t, constant.ChannelTypeOpenAI, common.ChannelStatusManuallyDisabled)
	insertCompactFallbackAbility(t, supported.Id, &priority)
	insertCompactFallbackAbility(t, unsupported.Id, &priority)
	insertCompactFallbackAbility(t, disabled.Id, &priority)

	abilities, err := getResponsesCompactFallbackAbilities("default", ratio_setting.WithCompactModelSuffix("gpt-5.5"), 0)
	require.NoError(t, err)
	require.Len(t, abilities, 1)
	require.Equal(t, supported.Id, abilities[0].ChannelId)
}

func TestGetChannelUsesCompactFallbackWhenExactRowsAreMissingOnRetry(t *testing.T) {
	truncateTables(t)

	highPriority := int64(10)
	lowPriority := int64(1)
	highPriorityChannel := insertCompactFallbackChannel(t, constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)
	lowPriorityChannel := insertCompactFallbackChannel(t, constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)
	insertCompactFallbackAbility(t, highPriorityChannel.Id, &highPriority)
	insertCompactFallbackAbility(t, lowPriorityChannel.Id, &lowPriority)

	channel, err := GetChannel("default", ratio_setting.WithCompactModelSuffix("gpt-5.5"), 1)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, lowPriorityChannel.Id, channel.Id)
}

func TestIsChannelEnabledForGroupModelDBAllowsCompactAliasFromBaseModel(t *testing.T) {
	truncateTables(t)

	priority := int64(10)
	supported := insertCompactFallbackChannel(t, constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)
	unsupported := insertCompactFallbackChannel(t, constant.ChannelTypeOpenRouter, common.ChannelStatusEnabled)
	insertCompactFallbackAbility(t, supported.Id, &priority)
	insertCompactFallbackAbility(t, unsupported.Id, &priority)

	compactModelName := ratio_setting.WithCompactModelSuffix("gpt-5.5")
	require.True(t, IsChannelEnabledForGroupModel("default", compactModelName, supported.Id))
	require.False(t, IsChannelEnabledForGroupModel("default", compactModelName, unsupported.Id))
}

func insertCompactFallbackChannel(t *testing.T, channelType int, status int) Channel {
	t.Helper()
	channel := Channel{
		Type:   channelType,
		Key:    "test-key",
		Status: status,
		Name:   "test-channel",
		Models: "gpt-5.5",
		Group:  "default",
	}
	require.NoError(t, DB.Create(&channel).Error)
	return channel
}

func insertCompactFallbackAbility(t *testing.T, channelID int, priority *int64) {
	t.Helper()
	ability := Ability{
		Group:     "default",
		Model:     "gpt-5.5",
		ChannelId: channelID,
		Enabled:   true,
		Priority:  priority,
		Weight:    1,
	}
	require.NoError(t, DB.Create(&ability).Error)
}
