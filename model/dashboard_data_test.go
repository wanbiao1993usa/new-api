package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dashboardQuotaRowByModel(rows []*QuotaData, modelName string, createdAt int64) (*QuotaData, bool) {
	for _, row := range rows {
		if row.ModelName == modelName && row.CreatedAt == createdAt {
			return row, true
		}
	}
	return nil, false
}

func dashboardQuotaRowByUsername(rows []*QuotaData, username string, createdAt int64) (*QuotaData, bool) {
	for _, row := range rows {
		if row.Username == username && row.CreatedAt == createdAt {
			return row, true
		}
	}
	return nil, false
}

func TestGetAllQuotaDatesUsesConsumeLogsInsteadOfQuotaDataCache(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&QuotaData{
		Username:  "alice",
		ModelName: "stale-cache",
		CreatedAt: 0,
		Count:     99,
		Quota:     9900,
		TokenUsed: 9900,
	}).Error)
	insertBillingAnalysisLog(t, Log{
		Id:               101,
		UserId:           401,
		Username:         "alice",
		TokenName:        "token-a",
		ModelName:        "gpt-4o",
		CreatedAt:        1000,
		Type:             LogTypeConsume,
		Quota:            1000,
		PromptTokens:     100,
		CompletionTokens: 50,
	})
	insertBillingAnalysisLog(t, Log{
		Id:               102,
		UserId:           401,
		Username:         "alice",
		TokenName:        "token-a",
		ModelName:        "gpt-4o",
		CreatedAt:        1100,
		Type:             LogTypeConsume,
		Quota:            3000,
		PromptTokens:     200,
		CompletionTokens: 100,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":        "subscription",
			"subscription_consumed": 1500,
		}),
	})
	insertBillingAnalysisLog(t, Log{
		Id:               103,
		UserId:           402,
		Username:         "bob",
		TokenName:        "token-b",
		ModelName:        "gpt-4o-mini",
		CreatedAt:        1200,
		Type:             LogTypeConsume,
		Quota:            600,
		PromptTokens:     25,
		CompletionTokens: 25,
	})
	insertBillingAnalysisLog(t, Log{
		Id:        104,
		UserId:    401,
		Username:  "alice",
		ModelName: "gpt-4o",
		CreatedAt: 1201,
		Type:      LogTypeRefund,
		Quota:     777,
	})

	rows, err := GetAllQuotaDates(900, 1300, "", "hour")
	require.NoError(t, err)

	require.Len(t, rows, 2)
	gpt4o, ok := dashboardQuotaRowByModel(rows, "gpt-4o", 0)
	require.True(t, ok)
	assert.EqualValues(t, 2, gpt4o.Count)
	assert.EqualValues(t, 2500, gpt4o.Quota)
	assert.EqualValues(t, 450, gpt4o.TokenUsed)

	mini, ok := dashboardQuotaRowByModel(rows, "gpt-4o-mini", 0)
	require.True(t, ok)
	assert.EqualValues(t, 1, mini.Count)
	assert.EqualValues(t, 600, mini.Quota)
	assert.EqualValues(t, 50, mini.TokenUsed)
}

func TestGetQuotaDataGroupByUserUsesDayBuckets(t *testing.T) {
	truncateTables(t)

	insertBillingAnalysisLog(t, Log{
		Id:               111,
		UserId:           501,
		Username:         "alice",
		ModelName:        "gpt-4o",
		CreatedAt:        3600,
		Type:             LogTypeConsume,
		Quota:            100,
		PromptTokens:     10,
		CompletionTokens: 5,
	})
	insertBillingAnalysisLog(t, Log{
		Id:               112,
		UserId:           501,
		Username:         "alice",
		ModelName:        "gpt-4o-mini",
		CreatedAt:        86399,
		Type:             LogTypeConsume,
		Quota:            200,
		PromptTokens:     20,
		CompletionTokens: 5,
	})
	insertBillingAnalysisLog(t, Log{
		Id:               113,
		UserId:           502,
		Username:         "bob",
		ModelName:        "claude-sonnet",
		CreatedAt:        90000,
		Type:             LogTypeConsume,
		Quota:            300,
		PromptTokens:     30,
		CompletionTokens: 5,
	})

	rows, err := GetQuotaDataGroupByUser(0, 100000, "day")
	require.NoError(t, err)

	require.Len(t, rows, 2)
	alice, ok := dashboardQuotaRowByUsername(rows, "alice", 0)
	require.True(t, ok)
	assert.EqualValues(t, 2, alice.Count)
	assert.EqualValues(t, 300, alice.Quota)
	assert.EqualValues(t, 40, alice.TokenUsed)

	bob, ok := dashboardQuotaRowByUsername(rows, "bob", 86400)
	require.True(t, ok)
	assert.EqualValues(t, 1, bob.Count)
	assert.EqualValues(t, 300, bob.Quota)
	assert.EqualValues(t, 35, bob.TokenUsed)
}
