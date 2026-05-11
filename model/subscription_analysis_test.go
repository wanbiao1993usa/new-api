package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertSubscriptionAnalysisUser(t *testing.T, id int, username string) {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id:       id,
		Username: username,
		Status:   common.UserStatusEnabled,
	}).Error)
}

func insertSubscriptionAnalysisLog(t *testing.T, log Log) {
	t.Helper()
	require.NoError(t, LOG_DB.Create(&log).Error)
}

func TestGetSubscriptionAnalysisAggregatesPlans(t *testing.T) {
	truncateTables(t)

	insertSubscriptionAnalysisUser(t, 901, "sub-analysis-a")
	insertSubscriptionAnalysisUser(t, 902, "sub-analysis-b")
	insertSubscriptionAnalysisUser(t, 903, "sub-analysis-c")

	planA := insertSubscriptionLimitPlan(t, 1001, 1000, "", SubscriptionResetNever)
	planA.Title = "Starter"
	require.NoError(t, DB.Save(planA).Error)
	InvalidateSubscriptionPlanCache(planA.Id)

	planB := insertSubscriptionLimitPlan(t, 1002, 0, "", SubscriptionResetNever)
	planB.Title = "Unlimited"
	require.NoError(t, DB.Save(planB).Error)
	InvalidateSubscriptionPlanCache(planB.Id)

	now := time.Now().Unix()
	require.NoError(t, DB.Create(&UserSubscription{
		Id:          2001,
		UserId:      901,
		PlanId:      1001,
		AmountTotal: 1000,
		AmountUsed:  400,
		StartTime:   now - 3600,
		EndTime:     now + 3600,
		Status:      "active",
	}).Error)
	require.NoError(t, DB.Create(&UserSubscription{
		Id:          2002,
		UserId:      902,
		PlanId:      1001,
		AmountTotal: 1000,
		AmountUsed:  700,
		StartTime:   now - 3600,
		EndTime:     now + 7200,
		Status:      "active",
	}).Error)
	require.NoError(t, DB.Create(&UserSubscription{
		Id:          2003,
		UserId:      903,
		PlanId:      1001,
		AmountTotal: 1000,
		AmountUsed:  900,
		StartTime:   now - 7200,
		EndTime:     now - 60,
		Status:      "expired",
	}).Error)
	require.NoError(t, DB.Create(&UserSubscription{
		Id:          2004,
		UserId:      902,
		PlanId:      1002,
		AmountTotal: 0,
		AmountUsed:  250,
		StartTime:   now - 3600,
		EndTime:     now + 3600,
		Status:      "active",
	}).Error)

	insertSubscriptionAnalysisLog(t, Log{
		Id:        3001,
		UserId:    901,
		CreatedAt: 1000,
		Type:      LogTypeConsume,
		Quota:     400,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":        "subscription",
			"subscription_plan_id":  1001,
			"subscription_consumed": 400,
		}),
	})
	insertSubscriptionAnalysisLog(t, Log{
		Id:        3002,
		UserId:    902,
		CreatedAt: 1100,
		Type:      LogTypeConsume,
		Quota:     600,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":       "subscription",
			"subscription_plan_id": 1001,
		}),
	})
	insertSubscriptionAnalysisLog(t, Log{
		Id:        3003,
		UserId:    902,
		CreatedAt: 1200,
		Type:      LogTypeRefund,
		Quota:     100,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":        "subscription",
			"subscription_plan_id":  1001,
			"subscription_consumed": 100,
		}),
	})
	insertSubscriptionAnalysisLog(t, Log{
		Id:        3004,
		UserId:    903,
		CreatedAt: 1300,
		Type:      LogTypeConsume,
		Quota:     900,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":        "subscription",
			"subscription_plan_id":  1001,
			"subscription_consumed": 900,
		}),
	})
	insertSubscriptionAnalysisLog(t, Log{
		Id:        3005,
		UserId:    902,
		CreatedAt: 1400,
		Type:      LogTypeConsume,
		Quota:     250,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":        "subscription",
			"subscription_plan_id":  1002,
			"subscription_consumed": 250,
		}),
	})

	result, err := GetSubscriptionAnalysis(SubscriptionAnalysisFilters{})
	require.NoError(t, err)

	assert.EqualValues(t, 2, result.Summary.PlanCount)
	assert.EqualValues(t, 2, result.Summary.ActiveUserCount)
	assert.EqualValues(t, 3, result.Summary.ActiveSubscriptionCount)
	assert.EqualValues(t, 2050, result.Summary.HistoricalUsedTotal)
	assert.EqualValues(t, 1350, result.Summary.CurrentUsedTotal)
	assert.EqualValues(t, 900, result.Summary.CurrentRemainingTotal)
	assert.EqualValues(t, 1, result.Summary.UnlimitedActiveSubscriptionCount)

	require.Len(t, result.Plans, 2)
	assert.EqualValues(t, 1001, result.Plans[0].PlanId)
	assert.Equal(t, "Starter", result.Plans[0].Title)
	assert.EqualValues(t, 3, result.Plans[0].UserCount)
	assert.EqualValues(t, 2, result.Plans[0].ActiveUserCount)
	assert.EqualValues(t, 3, result.Plans[0].SubscriptionCount)
	assert.EqualValues(t, 2, result.Plans[0].ActiveSubscriptionCount)
	assert.EqualValues(t, 1800, result.Plans[0].HistoricalUsedTotal)
	assert.EqualValues(t, 1100, result.Plans[0].CurrentUsedTotal)
	assert.EqualValues(t, 900, result.Plans[0].CurrentRemainingTotal)
	assert.EqualValues(t, 0, result.Plans[0].UnlimitedActiveSubscriptionCount)

	assert.EqualValues(t, 1002, result.Plans[1].PlanId)
	assert.Equal(t, "Unlimited", result.Plans[1].Title)
	assert.EqualValues(t, 1, result.Plans[1].UserCount)
	assert.EqualValues(t, 1, result.Plans[1].ActiveUserCount)
	assert.EqualValues(t, 1, result.Plans[1].SubscriptionCount)
	assert.EqualValues(t, 1, result.Plans[1].ActiveSubscriptionCount)
	assert.EqualValues(t, 250, result.Plans[1].HistoricalUsedTotal)
	assert.EqualValues(t, 250, result.Plans[1].CurrentUsedTotal)
	assert.EqualValues(t, 0, result.Plans[1].CurrentRemainingTotal)
	assert.EqualValues(t, 1, result.Plans[1].UnlimitedActiveSubscriptionCount)
}

func TestGetSubscriptionAnalysisFiltersHistoricalUsageByTimeRange(t *testing.T) {
	truncateTables(t)

	insertSubscriptionAnalysisUser(t, 911, "sub-analysis-filter")
	plan := insertSubscriptionLimitPlan(t, 1011, 1000, "", SubscriptionResetNever)
	plan.Title = "Filter Plan"
	require.NoError(t, DB.Save(plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)

	now := time.Now().Unix()
	require.NoError(t, DB.Create(&UserSubscription{
		Id:          2011,
		UserId:      911,
		PlanId:      1011,
		AmountTotal: 1000,
		AmountUsed:  300,
		StartTime:   now - 3600,
		EndTime:     now + 3600,
		Status:      "active",
	}).Error)

	insertSubscriptionAnalysisLog(t, Log{
		Id:        3011,
		UserId:    911,
		CreatedAt: 1000,
		Type:      LogTypeConsume,
		Quota:     200,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":        "subscription",
			"subscription_plan_id":  1011,
			"subscription_consumed": 200,
		}),
	})
	insertSubscriptionAnalysisLog(t, Log{
		Id:        3012,
		UserId:    911,
		CreatedAt: 1100,
		Type:      LogTypeConsume,
		Quota:     300,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":        "subscription",
			"subscription_plan_id":  1011,
			"subscription_consumed": 300,
		}),
	})

	result, err := GetSubscriptionAnalysis(SubscriptionAnalysisFilters{
		StartTimestamp: 1050,
		EndTimestamp:   1200,
	})
	require.NoError(t, err)

	require.Len(t, result.Plans, 1)
	assert.EqualValues(t, 300, result.Summary.HistoricalUsedTotal)
	assert.EqualValues(t, 300, result.Plans[0].HistoricalUsedTotal)
	assert.EqualValues(t, 300, result.Plans[0].CurrentUsedTotal)
	assert.EqualValues(t, 700, result.Plans[0].CurrentRemainingTotal)
}
