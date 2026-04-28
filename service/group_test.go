package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withGroupSettings(t *testing.T) {
	t.Helper()
	oldUsable := setting.UserUsableGroups2JSONString()
	oldVisible := setting.UserVisibleGroups2JSONString()
	groupSetting := ratio_setting.GetGroupRatioSetting()
	oldSpecialUsable := groupSetting.GroupSpecialUsableGroup.ReadAll()
	oldSpecialVisible := groupSetting.GroupSpecialVisibleGroup.ReadAll()

	t.Cleanup(func() {
		_ = setting.UpdateUserUsableGroupsByJSONString(oldUsable)
		_ = setting.UpdateUserVisibleGroupsByJSONString(oldVisible)
		groupSetting.GroupSpecialUsableGroup.Clear()
		groupSetting.GroupSpecialUsableGroup.AddAll(oldSpecialUsable)
		groupSetting.GroupSpecialVisibleGroup.Clear()
		groupSetting.GroupSpecialVisibleGroup.AddAll(oldSpecialVisible)
	})

	groupSetting.GroupSpecialUsableGroup.Clear()
	groupSetting.GroupSpecialVisibleGroup.Clear()
}

func TestGetUserVisibleGroupsDefaultsToAccessibleGroups(t *testing.T) {
	withGroupSettings(t)
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"legacy":"Legacy","standard":"Standard"}`))
	require.NoError(t, setting.UpdateUserVisibleGroupsByJSONString(""))
	ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Set("vip", map[string]string{
		"+:premium":  "Premium",
		"-:standard": "remove",
	})

	accessible := GetUserAccessibleGroups("vip")
	visible := GetUserVisibleGroups("vip")

	assert.Equal(t, accessible, visible)
	assert.Contains(t, visible, "legacy")
	assert.Contains(t, visible, "premium")
	assert.NotContains(t, visible, "standard")
	assert.Contains(t, visible, "vip")
}

func TestGetUserVisibleGroupsCanHideAccessibleGroups(t *testing.T) {
	withGroupSettings(t)
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"legacy":"Legacy","unlimited":"Unlimited"}`))
	require.NoError(t, setting.UpdateUserVisibleGroupsByJSONString(`{"unlimited":"Unlimited"}`))

	accessible := GetUserAccessibleGroups("unlimited")
	visible := GetUserVisibleGroups("unlimited")

	assert.Contains(t, accessible, "legacy")
	assert.Contains(t, accessible, "unlimited")
	assert.NotContains(t, visible, "legacy")
	assert.Contains(t, visible, "unlimited")
}

func TestGetUserVisibleGroupsAppliesSpecialVisibleRules(t *testing.T) {
	withGroupSettings(t)
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"legacy":"Legacy","standard":"Standard"}`))
	require.NoError(t, setting.UpdateUserVisibleGroupsByJSONString(`{"standard":"Standard"}`))
	ratio_setting.GetGroupRatioSetting().GroupSpecialVisibleGroup.Set("vip", map[string]string{
		"+:exclusive": "Exclusive",
		"-:standard":  "remove",
	})

	visible := GetUserVisibleGroups("vip")

	assert.NotContains(t, visible, "legacy")
	assert.NotContains(t, visible, "standard")
	assert.Contains(t, visible, "exclusive")
	assert.Contains(t, visible, "vip")
}

func TestResolveBillingGroupPrefersAutoGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyAutoGroup, "auto-real")

	group := ResolveBillingGroup(ctx, &relaycommon.RelayInfo{UsingGroup: "token-group"})

	assert.Equal(t, "auto-real", group)
}

func TestResolveGroupBillingTypeUsesResolvedGroup(t *testing.T) {
	old := ratio_setting.GroupBillingType2JSONString()
	t.Cleanup(func() {
		_ = ratio_setting.UpdateGroupBillingTypeByJSONString(old)
	})
	require.NoError(t, ratio_setting.UpdateGroupBillingTypeByJSONString(`{"auto-real":"subscription_only","token-group":"wallet_only"}`))

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyAutoGroup, "auto-real")

	group, billingType := ResolveGroupBillingType(ctx, &relaycommon.RelayInfo{UsingGroup: "token-group"})

	assert.Equal(t, "auto-real", group)
	assert.Equal(t, ratio_setting.GroupBillingTypeSubscriptionOnly, billingType)
}
