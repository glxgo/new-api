package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetGroups(c *gin.Context) {
	groupNames := make([]string, 0)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		groupNames = append(groupNames, groupName)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}

func GetUserGroups(c *gin.Context) {
	usableGroups := make(map[string]map[string]interface{})
	userGroup := ""
	userId := c.GetInt("id")
	userGroup, _ = model.GetUserGroup(userId, false)
	userUsableGroups := service.GetUserUsableGroups(userGroup)
	for groupName, _ := range ratio_setting.GetGroupRatioCopy() {
		// UserUsableGroups contains the groups that the user can use
		if desc, ok := userUsableGroups[groupName]; ok {
			usableGroups[groupName] = map[string]interface{}{
				"ratio": service.GetUserGroupRatio(userGroup, groupName),
				"desc":  desc,
			}
		}
	}
	if _, ok := userUsableGroups["auto"]; ok {
		usableGroups["auto"] = map[string]interface{}{
			"ratio": "自动",
			"desc":  setting.GetUsableGroupDescription("auto"),
		}
	}
	// 订阅即凭证: 并入用户有效订阅的 AllowedGroup(月卡/周卡绑定的分组), 让 token 分组下拉能选到
	if subGroups, err := model.GetActiveUserSubscriptionAllowedGroups(userId); err == nil {
		for _, g := range subGroups {
			if _, exists := usableGroups[g]; !exists {
				usableGroups[g] = map[string]interface{}{
					"ratio": service.GetUserGroupRatio(userGroup, g),
					"desc":  setting.GetUsableGroupDescription(g),
				}
			}
		}
	}
	// 虚拟会员即凭证: 会员专属分组也要出现在 API Key 下拉中，
	// 否则用户只能绑定会员实例而无法选择对应的线路分组。
	if membershipGroups, err := model.GetActiveUserVirtualMembershipAllowedGroups(userId); err == nil {
		for _, g := range membershipGroups {
			if _, exists := usableGroups[g]; !exists {
				usableGroups[g] = map[string]interface{}{
					"ratio": service.GetUserGroupRatio(userGroup, g),
					"desc":  "虚拟会员专属分组",
				}
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    usableGroups,
	})
}
