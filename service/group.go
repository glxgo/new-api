package service

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func GetUserUsableGroups(userGroup string) map[string]string {
	groupsCopy := setting.GetUserUsableGroupsCopy()
	if userGroup != "" {
		specialSettings, b := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(userGroup)
		if b {
			// 处理特殊可用分组
			for specialGroup, desc := range specialSettings {
				if strings.HasPrefix(specialGroup, "-:") {
					// 移除分组
					groupToRemove := strings.TrimPrefix(specialGroup, "-:")
					delete(groupsCopy, groupToRemove)
				} else if strings.HasPrefix(specialGroup, "+:") {
					// 添加分组
					groupToAdd := strings.TrimPrefix(specialGroup, "+:")
					groupsCopy[groupToAdd] = desc
				} else {
					// 直接添加分组
					groupsCopy[specialGroup] = desc
				}
			}
		}
		// 如果userGroup不在UserUsableGroups中，返回UserUsableGroups + userGroup
		if _, ok := groupsCopy[userGroup]; !ok {
			groupsCopy[userGroup] = "用户分组"
		}
	}
	return groupsCopy
}

// GetOrderedUserGroups applies the administrator's display order and appends
// newly introduced groups deterministically. Subscription-only groups are
// appended by the controller's existing response logic. This is
// presentation metadata; the returned set is still controlled by groups.
func GetOrderedUserGroups(groups map[string]string) []string {
	configured := setting.GetGroupOrderCopy()
	ordered := make([]string, 0, len(groups))
	seen := make(map[string]struct{}, len(groups))
	for _, group := range configured {
		if _, ok := groups[group]; ok {
			ordered = append(ordered, group)
			seen[group] = struct{}{}
		}
	}
	remaining := make([]string, 0, len(groups))
	for group := range groups {
		if _, ok := seen[group]; !ok {
			remaining = append(remaining, group)
		}
	}
	sort.Strings(remaining)
	return append(ordered, remaining...)
}

func GroupInUserUsableGroups(userGroup, groupName string) bool {
	_, ok := GetUserUsableGroups(userGroup)[groupName]
	return ok
}

// GetUserAutoGroup 根据用户分组获取自动分组设置
func GetUserAutoGroup(userGroup string) []string {
	groups := GetUserUsableGroups(userGroup)
	autoGroups := make([]string, 0)
	for _, group := range setting.GetAutoGroups() {
		if _, ok := groups[group]; ok {
			autoGroups = append(autoGroups, group)
		}
	}
	return autoGroups
}

// GetUserGroupRatio 获取用户使用某个分组的倍率。
// 废弃 GroupGroupRatio 二级倍率(2026-06-22): 统一返回 GroupRatio(唯一分组售价倍率),
// 与实际计费一致。保留 userGroup 参数以兼容现有调用方(controller/group.go 展示用)。
func GetUserGroupRatio(userGroup, group string) float64 {
	_ = userGroup
	return ratio_setting.GetGroupRatio(group)
}
