package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withGroupBillingTypeSetting(t *testing.T) {
	t.Helper()
	old := GroupBillingType2JSONString()
	t.Cleanup(func() {
		_ = UpdateGroupBillingTypeByJSONString(old)
	})
	groupBillingTypeMap.Clear()
}

func TestGroupBillingTypeDefaultsAndValidation(t *testing.T) {
	withGroupBillingTypeSetting(t)

	assert.Equal(t, GroupBillingTypeDefault, GetGroupBillingType(""))
	assert.Equal(t, GroupBillingTypeDefault, GetGroupBillingType("unknown"))
	assert.NoError(t, CheckGroupBillingType(""))
	assert.Error(t, CheckGroupBillingType(`{"vip":"bad"}`))
	assert.Error(t, CheckGroupBillingType(`{"":"subscription_only"}`))
}

func TestUpdateGroupBillingTypeByJSONString(t *testing.T) {
	withGroupBillingTypeSetting(t)

	require.NoError(t, UpdateGroupBillingTypeByJSONString(`{"vip":"subscription_only","legacy":"wallet_only","normal":"default"}`))

	assert.Equal(t, GroupBillingTypeSubscriptionOnly, GetGroupBillingType("vip"))
	assert.Equal(t, GroupBillingTypeWalletOnly, GetGroupBillingType("legacy"))
	assert.Equal(t, GroupBillingTypeDefault, GetGroupBillingType("normal"))
	assert.Equal(t, GroupBillingTypeDefault, GetGroupBillingType("missing"))

	require.NoError(t, UpdateGroupBillingTypeByJSONString(""))
	assert.Equal(t, GroupBillingTypeDefault, GetGroupBillingType("vip"))
}
