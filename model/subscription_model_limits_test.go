package model

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertSubscriptionLimitUser(t *testing.T, id int) {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id:       id,
		Username: "subscription_limit_user",
		Status:   common.UserStatusEnabled,
	}).Error)
}

func insertSubscriptionLimitPlan(t *testing.T, id int, total int64, limits string, resetPeriod string) *SubscriptionPlan {
	t.Helper()
	plan := &SubscriptionPlan{
		Id:                id,
		Title:             "Limit Plan",
		PriceAmount:       9.99,
		Currency:          "USD",
		DurationUnit:      SubscriptionDurationMonth,
		DurationValue:     1,
		Enabled:           true,
		UserVisible:       true,
		TotalAmount:       total,
		ModelAmountLimits: limits,
		QuotaResetPeriod:  resetPeriod,
	}
	require.NoError(t, DB.Create(plan).Error)
	InvalidateSubscriptionPlanCache(id)
	return plan
}

func insertActiveUserSubscriptionForLimitTest(t *testing.T, id int, userId int, planId int, total int64, used int64) *UserSubscription {
	t.Helper()
	now := time.Now().Unix()
	sub := &UserSubscription{
		Id:          id,
		UserId:      userId,
		PlanId:      planId,
		AmountTotal: total,
		AmountUsed:  used,
		StartTime:   now - 60,
		EndTime:     now + 30*24*3600,
		Status:      "active",
	}
	require.NoError(t, DB.Create(sub).Error)
	return sub
}

func getSubscriptionLimitUsed(t *testing.T, subId int) int64 {
	t.Helper()
	var sub UserSubscription
	require.NoError(t, DB.Select("amount_used").Where("id = ?", subId).First(&sub).Error)
	return sub.AmountUsed
}

func getSubscriptionModelLimitUsed(t *testing.T, subId int, modelName string) int64 {
	t.Helper()
	var usage UserSubscriptionModelUsage
	require.NoError(t, DB.Where("user_subscription_id = ? AND model_name = ?", subId, modelName).First(&usage).Error)
	return usage.AmountUsed
}

func TestGetAllUserSubscriptionsIncludesHiddenPlansForSelfState(t *testing.T) {
	truncateTables(t)

	insertSubscriptionLimitUser(t, 514)
	visiblePlan := insertSubscriptionLimitPlan(t, 616, 1000, "", SubscriptionResetNever)
	hiddenPlan := insertSubscriptionLimitPlan(t, 617, 1000, "", SubscriptionResetNever)
	hiddenPlan.UserVisible = false
	require.NoError(t, DB.Save(hiddenPlan).Error)
	InvalidateSubscriptionPlanCache(hiddenPlan.Id)

	insertActiveUserSubscriptionForLimitTest(t, 716, 514, visiblePlan.Id, 1000, 0)
	insertActiveUserSubscriptionForLimitTest(t, 717, 514, hiddenPlan.Id, 1000, 0)

	allSubscriptions, err := GetAllUserSubscriptions(514)
	require.NoError(t, err)
	require.Len(t, allSubscriptions, 2)
	require.NotNil(t, allSubscriptions[0].Plan)
	require.NotNil(t, allSubscriptions[1].Plan)
	assert.ElementsMatch(t, []int{visiblePlan.Id, hiddenPlan.Id}, []int{allSubscriptions[0].Plan.Id, allSubscriptions[1].Plan.Id})

	activeSubscriptions, err := GetAllActiveUserSubscriptions(514)
	require.NoError(t, err)
	require.Len(t, activeSubscriptions, 2)
	require.NotNil(t, activeSubscriptions[0].Plan)
	require.NotNil(t, activeSubscriptions[1].Plan)
	assert.ElementsMatch(t, []int{visiblePlan.Id, hiddenPlan.Id}, []int{activeSubscriptions[0].Plan.Id, activeSubscriptions[1].Plan.Id})

	hasActive, err := HasActiveUserSubscription(514)
	require.NoError(t, err)
	assert.True(t, hasActive)
}

func TestPreConsumeUserSubscription_ModelLimitRefundsBothCounters(t *testing.T) {
	truncateTables(t)

	insertSubscriptionLimitUser(t, 501)
	insertSubscriptionLimitPlan(t, 601, 10000, `{"gpt-test":3000,"*":1000}`, SubscriptionResetNever)
	insertActiveUserSubscriptionForLimitTest(t, 701, 501, 601, 10000, 0)

	res, err := PreConsumeUserSubscription("req-model-limit-refund", 501, "gpt-test", 0, 2000)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, 701, res.UserSubscriptionId)
	assert.True(t, res.ModelLimitMatched)
	assert.EqualValues(t, 3000, res.ModelAmountLimit)
	assert.EqualValues(t, 2000, res.AmountUsedAfter)
	assert.EqualValues(t, 2000, res.ModelAmountUsedAfter)
	assert.EqualValues(t, 2000, getSubscriptionLimitUsed(t, 701))
	assert.EqualValues(t, 2000, getSubscriptionModelLimitUsed(t, 701, "gpt-test"))

	require.NoError(t, RefundSubscriptionPreConsume("req-model-limit-refund"))
	assert.EqualValues(t, 0, getSubscriptionLimitUsed(t, 701))
	assert.EqualValues(t, 0, getSubscriptionModelLimitUsed(t, 701, "gpt-test"))

	var record SubscriptionPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", "req-model-limit-refund").First(&record).Error)
	assert.Equal(t, "refunded", record.Status)
	assert.Equal(t, "gpt-test", record.ModelName)
}

func TestPreConsumeUserSubscription_ModelLimitInsufficientDoesNotConsume(t *testing.T) {
	truncateTables(t)

	insertSubscriptionLimitUser(t, 502)
	insertSubscriptionLimitPlan(t, 602, 10000, `{"gpt-test":1000}`, SubscriptionResetNever)
	insertActiveUserSubscriptionForLimitTest(t, 702, 502, 602, 10000, 0)

	res, err := PreConsumeUserSubscription("req-model-limit-insufficient", 502, "gpt-test", 0, 1500)
	require.Nil(t, res)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSubscriptionModelQuotaInsufficient))
	assert.EqualValues(t, 0, getSubscriptionLimitUsed(t, 702))

	var usageCount int64
	require.NoError(t, DB.Model(&UserSubscriptionModelUsage{}).Where("user_subscription_id = ?", 702).Count(&usageCount).Error)
	assert.EqualValues(t, 0, usageCount)
}

func TestFindSubscriptionModelLimit_PrefixWildcardPriority(t *testing.T) {
	plan := &SubscriptionPlan{
		ModelAmountLimits: `{"gpt-5.5-mini":8000,"gpt-5.5*":5000,"gpt-5*":9000,"*":1000}`,
	}

	limit, matched, err := findSubscriptionModelLimit(plan, "gpt-5.5-mini")
	require.NoError(t, err)
	assert.True(t, matched)
	assert.EqualValues(t, 8000, limit)
	assert.Equal(t, "gpt-5.5-mini", matchSubscriptionModelLimitKey(map[string]int64{
		"gpt-5.5-mini": 8000,
		"gpt-5.5*":     5000,
		"gpt-5*":       9000,
		"*":            1000,
	}, "gpt-5.5-mini"))

	limit, matched, err = findSubscriptionModelLimit(plan, "gpt-5.5-pro")
	require.NoError(t, err)
	assert.True(t, matched)
	assert.EqualValues(t, 5000, limit)

	limit, matched, err = findSubscriptionModelLimit(plan, "gpt-5-nano")
	require.NoError(t, err)
	assert.True(t, matched)
	assert.EqualValues(t, 9000, limit)

	limit, matched, err = findSubscriptionModelLimit(plan, "claude-sonnet-4")
	require.NoError(t, err)
	assert.True(t, matched)
	assert.EqualValues(t, 1000, limit)
}

func TestPreConsumeUserSubscription_WildcardLimitsAreSharedAcrossMatchingModels(t *testing.T) {
	truncateTables(t)

	insertSubscriptionLimitUser(t, 511)
	insertSubscriptionLimitPlan(t, 611, 10000, `{"gpt-5.5*":1000,"*":500}`, SubscriptionResetNever)
	insertActiveUserSubscriptionForLimitTest(t, 711, 511, 611, 10000, 0)

	res, err := PreConsumeUserSubscription("req-prefix-wildcard-1", 511, "gpt-5.5", 0, 600)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.ModelLimitMatched)
	assert.EqualValues(t, 1000, res.ModelAmountLimit)
	assert.EqualValues(t, 600, res.ModelAmountUsedAfter)

	res, err = PreConsumeUserSubscription("req-prefix-wildcard-2", 511, "gpt-5.5-mini", 0, 400)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.EqualValues(t, 1000, res.ModelAmountUsedAfter)

	res, err = PreConsumeUserSubscription("req-prefix-wildcard-3", 511, "gpt-5.5-pro", 0, 1)
	require.Nil(t, res)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSubscriptionModelQuotaInsufficient))
	assert.EqualValues(t, 1000, getSubscriptionLimitUsed(t, 711))

	res, err = PreConsumeUserSubscription("req-other-wildcard-1", 511, "claude-sonnet-4", 0, 300)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.EqualValues(t, 500, res.ModelAmountLimit)
	assert.EqualValues(t, 300, res.ModelAmountUsedAfter)

	res, err = PreConsumeUserSubscription("req-other-wildcard-2", 511, "gemini-2.5-pro", 0, 250)
	require.Nil(t, res)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSubscriptionModelQuotaInsufficient))
	assert.EqualValues(t, 1300, getSubscriptionLimitUsed(t, 711))
}

func TestPreConsumeUserSubscription_ConcurrentBoundaryDoesNotExceedModelLimit(t *testing.T) {
	truncateTables(t)

	insertSubscriptionLimitUser(t, 506)
	insertSubscriptionLimitPlan(t, 606, 500, `{"gpt-test":500}`, SubscriptionResetNever)
	insertActiveUserSubscriptionForLimitTest(t, 706, 506, 606, 500, 0)

	const workers = 10
	const amount = 100
	var successCount atomic.Int32
	var insufficientCount atomic.Int32
	errs := make(chan error, workers)

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			_, err := PreConsumeUserSubscription(fmt.Sprintf("req-concurrent-boundary-%d", idx), 506, "gpt-test", 0, amount)
			if err == nil {
				successCount.Add(1)
				return
			}
			if errors.Is(err, ErrSubscriptionModelQuotaInsufficient) || errors.Is(err, ErrSubscriptionQuotaInsufficient) {
				insufficientCount.Add(1)
				return
			}
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	assert.EqualValues(t, 5, successCount.Load())
	assert.EqualValues(t, 5, insufficientCount.Load())
	assert.EqualValues(t, 500, getSubscriptionLimitUsed(t, 706))
	assert.EqualValues(t, 500, getSubscriptionModelLimitUsed(t, 706, "gpt-test"))
}

func TestPreConsumeUserSubscription_ZeroAmountRequiresSubscriptionAndRefundsRecord(t *testing.T) {
	truncateTables(t)

	_, err := PreConsumeUserSubscription("req-zero-no-subscription", 503, "gpt-test", 0, 0)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoActiveSubscription))

	insertSubscriptionLimitUser(t, 503)
	insertSubscriptionLimitPlan(t, 603, 10000, `{"gpt-test":1000}`, SubscriptionResetNever)
	insertActiveUserSubscriptionForLimitTest(t, 703, 503, 603, 10000, 0)

	res, err := PreConsumeUserSubscription("req-zero-subscription", 503, "gpt-test", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.EqualValues(t, 0, res.PreConsumed)
	assert.EqualValues(t, 0, getSubscriptionLimitUsed(t, 703))
	assert.EqualValues(t, 0, getSubscriptionModelLimitUsed(t, 703, "gpt-test"))

	require.NoError(t, RefundSubscriptionPreConsume("req-zero-subscription"))
	var record SubscriptionPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", "req-zero-subscription").First(&record).Error)
	assert.Equal(t, "refunded", record.Status)
}

func TestPostConsumeUserSubscriptionModelDelta_AllowsOverLimitThenBlocksNextPreConsume(t *testing.T) {
	truncateTables(t)

	insertSubscriptionLimitUser(t, 504)
	insertSubscriptionLimitPlan(t, 604, 10000, `{"gpt-test":1000}`, SubscriptionResetNever)
	insertActiveUserSubscriptionForLimitTest(t, 704, 504, 604, 10000, 0)

	_, err := PreConsumeUserSubscription("req-over-limit-initial", 504, "gpt-test", 0, 500)
	require.NoError(t, err)
	require.NoError(t, PostConsumeUserSubscriptionModelDelta(704, "gpt-test", 800, false))
	assert.EqualValues(t, 1300, getSubscriptionLimitUsed(t, 704))
	assert.EqualValues(t, 1300, getSubscriptionModelLimitUsed(t, 704, "gpt-test"))

	_, err = PreConsumeUserSubscription("req-over-limit-next", 504, "gpt-test", 0, 1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSubscriptionModelQuotaInsufficient))
}

func TestResetDueSubscriptions_ClearsModelUsage(t *testing.T) {
	truncateTables(t)

	insertSubscriptionLimitUser(t, 505)
	insertSubscriptionLimitPlan(t, 605, 10000, `{"gpt-test":5000}`, SubscriptionResetDaily)
	sub := insertActiveUserSubscriptionForLimitTest(t, 705, 505, 605, 10000, 2500)
	sub.LastResetTime = time.Now().Add(-48 * time.Hour).Unix()
	sub.NextResetTime = time.Now().Add(-24 * time.Hour).Unix()
	require.NoError(t, DB.Save(sub).Error)
	require.NoError(t, DB.Create(&UserSubscriptionModelUsage{
		UserSubscriptionId: 705,
		UserId:             505,
		ModelName:          "gpt-test",
		AmountUsed:         2500,
	}).Error)

	count, err := ResetDueSubscriptions(10)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.EqualValues(t, 0, getSubscriptionLimitUsed(t, 705))
	assert.EqualValues(t, 0, getSubscriptionModelLimitUsed(t, 705, "gpt-test"))
}

func TestGetAllUserSubscriptionsIncludesModelLimitUsageSummary(t *testing.T) {
	truncateTables(t)

	insertSubscriptionLimitUser(t, 509)
	insertSubscriptionLimitPlan(t, 609, 0, `{"gpt-test":5000,"*":1000}`, SubscriptionResetNever)
	insertActiveUserSubscriptionForLimitTest(t, 709, 509, 609, 0, 800)
	require.NoError(t, DB.Create(&UserSubscriptionModelUsage{
		UserSubscriptionId: 709,
		UserId:             509,
		ModelName:          "gpt-test",
		AmountUsed:         500,
	}).Error)
	require.NoError(t, DB.Create(&UserSubscriptionModelUsage{
		UserSubscriptionId: 709,
		UserId:             509,
		ModelName:          "other-model",
		AmountUsed:         300,
	}).Error)

	summaries, err := GetAllUserSubscriptions(509)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	require.NotNil(t, summaries[0].Plan)
	assert.EqualValues(t, 5000, summaries[0].ModelAmountLimits["gpt-test"])
	assert.EqualValues(t, 1000, summaries[0].ModelAmountLimits["*"])
	assert.EqualValues(t, 500, summaries[0].ModelAmountUsages["gpt-test"])
	assert.EqualValues(t, 300, summaries[0].ModelAmountUsages["other-model"])
	assert.EqualValues(t, 500, summaries[0].ModelAmountLimitUsages["gpt-test"])
	assert.EqualValues(t, 300, summaries[0].ModelAmountLimitUsages["*"])
}

func TestGetAllUserSubscriptionsByPlanIncludesUsersAndModelLimitUsage(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&User{
		Id:          513,
		Username:    "plan_usage_user_a",
		DisplayName: "Plan User A",
		Email:       "plan-user-a@example.com",
		Group:       "default",
		AffCode:     "plan_usage_a",
		Status:      common.UserStatusEnabled,
	}).Error)
	require.NoError(t, DB.Create(&User{
		Id:       514,
		Username: "plan_usage_user_b",
		Group:    "vip",
		AffCode:  "plan_usage_b",
		Status:   common.UserStatusEnabled,
	}).Error)
	insertSubscriptionLimitPlan(t, 613, 0, `{"gpt-5.5*":1000,"*":500}`, SubscriptionResetNever)
	insertSubscriptionLimitPlan(t, 614, 0, `{"gpt-other":1000}`, SubscriptionResetNever)
	insertActiveUserSubscriptionForLimitTest(t, 713, 513, 613, 0, 200)
	insertActiveUserSubscriptionForLimitTest(t, 714, 514, 613, 0, 300)
	insertActiveUserSubscriptionForLimitTest(t, 715, 514, 614, 0, 400)
	require.NoError(t, DB.Create(&UserSubscriptionModelUsage{
		UserSubscriptionId: 713,
		UserId:             513,
		ModelName:          "gpt-5.5-mini",
		AmountUsed:         200,
	}).Error)
	require.NoError(t, DB.Create(&UserSubscriptionModelUsage{
		UserSubscriptionId: 714,
		UserId:             514,
		ModelName:          "claude-sonnet-4",
		AmountUsed:         300,
	}).Error)

	summaries, err := GetAllUserSubscriptionsByPlan(613)
	require.NoError(t, err)
	require.Len(t, summaries, 2)

	bySubId := map[int]SubscriptionSummaryWithUser{}
	for _, summary := range summaries {
		require.NotNil(t, summary.Subscription)
		bySubId[summary.Subscription.Id] = summary
	}

	require.NotNil(t, bySubId[713].User)
	assert.Equal(t, "plan_usage_user_a", bySubId[713].User.Username)
	assert.Equal(t, "Plan User A", bySubId[713].User.DisplayName)
	assert.Equal(t, "default", bySubId[713].User.Group)
	assert.EqualValues(t, 200, bySubId[713].ModelAmountLimitUsages["gpt-5.5*"])
	assert.EqualValues(t, 0, bySubId[713].ModelAmountLimitUsages["*"])

	require.NotNil(t, bySubId[714].User)
	assert.Equal(t, "plan_usage_user_b", bySubId[714].User.Username)
	assert.Equal(t, "vip", bySubId[714].User.Group)
	assert.EqualValues(t, 0, bySubId[714].ModelAmountLimitUsages["gpt-5.5*"])
	assert.EqualValues(t, 300, bySubId[714].ModelAmountLimitUsages["*"])
}

func TestGetSubscriptionPlanHistoricalUsageStatsAggregatesConsumeLogs(t *testing.T) {
	truncateTables(t)

	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    801,
		CreatedAt: time.Now().Unix(),
		Type:      LogTypeConsume,
		Quota:     120,
		RequestId: "req-sub-1",
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":        "subscription",
			"subscription_plan_id":  901,
			"subscription_consumed": 120,
		}),
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    801,
		CreatedAt: time.Now().Unix(),
		Type:      LogTypeConsume,
		Quota:     30,
		RequestId: "req-sub-2",
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":       "subscription",
			"subscription_plan_id": 901,
		}),
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    802,
		CreatedAt: time.Now().Unix(),
		Type:      LogTypeConsume,
		Quota:     80,
		RequestId: "req-sub-3",
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":        "subscription",
			"subscription_plan_id":  901,
			"subscription_consumed": 80,
		}),
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    802,
		CreatedAt: time.Now().Unix(),
		Type:      LogTypeRefund,
		Quota:     20,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":       "subscription",
			"subscription_plan_id": 901,
		}),
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    802,
		CreatedAt: time.Now().Unix(),
		Type:      LogTypeConsume,
		Quota:     15,
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":        "subscription",
			"subscription_plan_id":  901,
			"subscription_consumed": 15,
			"task_id":               "task_1",
			"pre_consumed_quota":    100,
			"actual_quota":          115,
		}),
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    803,
		CreatedAt: time.Now().Unix(),
		Type:      LogTypeConsume,
		Quota:     999,
		RequestId: "req-wallet-1",
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":        "wallet",
			"subscription_plan_id":  901,
			"subscription_consumed": 999,
		}),
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    803,
		CreatedAt: time.Now().Unix(),
		Type:      LogTypeConsume,
		Quota:     50,
		RequestId: "req-violation-1",
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":        "subscription",
			"subscription_plan_id":  901,
			"subscription_consumed": 50,
			"violation_fee":         true,
		}),
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    804,
		CreatedAt: time.Now().Unix(),
		Type:      LogTypeConsume,
		Quota:     777,
		RequestId: "req-sub-4",
		Other: common.MapToJsonStr(map[string]interface{}{
			"billing_source":        "subscription",
			"subscription_plan_id":  902,
			"subscription_consumed": 777,
		}),
	}).Error)

	stats, err := GetSubscriptionPlanHistoricalUsageStats(901)
	require.NoError(t, err)
	assert.EqualValues(t, 275, stats.HistoricalUsedTotal)
	assert.EqualValues(t, 150, stats.HistoricalUsedByUser[801])
	assert.EqualValues(t, 75, stats.HistoricalUsedByUser[802])
	assert.EqualValues(t, 50, stats.HistoricalUsedByUser[803])
	assert.EqualValues(t, 3, stats.HistoricalCallCountTotal)
	assert.EqualValues(t, 2, stats.HistoricalCallCountByUser[801])
	assert.EqualValues(t, 1, stats.HistoricalCallCountByUser[802])
	_, exists := stats.HistoricalCallCountByUser[803]
	assert.False(t, exists)
}

func TestGetAllUserSubscriptionsGroupsPrefixWildcardUsageSummary(t *testing.T) {
	truncateTables(t)

	insertSubscriptionLimitUser(t, 512)
	insertSubscriptionLimitPlan(t, 612, 0, `{"gpt-5.5-mini":700,"gpt-5.5*":1000,"*":500}`, SubscriptionResetNever)
	insertActiveUserSubscriptionForLimitTest(t, 712, 512, 612, 0, 800)
	require.NoError(t, DB.Create(&UserSubscriptionModelUsage{
		UserSubscriptionId: 712,
		UserId:             512,
		ModelName:          "gpt-5.5-pro",
		AmountUsed:         300,
	}).Error)
	require.NoError(t, DB.Create(&UserSubscriptionModelUsage{
		UserSubscriptionId: 712,
		UserId:             512,
		ModelName:          "gpt-5.5-turbo",
		AmountUsed:         200,
	}).Error)
	require.NoError(t, DB.Create(&UserSubscriptionModelUsage{
		UserSubscriptionId: 712,
		UserId:             512,
		ModelName:          "gpt-5.5-mini",
		AmountUsed:         100,
	}).Error)
	require.NoError(t, DB.Create(&UserSubscriptionModelUsage{
		UserSubscriptionId: 712,
		UserId:             512,
		ModelName:          "other-model",
		AmountUsed:         50,
	}).Error)

	summaries, err := GetAllUserSubscriptions(512)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.EqualValues(t, 500, summaries[0].ModelAmountLimitUsages["gpt-5.5*"])
	assert.EqualValues(t, 100, summaries[0].ModelAmountLimitUsages["gpt-5.5-mini"])
	assert.EqualValues(t, 50, summaries[0].ModelAmountLimitUsages["*"])
}

func TestSubscriptionLifecycle_OverlappingUpgradeSubscriptionsDowngradeAfterLastExpires(t *testing.T) {
	truncateTables(t)

	require.NoError(t, DB.Create(&User{
		Id:       507,
		Username: "subscription_lifecycle_user",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	plan := insertSubscriptionLimitPlan(t, 607, 0, "", SubscriptionResetNever)
	plan.UpgradeGroup = "vip"
	require.NoError(t, DB.Save(plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)

	var first *UserSubscription
	var err error
	first, err = CreateUserSubscriptionFromPlanTx(DB, 507, plan, "admin")
	require.NoError(t, err)
	require.Equal(t, "default", first.PrevUserGroup)
	require.Equal(t, "vip", getSubscriptionLifecycleUserGroup(t, 507))

	var second *UserSubscription
	second, err = CreateUserSubscriptionFromPlanTx(DB, 507, plan, "admin")
	require.NoError(t, err)
	require.Equal(t, "default", second.PrevUserGroup)

	now := common.GetTimestamp()
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", first.Id).Updates(map[string]interface{}{
		"end_time": now - 20,
	}).Error)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", second.Id).Updates(map[string]interface{}{
		"end_time": now + 3600,
	}).Error)

	count, err := ExpireDueSubscriptions(10)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, "vip", getSubscriptionLifecycleUserGroup(t, 507))

	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", second.Id).Updates(map[string]interface{}{
		"end_time": now - 10,
	}).Error)
	count, err = ExpireDueSubscriptions(10)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, "default", getSubscriptionLifecycleUserGroup(t, 507))
}

func TestSubscriptionLifecycle_ResetAfterEndIsNotScheduledAndExpiredCannotConsume(t *testing.T) {
	truncateTables(t)

	insertSubscriptionLimitUser(t, 508)
	plan := insertSubscriptionLimitPlan(t, 608, 1000, `{"gpt-test":1000}`, SubscriptionResetCustom)
	plan.DurationUnit = SubscriptionDurationCustom
	plan.DurationValue = 0
	plan.CustomSeconds = 60
	plan.QuotaResetCustomSeconds = 120
	require.NoError(t, DB.Save(plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)

	var sub *UserSubscription
	sub, err := CreateUserSubscriptionFromPlanTx(DB, 508, plan, "admin")
	require.NoError(t, err)
	require.Zero(t, sub.NextResetTime)

	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).Updates(map[string]interface{}{
		"end_time": common.GetTimestamp() - 1,
	}).Error)
	_, err = PreConsumeUserSubscription("req-expired-lifecycle", 508, "gpt-test", 0, 1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoActiveSubscription))
}

func getSubscriptionLifecycleUserGroup(t *testing.T, userId int) string {
	t.Helper()
	var group string
	require.NoError(t, DB.Model(&User{}).Where("id = ?", userId).Select(commonGroupCol).Find(&group).Error)
	return group
}
