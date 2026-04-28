package setting

import (
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

var userUsableGroups = map[string]string{
	"default": "默认分组",
	"vip":     "vip分组",
}
var userUsableGroupsMutex sync.RWMutex
var userVisibleGroups map[string]string
var userVisibleGroupsMutex sync.RWMutex

func GetUserUsableGroupsCopy() map[string]string {
	userUsableGroupsMutex.RLock()
	defer userUsableGroupsMutex.RUnlock()

	copyUserUsableGroups := make(map[string]string)
	for k, v := range userUsableGroups {
		copyUserUsableGroups[k] = v
	}
	return copyUserUsableGroups
}

func GetUserVisibleGroupsCopy() (map[string]string, bool) {
	userVisibleGroupsMutex.RLock()
	defer userVisibleGroupsMutex.RUnlock()

	if userVisibleGroups == nil {
		return nil, false
	}
	copyUserVisibleGroups := make(map[string]string, len(userVisibleGroups))
	for k, v := range userVisibleGroups {
		copyUserVisibleGroups[k] = v
	}
	return copyUserVisibleGroups, true
}

func UserUsableGroups2JSONString() string {
	userUsableGroupsMutex.RLock()
	defer userUsableGroupsMutex.RUnlock()

	jsonBytes, err := common.Marshal(userUsableGroups)
	if err != nil {
		common.SysLog("error marshalling user groups: " + err.Error())
	}
	return string(jsonBytes)
}

func UserVisibleGroups2JSONString() string {
	userVisibleGroupsMutex.RLock()
	defer userVisibleGroupsMutex.RUnlock()

	if userVisibleGroups == nil {
		return ""
	}
	jsonBytes, err := common.Marshal(userVisibleGroups)
	if err != nil {
		common.SysLog("error marshalling visible user groups: " + err.Error())
	}
	return string(jsonBytes)
}

func CheckUserGroupMap(jsonStr string) error {
	if strings.TrimSpace(jsonStr) == "" {
		return nil
	}
	checkMap := make(map[string]string)
	return common.UnmarshalJsonStr(jsonStr, &checkMap)
}

func UpdateUserUsableGroupsByJSONString(jsonStr string) error {
	userUsableGroupsMutex.Lock()
	defer userUsableGroupsMutex.Unlock()

	userUsableGroups = make(map[string]string)
	if strings.TrimSpace(jsonStr) == "" {
		return nil
	}
	return common.UnmarshalJsonStr(jsonStr, &userUsableGroups)
}

func UpdateUserVisibleGroupsByJSONString(jsonStr string) error {
	userVisibleGroupsMutex.Lock()
	defer userVisibleGroupsMutex.Unlock()

	if strings.TrimSpace(jsonStr) == "" {
		userVisibleGroups = nil
		return nil
	}
	userVisibleGroups = make(map[string]string)
	return common.UnmarshalJsonStr(jsonStr, &userVisibleGroups)
}

func GetUsableGroupDescription(groupName string) string {
	userUsableGroupsMutex.RLock()
	defer userUsableGroupsMutex.RUnlock()

	if desc, ok := userUsableGroups[groupName]; ok {
		return desc
	}
	return groupName
}
