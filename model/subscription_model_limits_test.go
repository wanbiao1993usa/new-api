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
