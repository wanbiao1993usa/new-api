package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

func applyGroupVisibilityRules(groups map[string]string, rules map[string]string) {
	for specialGroup, desc := range rules {
		if strings.HasPrefix(specialGroup, "-:") {
			groupToRemove := strings.TrimPrefix(specialGroup, "-:")
			delete(groups, groupToRemove)
		} else if strings.HasPrefix(specialGroup, "+:") {
			groupToAdd := strings.TrimPrefix(specialGroup, "+:")
			groups[groupToAdd] = desc
		} else {
			groups[specialGroup] = desc
		}
	}
}

func ensureOwnGroup(groups map[string]string, userGroup string) {
	if userGroup == "" {
		return
	}
	if _, ok := groups[userGroup]; !ok {
		groups[userGroup] = "用户分组"
	}
}

func GetUserAccessibleGroups(userGroup string) map[string]string {
	groupsCopy := setting.GetUserUsableGroupsCopy()
	if userGroup != "" {
		specialSettings, ok := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(userGroup)
		if ok {
			applyGroupVisibilityRules(groupsCopy, specialSettings)
		}
		ensureOwnGroup(groupsCopy, userGroup)
	}
	return groupsCopy
}

func GetUserVisibleGroups(userGroup string) map[string]string {
	groupsCopy, configured := setting.GetUserVisibleGroupsCopy()
	if !configured {
		groupsCopy = GetUserAccessibleGroups(userGroup)
	} else if userGroup != "" {
		ensureOwnGroup(groupsCopy, userGroup)
	}
	if userGroup != "" {
		specialSettings, ok := ratio_setting.GetGroupRatioSetting().GroupSpecialVisibleGroup.Get(userGroup)
		if ok {
			applyGroupVisibilityRules(groupsCopy, specialSettings)
		}
		ensureOwnGroup(groupsCopy, userGroup)
	}
	return groupsCopy
}

func GetUserUsableGroups(userGroup string) map[string]string {
	return GetUserAccessibleGroups(userGroup)
}

func GroupInUserAccessibleGroups(userGroup, groupName string) bool {
	_, ok := GetUserAccessibleGroups(userGroup)[groupName]
	return ok
}

func GroupInUserVisibleGroups(userGroup, groupName string) bool {
	_, ok := GetUserVisibleGroups(userGroup)[groupName]
	return ok
}

func GroupInUserUsableGroups(userGroup, groupName string) bool {
	_, ok := GetUserAccessibleGroups(userGroup)[groupName]
	return ok
}

// GetUserAutoGroup 根据用户分组获取自动分组设置
func GetUserAutoGroup(userGroup string) []string {
	groups := GetUserAccessibleGroups(userGroup)
	autoGroups := make([]string, 0)
	for _, group := range setting.GetAutoGroups() {
		if _, ok := groups[group]; ok {
			autoGroups = append(autoGroups, group)
		}
	}
	return autoGroups
}

// GetUserGroupRatio 获取用户使用某个分组的倍率
// userGroup 用户分组
// group 需要获取倍率的分组
func GetUserGroupRatio(userGroup, group string) float64 {
	ratio, ok := ratio_setting.GetGroupGroupRatio(userGroup, group)
	if ok {
		return ratio
	}
	return ratio_setting.GetGroupRatio(group)
}

func ResolveBillingGroup(c *gin.Context, relayInfo *relaycommon.RelayInfo) string {
	if c != nil {
		if autoGroup := common.GetContextKeyString(c, constant.ContextKeyAutoGroup); autoGroup != "" {
			return autoGroup
		}
		if autoGroup, exists := c.Get("auto_group"); exists {
			if group, ok := autoGroup.(string); ok && group != "" {
				return group
			}
		}
	}
	if relayInfo != nil && relayInfo.UsingGroup != "" {
		return relayInfo.UsingGroup
	}
	return ""
}

func ResolveGroupBillingType(c *gin.Context, relayInfo *relaycommon.RelayInfo) (string, string) {
	group := ResolveBillingGroup(c, relayInfo)
	return group, ratio_setting.GetGroupBillingType(group)
}
