package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertBillingAnalysisLog(t *testing.T, log Log) {
	t.Helper()
	require.NoError(t, LOG_DB.Create(&log).Error)
}

func seedBillingAnalysisLedger(t *testing.T) {
	t.Helper()
	insertBillingAnalysisLog(t, Log{
		Id:               21,
		UserId:           301,
		Username:         "alice",
		TokenName:        "token-a",
		ModelName:        "gpt-4o",
		ChannelId:        11,
		Group:            "default",
		CreatedAt:        1000,
		Type:             LogTypeConsume,
		Quota:            1000,
		PromptTokens:     100,
		CompletionTokens: 100,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source": "wallet",
		}),
	})
	insertBillingAnalysisLog(t, Log{
		Id:               22,
		UserId:           301,
		Username:         "alice",
		TokenName:        "token-a",
		ModelName:        "gpt-4o",
		ChannelId:        11,
		Group:            "default",
		CreatedAt:        1100,
		Type:             LogTypeConsume,
		Quota:            2000,
		PromptTokens:     200,
		CompletionTokens: 300,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":        "subscription",
			"subscription_consumed": 1200,
		}),
	})
	insertBillingAnalysisLog(t, Log{
		Id:               23,
		UserId:           301,
		Username:         "alice",
		TokenName:        "token-b",
		ModelName:        "gpt-4o-mini",
		ChannelId:        12,
		Group:            "vip",
		CreatedAt:        1200,
		Type:             LogTypeConsume,
		Quota:            300,
		PromptTokens:     50,
		CompletionTokens: 50,
	})
	insertBillingAnalysisLog(t, Log{
		Id:               24,
		UserId:           302,
		Username:         "bob",
		TokenName:        "token-c",
		ModelName:        "claude-sonnet",
		ChannelId:        13,
		Group:            "vip",
		CreatedAt:        1300,
		Type:             LogTypeConsume,
		Quota:            900,
		PromptTokens:     20,
		CompletionTokens: 30,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source": "subscription",
		}),
	})
	insertBillingAnalysisLog(t, Log{
		Id:               25,
		UserId:           303,
		Username:         "carol",
		TokenName:        "outside-window",
		ModelName:        "gpt-4o",
		ChannelId:        14,
		Group:            "default",
		CreatedAt:        1401,
		Type:             LogTypeConsume,
		Quota:            700,
		PromptTokens:     70,
		CompletionTokens: 70,
	})
	insertBillingAnalysisLog(t, Log{
		Id:        26,
		UserId:    301,
		Username:  "alice",
		TokenName: "refund-noise",
		CreatedAt: 1201,
		Type:      LogTypeRefund,
		Quota:     777,
	})
	insertBillingAnalysisLog(t, Log{
		Id:        27,
		UserId:    302,
		Username:  "bob",
		TokenName: "error-noise",
		CreatedAt: 1202,
		Type:      LogTypeError,
		Quota:     888,
	})
}

func billingAnalysisRowByKey(rows []BillingAnalysisRow, key string) (BillingAnalysisRow, bool) {
	for _, row := range rows {
		if row.Key == key {
			return row, true
		}
	}
	return BillingAnalysisRow{}, false
}

func TestGetBillingAnalysisSplitsWalletAndSubscription(t *testing.T) {
	truncateTables(t)

	insertBillingAnalysisLog(t, Log{
		Id:               1,
		UserId:           101,
		Username:         "alice",
		TokenName:        "prod-token",
		ModelName:        "gpt-4o",
		ChannelId:        10,
		Group:            "default",
		CreatedAt:        1000,
		Type:             LogTypeConsume,
		Quota:            1000,
		PromptTokens:     100,
		CompletionTokens: 50,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source": "wallet",
		}),
	})
	insertBillingAnalysisLog(t, Log{
		Id:               2,
		UserId:           101,
		Username:         "alice",
		TokenName:        "prod-token",
		ModelName:        "gpt-4o",
		ChannelId:        10,
		Group:            "vip",
		CreatedAt:        1100,
		Type:             LogTypeConsume,
		Quota:            2000,
		PromptTokens:     200,
		CompletionTokens: 300,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":        "subscription",
			"subscription_consumed": 1500,
		}),
	})
	insertBillingAnalysisLog(t, Log{
		Id:               3,
		UserId:           102,
		Username:         "bob",
		TokenName:        "dev-token",
		ModelName:        "claude-sonnet",
		ChannelId:        20,
		Group:            "vip",
		CreatedAt:        1200,
		Type:             LogTypeConsume,
		Quota:            500,
		PromptTokens:     5,
		CompletionTokens: 5,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source": "subscription",
		}),
	})
	insertBillingAnalysisLog(t, Log{
		Id:        4,
		UserId:    101,
		Username:  "alice",
		CreatedAt: 1201,
		Type:      LogTypeRefund,
		Quota:     777,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":        "subscription",
			"subscription_consumed": 777,
		}),
	})
	insertBillingAnalysisLog(t, Log{
		Id:        5,
		UserId:    101,
		Username:  "alice",
		CreatedAt: 1202,
		Type:      LogTypeError,
		Quota:     999,
	})

	result, err := GetBillingAnalysis(BillingAnalysisFilters{
		StartTimestamp: 900,
		EndTimestamp:   1300,
	}, true)
	require.NoError(t, err)

	assert.EqualValues(t, 3, result.Summary.RequestCount)
	assert.EqualValues(t, 660, result.Summary.TokenCount)
	assert.EqualValues(t, 3000, result.Summary.TotalQuota)
	assert.EqualValues(t, 1000, result.Summary.WalletQuota)
	assert.EqualValues(t, 2000, result.Summary.SubscriptionQuota)
	assert.InDelta(t, 3000, result.Summary.OriginalTotalQuota, 0.0001)
	assert.InDelta(t, 4545454.55, result.Summary.EffectiveQuotaPer1KTokens, 0.01)

	require.Len(t, result.Tokens, 2)
	prodToken := result.Tokens[0]
	assert.Equal(t, "prod-token", prodToken.Name)
	assert.EqualValues(t, 2, prodToken.RequestCount)
	assert.EqualValues(t, 650, prodToken.TokenCount)
	assert.EqualValues(t, 2500, prodToken.TotalQuota)
	assert.EqualValues(t, 1000, prodToken.WalletQuota)
	assert.EqualValues(t, 1500, prodToken.SubscriptionQuota)
	assert.EqualValues(t, 1100, prodToken.LastUsedAt)

	require.Len(t, result.Users, 2)
	assert.Equal(t, "alice", result.Users[0].Name)
	assert.EqualValues(t, 2500, result.Users[0].TotalQuota)

	require.Len(t, result.Channels, 2)
	assert.Equal(t, "10", result.Channels[0].Key)
	assert.EqualValues(t, 2500, result.Channels[0].TotalQuota)
}

func TestGetBillingAnalysisFiltersAndHidesAdminDimensionsForSelf(t *testing.T) {
	truncateTables(t)

	insertBillingAnalysisLog(t, Log{
		Id:               11,
		UserId:           201,
		Username:         "self-user",
		TokenName:        "self-token",
		ModelName:        "gpt-4o",
		ChannelId:        30,
		Group:            "default",
		CreatedAt:        2000,
		Type:             LogTypeConsume,
		Quota:            100,
		PromptTokens:     10,
		CompletionTokens: 10,
	})
	insertBillingAnalysisLog(t, Log{
		Id:               12,
		UserId:           202,
		Username:         "other-user",
		TokenName:        "other-token",
		ModelName:        "gpt-4o",
		ChannelId:        31,
		Group:            "default",
		CreatedAt:        2001,
		Type:             LogTypeConsume,
		Quota:            900,
		PromptTokens:     90,
		CompletionTokens: 90,
	})

	result, err := GetBillingAnalysis(BillingAnalysisFilters{
		UserId:    201,
		ModelName: "gpt-4o",
		Group:     "default",
	}, false)
	require.NoError(t, err)

	assert.EqualValues(t, 1, result.Summary.RequestCount)
	assert.EqualValues(t, 20, result.Summary.TokenCount)
	assert.EqualValues(t, 100, result.Summary.TotalQuota)
	assert.Empty(t, result.Users)
	assert.Empty(t, result.Channels)

	require.Len(t, result.Tokens, 1)
	assert.Equal(t, "self-token", result.Tokens[0].Name)
	require.Len(t, result.Models, 1)
	assert.Equal(t, "gpt-4o", result.Models[0].Name)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, "default", result.Groups[0].Name)
}

func TestGetBillingAnalysisSeededLedgerAggregatesEveryDimension(t *testing.T) {
	truncateTables(t)
	seedBillingAnalysisLedger(t)

	result, err := GetBillingAnalysis(BillingAnalysisFilters{
		StartTimestamp: 1000,
		EndTimestamp:   1300,
	}, true)
	require.NoError(t, err)

	assert.EqualValues(t, 4, result.Summary.RequestCount)
	assert.EqualValues(t, 850, result.Summary.TokenCount)
	assert.EqualValues(t, 3400, result.Summary.TotalQuota)
	assert.EqualValues(t, 1300, result.Summary.WalletQuota)
	assert.EqualValues(t, 2100, result.Summary.SubscriptionQuota)
	assert.InDelta(t, 3400, result.Summary.OriginalTotalQuota, 0.0001)
	assert.InDelta(t, 4000000, result.Summary.EffectiveQuotaPer1KTokens, 0.01)

	require.Len(t, result.Users, 2)
	alice, ok := billingAnalysisRowByKey(result.Users, "301")
	require.True(t, ok)
	assert.Equal(t, "alice", alice.Name)
	assert.EqualValues(t, 3, alice.RequestCount)
	assert.EqualValues(t, 800, alice.TokenCount)
	assert.EqualValues(t, 2500, alice.TotalQuota)
	assert.EqualValues(t, 1300, alice.WalletQuota)
	assert.EqualValues(t, 1200, alice.SubscriptionQuota)
	assert.EqualValues(t, 1200, alice.LastUsedAt)

	bob, ok := billingAnalysisRowByKey(result.Users, "302")
	require.True(t, ok)
	assert.EqualValues(t, 1, bob.RequestCount)
	assert.EqualValues(t, 50, bob.TokenCount)
	assert.EqualValues(t, 900, bob.TotalQuota)
	assert.EqualValues(t, 0, bob.WalletQuota)
	assert.EqualValues(t, 900, bob.SubscriptionQuota)

	require.Len(t, result.Tokens, 3)
	assert.Equal(t, "token-a", result.Tokens[0].Key)
	assert.Equal(t, "token-c", result.Tokens[1].Key)
	assert.Equal(t, "token-b", result.Tokens[2].Key)

	tokenA, ok := billingAnalysisRowByKey(result.Tokens, "token-a")
	require.True(t, ok)
	assert.EqualValues(t, 2, tokenA.RequestCount)
	assert.EqualValues(t, 700, tokenA.TokenCount)
	assert.EqualValues(t, 2200, tokenA.TotalQuota)
	assert.EqualValues(t, 1000, tokenA.WalletQuota)
	assert.EqualValues(t, 1200, tokenA.SubscriptionQuota)
	assert.InDelta(t, 3142857.14, tokenA.EffectiveQuotaPer1KTokens, 0.01)

	modelGPT4o, ok := billingAnalysisRowByKey(result.Models, "gpt-4o")
	require.True(t, ok)
	assert.EqualValues(t, 2, modelGPT4o.RequestCount)
	assert.EqualValues(t, 700, modelGPT4o.TokenCount)
	assert.EqualValues(t, 2200, modelGPT4o.TotalQuota)

	groupDefault, ok := billingAnalysisRowByKey(result.Groups, "default")
	require.True(t, ok)
	assert.EqualValues(t, 2, groupDefault.RequestCount)
	assert.EqualValues(t, 2200, groupDefault.TotalQuota)

	groupVIP, ok := billingAnalysisRowByKey(result.Groups, "vip")
	require.True(t, ok)
	assert.EqualValues(t, 2, groupVIP.RequestCount)
	assert.EqualValues(t, 1200, groupVIP.TotalQuota)

	channel11, ok := billingAnalysisRowByKey(result.Channels, "11")
	require.True(t, ok)
	assert.EqualValues(t, 2, channel11.RequestCount)
	assert.EqualValues(t, 2200, channel11.TotalQuota)
}

func TestGetBillingAnalysisSeededLedgerFiltersBeforeAggregating(t *testing.T) {
	truncateTables(t)
	seedBillingAnalysisLedger(t)

	result, err := GetBillingAnalysis(BillingAnalysisFilters{
		StartTimestamp: 1000,
		EndTimestamp:   1300,
		Username:       "alice",
		TokenName:      "token-a",
		ModelName:      "gpt-4o",
		Channel:        11,
		Group:          "default",
	}, true)
	require.NoError(t, err)

	assert.EqualValues(t, 2, result.Summary.RequestCount)
	assert.EqualValues(t, 700, result.Summary.TokenCount)
	assert.EqualValues(t, 2200, result.Summary.TotalQuota)
	assert.EqualValues(t, 1000, result.Summary.WalletQuota)
	assert.EqualValues(t, 1200, result.Summary.SubscriptionQuota)

	require.Len(t, result.Users, 1)
	assert.Equal(t, "alice", result.Users[0].Name)
	require.Len(t, result.Tokens, 1)
	assert.Equal(t, "token-a", result.Tokens[0].Name)
	require.Len(t, result.Models, 1)
	assert.Equal(t, "gpt-4o", result.Models[0].Name)
	require.Len(t, result.Groups, 1)
	assert.Equal(t, "default", result.Groups[0].Name)
	require.Len(t, result.Channels, 1)
	assert.Equal(t, "11", result.Channels[0].Key)
}

func TestGetBillingAnalysisBuildsMultiplierOverview(t *testing.T) {
	truncateTables(t)

	insertBillingAnalysisLog(t, Log{
		Id:               31,
		UserId:           401,
		Username:         "alice",
		TokenName:        "token-a",
		ModelName:        "gpt-4o",
		ChannelId:        21,
		Group:            "default",
		CreatedAt:        2000,
		Type:             LogTypeConsume,
		Quota:            1000,
		PromptTokens:     100,
		CompletionTokens: 100,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":   "wallet",
			"model_ratio":      2.0,
			"group_ratio":      0.5,
			"user_group_ratio": -1.0,
		}),
	})
	insertBillingAnalysisLog(t, Log{
		Id:               32,
		UserId:           401,
		Username:         "alice",
		TokenName:        "token-b",
		ModelName:        "gpt-4o-mini",
		ChannelId:        21,
		Group:            "default",
		CreatedAt:        2010,
		Type:             LogTypeConsume,
		Quota:            150,
		PromptTokens:     50,
		CompletionTokens: 50,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":   "wallet",
			"model_ratio":      0.2,
			"group_ratio":      0.5,
			"user_group_ratio": -1.0,
		}),
	})
	insertBillingAnalysisLog(t, Log{
		Id:               35,
		UserId:           401,
		Username:         "alice",
		TokenName:        "token-e",
		ModelName:        "gpt-4.1-mini",
		ChannelId:        21,
		Group:            "default",
		CreatedAt:        2015,
		Type:             LogTypeConsume,
		Quota:            400,
		PromptTokens:     20,
		CompletionTokens: 20,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":   "wallet",
			"model_ratio":      1.0,
			"group_ratio":      0.25,
			"user_group_ratio": -1.0,
		}),
	})
	insertBillingAnalysisLog(t, Log{
		Id:               33,
		UserId:           402,
		Username:         "bob",
		TokenName:        "token-c",
		ModelName:        "claude-sonnet",
		ChannelId:        22,
		Group:            "vip",
		CreatedAt:        2020,
		Type:             LogTypeConsume,
		Quota:            2000,
		PromptTokens:     120,
		CompletionTokens: 80,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":        "subscription",
			"subscription_consumed": 1500,
			"model_ratio":           2.0,
			"group_ratio":           1.0,
			"user_group_ratio":      0.5,
		}),
	})
	insertBillingAnalysisLog(t, Log{
		Id:               34,
		UserId:           402,
		Username:         "bob",
		TokenName:        "token-d",
		ModelName:        "claude-opus",
		ChannelId:        22,
		Group:            "vip",
		CreatedAt:        2030,
		Type:             LogTypeConsume,
		Quota:            900,
		PromptTokens:     60,
		CompletionTokens: 40,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":        "subscription",
			"subscription_consumed": 900,
			"billing_mode":          "tiered_expr",
			"matched_tier":          "long_context",
			"group_ratio":           0.25,
		}),
	})

	result, err := GetBillingAnalysis(BillingAnalysisFilters{}, true)
	require.NoError(t, err)
	assert.InDelta(t, 10500, result.Summary.OriginalTotalQuota, 0.0001)

	require.Len(t, result.Summary.WalletMultiplierOverview, 2)
	assert.Equal(t, "分组倍率 0.5x", result.Summary.WalletMultiplierOverview[0].Label)
	assert.EqualValues(t, 1150, result.Summary.WalletMultiplierOverview[0].Quota)
	assert.InDelta(t, 2300, result.Summary.WalletMultiplierOverview[0].OriginalQuota, 0.0001)
	assert.EqualValues(t, 2, result.Summary.WalletMultiplierOverview[0].RequestCount)
	assert.Equal(t, "分组倍率 0.25x", result.Summary.WalletMultiplierOverview[1].Label)
	assert.EqualValues(t, 400, result.Summary.WalletMultiplierOverview[1].Quota)
	assert.InDelta(t, 1600, result.Summary.WalletMultiplierOverview[1].OriginalQuota, 0.0001)
	assert.EqualValues(t, 1, result.Summary.WalletMultiplierOverview[1].RequestCount)

	require.Len(t, result.Summary.SubscriptionMultiplierOverview, 2)
	assert.Equal(t, "专属倍率 0.5x", result.Summary.SubscriptionMultiplierOverview[0].Label)
	assert.EqualValues(t, 1500, result.Summary.SubscriptionMultiplierOverview[0].Quota)
	assert.InDelta(t, 3000, result.Summary.SubscriptionMultiplierOverview[0].OriginalQuota, 0.0001)
	assert.Equal(t, "阶梯计费 / long_context / 分组倍率 0.25x", result.Summary.SubscriptionMultiplierOverview[1].Label)
	assert.EqualValues(t, 900, result.Summary.SubscriptionMultiplierOverview[1].Quota)
	assert.InDelta(t, 3600, result.Summary.SubscriptionMultiplierOverview[1].OriginalQuota, 0.0001)
}

func TestGetBillingAnalysisBuildsUsageMultiplierOverview(t *testing.T) {
	truncateTables(t)

	insertBillingAnalysisLog(t, Log{
		Id:               41,
		UserId:           501,
		Username:         "alice",
		TokenName:        "token-a",
		ModelName:        "gpt-4o",
		ChannelId:        31,
		Group:            "default",
		CreatedAt:        3000,
		Type:             LogTypeConsume,
		Quota:            100,
		PromptTokens:     40,
		CompletionTokens: 60,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":   "wallet",
			"model_ratio":      1.0,
			"group_ratio":      0.5,
			"user_group_ratio": -1.0,
		}),
	})
	insertBillingAnalysisLog(t, Log{
		Id:               42,
		UserId:           501,
		Username:         "alice",
		TokenName:        "token-b",
		ModelName:        "gpt-4o-mini",
		ChannelId:        31,
		Group:            "default",
		CreatedAt:        3010,
		Type:             LogTypeConsume,
		Quota:            800,
		PromptTokens:     120,
		CompletionTokens: 180,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":        "subscription",
			"subscription_consumed": 400,
			"model_ratio":           1.0,
			"group_ratio":           1.0,
			"user_group_ratio":      0.5,
		}),
	})
	insertBillingAnalysisLog(t, Log{
		Id:               43,
		UserId:           502,
		Username:         "bob",
		TokenName:        "token-c",
		ModelName:        "claude-sonnet",
		ChannelId:        32,
		Group:            "vip",
		CreatedAt:        3020,
		Type:             LogTypeConsume,
		Quota:            200,
		PromptTokens:     25,
		CompletionTokens: 75,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":   "wallet",
			"billing_mode":     "tiered_expr",
			"matched_tier":     "long_context",
			"group_ratio":      0.25,
			"user_group_ratio": -1.0,
		}),
	})

	result, err := GetBillingAnalysis(BillingAnalysisFilters{}, true)
	require.NoError(t, err)

	data, err := common.Marshal(result.Summary)
	require.NoError(t, err)
	var summary map[string]interface{}
	require.NoError(t, common.Unmarshal(data, &summary))

	rawOverview, ok := summary["multiplier_overview"].([]interface{})
	require.True(t, ok, "summary should expose multiplier_overview")
	require.Len(t, rawOverview, 3)

	first, ok := rawOverview[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "专属倍率 0.5x", first["label"])
	assert.EqualValues(t, 400, first["quota"])
	assert.EqualValues(t, 300, first["token_count"])
	assert.EqualValues(t, 1, first["request_count"])
	assert.InDelta(t, 800, first["original_quota"], 0.0001)
	assert.InDelta(t, 1333333.33, first["effective_quota_per_1k_tokens"], 0.01)

	second, ok := rawOverview[1].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "阶梯计费 / long_context / 分组倍率 0.25x", second["label"])
	assert.EqualValues(t, 200, second["quota"])
	assert.EqualValues(t, 100, second["token_count"])
	assert.EqualValues(t, 1, second["request_count"])
	assert.InDelta(t, 800, second["original_quota"], 0.0001)
	assert.InDelta(t, 2000000, second["effective_quota_per_1k_tokens"], 0.01)

	third, ok := rawOverview[2].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "分组倍率 0.5x", third["label"])
	assert.EqualValues(t, 100, third["quota"])
	assert.EqualValues(t, 100, third["token_count"])
	assert.EqualValues(t, 1, third["request_count"])
	assert.InDelta(t, 200, third["original_quota"], 0.0001)
	assert.InDelta(t, 1000000, third["effective_quota_per_1k_tokens"], 0.01)
}

func TestGetBillingAnalysisOverviewMetaIgnoresUnsetModelPrice(t *testing.T) {
	modelRatio := 2.0
	groupRatio := 0.5
	unsetModelPrice := -1.0

	meta := getBillingAnalysisOverviewMeta(billingAnalysisLogOther{
		ModelRatio: &modelRatio,
		GroupRatio: &groupRatio,
		ModelPrice: &unsetModelPrice,
	})

	assert.Equal(t, "分组倍率 0.5x", meta.Label)
	assert.Equal(t, "group:0.500000", meta.Key)
	assert.InDelta(t, 0.5, meta.EffectiveRatio, 0.000001)
}
