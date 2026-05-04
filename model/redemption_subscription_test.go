package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertRedemptionTestUser(t *testing.T, id int, group string, quota int) {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id:       id,
		Username: "redemption_user",
		Status:   common.UserStatusEnabled,
		Group:    group,
		Quota:    quota,
	}).Error)
}

func TestRedeemQuotaTypePreservesExistingTopUpBehavior(t *testing.T) {
	truncateTables(t)

	insertRedemptionTestUser(t, 801, "default", 100)
	require.NoError(t, DB.Create(&Redemption{
		Key:         "quota-redemption",
		Status:      common.RedemptionCodeStatusEnabled,
		Type:        RedemptionTypeQuota,
		Name:        "Quota Code",
		Quota:       250,
		CreatedTime: common.GetTimestamp(),
	}).Error)

	result, err := Redeem("quota-redemption", 801)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, RedemptionTypeQuota, result.Type)
	assert.Equal(t, 250, result.Quota)
	assert.Nil(t, result.Subscription)
	assert.Nil(t, result.Plan)

	var user User
	require.NoError(t, DB.Select("quota").Where("id = ?", 801).First(&user).Error)
	assert.Equal(t, 350, user.Quota)

	var redeemed Redemption
	require.NoError(t, DB.Where("key = ?", "quota-redemption").First(&redeemed).Error)
	assert.Equal(t, common.RedemptionCodeStatusUsed, redeemed.Status)
	assert.Equal(t, 801, redeemed.UsedUserId)
	assert.NotZero(t, redeemed.RedeemedTime)
}

func TestRedeemBlankTypeDefaultsToQuotaForLegacyCodes(t *testing.T) {
	truncateTables(t)

	insertRedemptionTestUser(t, 802, "default", 0)
	require.NoError(t, DB.Create(&Redemption{
		Key:         "legacy-redemption",
		Status:      common.RedemptionCodeStatusEnabled,
		Name:        "Legacy Code",
		Quota:       500,
		CreatedTime: common.GetTimestamp(),
	}).Error)

	result, err := Redeem("legacy-redemption", 802)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, RedemptionTypeQuota, result.Type)
	assert.Equal(t, 500, result.Quota)
}

func TestRedeemInvalidTypeFailsWithoutConsumingCode(t *testing.T) {
	truncateTables(t)

	insertRedemptionTestUser(t, 805, "default", 0)
	require.NoError(t, DB.Create(&Redemption{
		Key:         "invalid-type-redemption",
		Status:      common.RedemptionCodeStatusEnabled,
		Type:        "unknown",
		Name:        "Invalid Type Code",
		Quota:       500,
		CreatedTime: common.GetTimestamp(),
	}).Error)

	result, err := Redeem("invalid-type-redemption", 805)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRedeemFailed))
	assert.Nil(t, result)

	var redeemed Redemption
	require.NoError(t, DB.Where("key = ?", "invalid-type-redemption").First(&redeemed).Error)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, redeemed.Status)
	assert.Zero(t, redeemed.UsedUserId)
}

func TestRedeemSubscriptionTypeCreatesSubscriptionAndUpgradesGroup(t *testing.T) {
	truncateTables(t)

	insertRedemptionTestUser(t, 803, "default", 0)
	plan := insertSubscriptionLimitPlan(t, 803, 12345, "", SubscriptionResetNever)
	plan.Title = "Redeemable Plan"
	plan.UpgradeGroup = "vip"
	require.NoError(t, DB.Save(plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	require.NoError(t, DB.Create(&Redemption{
		Key:                "subscription-redemption",
		Status:             common.RedemptionCodeStatusEnabled,
		Type:               RedemptionTypeSubscription,
		Name:               "Subscription Code",
		SubscriptionPlanId: plan.Id,
		CreatedTime:        common.GetTimestamp(),
	}).Error)

	result, err := Redeem("subscription-redemption", 803)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, RedemptionTypeSubscription, result.Type)
	assert.Zero(t, result.Quota)
	require.NotNil(t, result.Subscription)
	require.NotNil(t, result.Plan)
	assert.Equal(t, plan.Id, result.Subscription.PlanId)
	assert.Equal(t, "redemption", result.Subscription.Source)
	assert.EqualValues(t, 12345, result.Subscription.AmountTotal)
	assert.Equal(t, "Redeemable Plan", result.Plan.Title)
	assert.Equal(t, "vip", getSubscriptionLifecycleUserGroup(t, 803))

	var user User
	require.NoError(t, DB.Select("quota").Where("id = ?", 803).First(&user).Error)
	assert.Zero(t, user.Quota)

	var redeemed Redemption
	require.NoError(t, DB.Where("key = ?", "subscription-redemption").First(&redeemed).Error)
	assert.Equal(t, common.RedemptionCodeStatusUsed, redeemed.Status)
	assert.Equal(t, 803, redeemed.UsedUserId)
	assert.NotZero(t, redeemed.RedeemedTime)
}

func TestRedeemSubscriptionTypeRespectsMaxPurchasePerUser(t *testing.T) {
	truncateTables(t)

	insertRedemptionTestUser(t, 804, "default", 0)
	plan := insertSubscriptionLimitPlan(t, 804, 1000, "", SubscriptionResetNever)
	plan.MaxPurchasePerUser = 1
	require.NoError(t, DB.Save(plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	_, err := CreateUserSubscriptionFromPlanTx(DB, 804, plan, "admin")
	require.NoError(t, err)
	require.NoError(t, DB.Create(&Redemption{
		Key:                "subscription-limit-redemption",
		Status:             common.RedemptionCodeStatusEnabled,
		Type:               RedemptionTypeSubscription,
		Name:               "Subscription Limit Code",
		SubscriptionPlanId: plan.Id,
		CreatedTime:        common.GetTimestamp(),
	}).Error)

	result, err := Redeem("subscription-limit-redemption", 804)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRedeemFailed))
	assert.Nil(t, result)

	var redeemed Redemption
	require.NoError(t, DB.Where("key = ?", "subscription-limit-redemption").First(&redeemed).Error)
	assert.Equal(t, common.RedemptionCodeStatusEnabled, redeemed.Status)
	assert.Zero(t, redeemed.UsedUserId)
}
