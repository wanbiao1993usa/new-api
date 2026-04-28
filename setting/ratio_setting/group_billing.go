package ratio_setting

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
)

const (
	GroupBillingTypeDefault          = "default"
	GroupBillingTypeSubscriptionOnly = "subscription_only"
	GroupBillingTypeWalletOnly       = "wallet_only"
)

var groupBillingTypeMap = types.NewRWMap[string, string]()

func NormalizeGroupBillingType(value string) string {
	switch strings.TrimSpace(value) {
	case GroupBillingTypeSubscriptionOnly:
		return GroupBillingTypeSubscriptionOnly
	case GroupBillingTypeWalletOnly:
		return GroupBillingTypeWalletOnly
	default:
		return GroupBillingTypeDefault
	}
}

func GetGroupBillingType(group string) string {
	if group == "" {
		return GroupBillingTypeDefault
	}
	value, ok := groupBillingTypeMap.Get(group)
	if !ok {
		return GroupBillingTypeDefault
	}
	return NormalizeGroupBillingType(value)
}

func GetGroupBillingTypeCopy() map[string]string {
	return groupBillingTypeMap.ReadAll()
}

func GroupBillingType2JSONString() string {
	return groupBillingTypeMap.MarshalJSONString()
}

func CheckGroupBillingType(jsonStr string) error {
	if strings.TrimSpace(jsonStr) == "" {
		return nil
	}
	checkMap := make(map[string]string)
	if err := common.UnmarshalJsonStr(jsonStr, &checkMap); err != nil {
		return err
	}
	for group, billingType := range checkMap {
		if strings.TrimSpace(group) == "" {
			return errors.New("group name cannot be empty")
		}
		if billingType != GroupBillingTypeDefault &&
			billingType != GroupBillingTypeSubscriptionOnly &&
			billingType != GroupBillingTypeWalletOnly {
			return errors.New("invalid group billing type: " + group)
		}
	}
	return nil
}

func UpdateGroupBillingTypeByJSONString(jsonStr string) error {
	if strings.TrimSpace(jsonStr) == "" {
		groupBillingTypeMap.Clear()
		return nil
	}
	if err := CheckGroupBillingType(jsonStr); err != nil {
		return err
	}
	return types.LoadFromJsonString(groupBillingTypeMap, jsonStr)
}
