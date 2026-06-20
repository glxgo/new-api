package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

// 邀新计划(普通用户): 展示自己的邀请码/链接、直接+间接两层下级、下级产生的返利。
// 与分润两层(直接 AffiliateDirectRate / 间接 AffiliateIndirectRate)一致, 第 3 层+不产生返利故不展示。

// downlineUser 脱敏的下级用户视图(只暴露非隐私字段 + 为我产生的返利)。
type downlineUser struct {
	Id        int    `json:"id"`
	Username  string `json:"username"`
	CreatedAt int64  `json:"created_at"`
	Rebate    int64  `json:"rebate"` // 该下级为我产生的累计返利(quota)
}

// GetAffiliateSummary 当前用户的邀新概览: 邀请码/链接、直邀+间邀人数、累计返利、当前返利率。
func GetAffiliateSummary(c *gin.Context) {
	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil || user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}

	directIds, _ := model.GetDirectDownlineIds(userId)
	directCount := int64(len(directIds))
	indirectCount := int64(0)
	if len(directIds) > 0 {
		indirectCount, _ = model.CountDownline(directIds)
	}
	totalRebate, _ := model.SumDividendByRecipient(userId)

	affLink := ""
	serverAddress := system_setting.ServerAddress
	if user.AffCode != "" && serverAddress != "" {
		affLink = strings.TrimRight(serverAddress, "/") + "/register?aff=" + user.AffCode
	}

	common.ApiSuccess(c, gin.H{
		"aff_code":       user.AffCode,
		"aff_link":       affLink,
		"direct_count":   directCount,
		"indirect_count": indirectCount,
		"total_rebate":   totalRebate,
		"direct_rate":    common.AffiliateDirectRate,
		"indirect_rate":  common.AffiliateIndirectRate,
	})
}

// GetAffiliateDownline 下级用户列表。layer=1 直接(我邀请的), layer=2 间接(我下级邀请的)。
// 仅返回 id/username/created_at + 该下级为我产生的返利, 不泄露 email/quota/余额。
func GetAffiliateDownline(c *gin.Context) {
	userId := c.GetInt("id")
	layer, _ := strconv.Atoi(c.DefaultQuery("layer", "1"))
	if layer != 2 {
		layer = 1
	}
	page, pageSize := parseAffiliatePaging(c)

	// 决定查 inviter_id 的集合: 直接层=[我], 间接层=[我的直接下级 ids]
	var inviterIds []int
	if layer == 1 {
		inviterIds = []int{userId}
	} else {
		ids, _ := model.GetDirectDownlineIds(userId)
		if len(ids) == 0 {
			common.ApiSuccess(c, gin.H{"data": []downlineUser{}, "total": 0})
			return
		}
		inviterIds = ids
	}

	users, total, err := model.GetDownlineUsers(inviterIds, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	data := make([]downlineUser, 0, len(users))
	for _, u := range users {
		rebate, _ := model.SumDividendBySource(userId, u.Id)
		data = append(data, downlineUser{
			Id:        u.Id,
			Username:  u.Username,
			CreatedAt: u.CreatedAt,
			Rebate:    rebate,
		})
	}
	common.ApiSuccess(c, gin.H{"data": data, "total": total})
}

// GetAffiliateRebates 当前用户收到的返利明细(type=1,2 拉新返利)。
func GetAffiliateRebates(c *gin.Context) {
	userId := c.GetInt("id")
	page, pageSize := parseAffiliatePaging(c)
	records, total, err := model.GetDividendRecordsByRecipient(userId, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"data": records, "total": total})
}

// parseAffiliatePaging 解析分页参数(1-based page, 默认 page=1/pageSize=20)。
func parseAffiliatePaging(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}
