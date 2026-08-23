package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

type virtualMembershipPlanRequest struct {
	Code                    string  `json:"code"`
	Title                   string  `json:"title"`
	Subtitle                string  `json:"subtitle"`
	Description             string  `json:"description"`
	OriginalPriceAmount     float64 `json:"original_price_amount"`
	PriceAmount             float64 `json:"price_amount"`
	TwoGroupOriginalPrice   float64 `json:"two_group_original_price"`
	TwoGroupPrice           float64 `json:"two_group_price"`
	ThreeGroupOriginalPrice float64 `json:"three_group_original_price"`
	ThreeGroupPrice         float64 `json:"three_group_price"`
	FourGroupOriginalPrice  float64 `json:"four_group_original_price"`
	FourGroupPrice          float64 `json:"four_group_price"`
	Currency                string  `json:"currency"`
	DurationDays            int     `json:"duration_days"`
	WeeklyQuota             int64   `json:"weekly_quota"`
	FiveHourEnabled         bool    `json:"five_hour_enabled"`
	FiveHourQuota           int64   `json:"five_hour_quota"`
	ConcurrencyLimit        int     `json:"concurrency_limit"`
	RPMLimit                int     `json:"rpm_limit"`
	AllowedModels           string  `json:"allowed_models"`
	AllowedGroup            string  `json:"allowed_group"`
	Recommended             bool    `json:"recommended"`
	Enabled                 *bool   `json:"enabled"`
	SortOrder               int     `json:"sort_order"`
}

func virtualMembershipPlanResponse(plan *model.VirtualMembershipPlan) gin.H {
	if plan == nil {
		return gin.H{}
	}
	variant := func(groupSize int) gin.H {
		price, weekly, fiveHour, concurrency, rpm, err := model.VirtualMembershipVariantLimitsForDisplay(plan, groupSize)
		if err != nil {
			price, weekly, fiveHour, concurrency, rpm = 0, 0, 0, 0, 0
		}
		label := "单独购买"
		if groupSize > 1 {
			label = strconv.Itoa(groupSize) + " 人团"
		}
		originalPrice, originalPriceErr := model.VirtualMembershipOriginalPriceForDisplay(plan, groupSize)
		if originalPriceErr != nil {
			originalPrice = 0
		}
		return gin.H{"group_size": groupSize, "label": label, "original_price_amount": originalPrice, "price_amount": price, "weekly_quota": weekly, "five_hour_quota": fiveHour, "concurrency_limit": concurrency, "rpm_limit": rpm}
	}
	return gin.H{
		"id": plan.Id, "code": plan.Code, "title": plan.Title, "subtitle": plan.Subtitle,
		"description": plan.Description, "original_price_amount": plan.OriginalPriceAmount, "price_amount": plan.PriceAmount,
		"two_group_original_price": plan.TwoGroupOriginalPrice, "two_group_price": plan.TwoGroupPrice,
		"three_group_original_price": plan.ThreeGroupOriginalPrice, "three_group_price": plan.ThreeGroupPrice,
		"four_group_original_price": plan.FourGroupOriginalPrice, "four_group_price": plan.FourGroupPrice, "currency": plan.Currency,
		"duration_days": plan.DurationDays, "weekly_quota": plan.WeeklyQuota,
		"five_hour_enabled": plan.FiveHourEnabled, "five_hour_quota": plan.FiveHourQuota,
		"concurrency_limit": plan.ConcurrencyLimit, "rpm_limit": plan.RPMLimit,
		"allowed_models": plan.AllowedModels, "allowed_group": plan.AllowedGroup,
		"recommended": plan.Recommended, "enabled": plan.Enabled, "sort_order": plan.SortOrder,
		"variants": []gin.H{variant(1), variant(2), variant(3), variant(4)},
	}
}

func virtualMembershipInstanceResponse(membership *model.UserVirtualMembership) gin.H {
	if membership == nil {
		return gin.H{}
	}
	resetPrice := membership.ActiveResetPriceAmount
	if resetPrice <= 0 && membership.PurchasePriceAmount > 0 {
		resetPrice = model.VirtualMembershipActiveResetPrice(membership.PurchasePriceAmount)
	}
	return gin.H{
		"id": membership.Id, "plan_id": membership.PlanId, "order_id": membership.OrderId,
		"hidden": membership.Hidden, "renewed_from_id": membership.RenewedFromId,
		"plan_title": membership.PlanTitle, "plan_code": membership.PlanCode, "group_size": membership.GroupSize,
		"purchase_price_amount":     membership.PurchasePriceAmount,
		"active_reset_credits":      membership.ActiveResetCredits,
		"active_reset_price_amount": resetPrice,
		"weekly_quota":              membership.WeeklyQuota, "weekly_used": membership.WeeklyUsed,
		"weekly_remaining":  maxInt64(membership.WeeklyQuota - membership.WeeklyUsed),
		"weekly_percent":    model.VirtualMembershipQuotaPercent(membership.WeeklyUsed, membership.WeeklyQuota),
		"five_hour_enabled": membership.FiveHourActive, "five_hour_quota": membership.FiveHourQuota,
		"five_hour_used": membership.FiveHourUsed, "five_hour_remaining": maxInt64(membership.FiveHourQuota - membership.FiveHourUsed),
		"five_hour_percent": model.VirtualMembershipQuotaPercent(membership.FiveHourUsed, membership.FiveHourQuota),
		"lifetime_used":     membership.LifetimeUsed,
		"concurrency_limit": membership.ConcurrencyLimit, "rpm_limit": membership.RPMLimit,
		"weekly_reset_at": membership.WeeklyResetAt, "five_hour_reset_at": membership.FiveHourResetAt,
		"start_time": membership.StartTime, "end_time": membership.EndTime, "status": membership.Status,
		"allowed_models": membership.AllowedModels, "allowed_group": membership.AllowedGroup,
	}
}

func maxInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func virtualMembershipEpayMethods() []map[string]string {
	if !isEpayTopUpEnabled() {
		return []map[string]string{}
	}
	methods := make([]map[string]string, 0, len(operation_setting.PayMethods))
	for _, method := range operation_setting.PayMethods {
		if method["type"] == "" || method["type"] == model.PaymentMethodStripe || method["type"] == model.PaymentMethodCreem {
			continue
		}
		methods = append(methods, method)
	}
	return methods
}

func GetVirtualMembershipPage(c *gin.Context) {
	setting, err := model.GetVirtualMembershipSetting()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !setting.Enabled {
		common.ApiSuccess(c, gin.H{"announcement": setting.Announcement, "enabled": false, "plans": []gin.H{}, "memberships": []gin.H{}, "epay_enabled": false, "epay_methods": []map[string]string{}})
		return
	}
	plans, err := model.ListVirtualMembershipPlans(false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	memberships, err := model.ListUserVirtualMemberships(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	hiddenMemberships, err := model.ListHiddenUserVirtualMemberships(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	planItems := make([]gin.H, 0, len(plans))
	for _, plan := range plans {
		planItems = append(planItems, virtualMembershipPlanResponse(plan))
	}
	instanceItems := make([]gin.H, 0, len(memberships))
	for _, membership := range memberships {
		instanceItems = append(instanceItems, virtualMembershipInstanceResponse(membership))
	}
	hiddenInstanceItems := make([]gin.H, 0, len(hiddenMemberships))
	for _, membership := range hiddenMemberships {
		hiddenInstanceItems = append(hiddenInstanceItems, virtualMembershipInstanceResponse(membership))
	}
	common.ApiSuccess(c, gin.H{
		"announcement": setting.Announcement, "enabled": setting.Enabled,
		"plans": planItems, "memberships": instanceItems, "hidden_memberships": hiddenInstanceItems,
		"epay_enabled": len(virtualMembershipEpayMethods()) > 0,
		"epay_methods": virtualMembershipEpayMethods(),
	})
}

// SetSelfVirtualMembershipVisibility changes only the owner's remaining-quota
// card visibility; it never revokes or pauses the membership.
func SetSelfVirtualMembershipVisibility(c *gin.Context) {
	membershipId, _ := strconv.Atoi(c.Param("id"))
	if membershipId <= 0 {
		common.ApiErrorMsg(c, "无效的虚拟会员ID")
		return
	}
	var req struct {
		Hidden *bool `json:"hidden"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Hidden == nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := model.SetVirtualMembershipHiddenForUser(membershipId, c.GetInt("id"), *req.Hidden); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"hidden": *req.Hidden})
}

// ActiveResetVirtualMembership lets a user consume one administrator-granted
// reset credit. When no credit remains the caller receives the stable error
// message and can start the Epay add-on flow from the same membership card.
func ActiveResetVirtualMembership(c *gin.Context) {
	membershipId, _ := strconv.Atoi(c.Param("id"))
	membership, err := model.ActiveResetVirtualMembership(c.GetInt("id"), membershipId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"membership": virtualMembershipInstanceResponse(membership)})
}

func PurchaseVirtualMembership(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	var req struct {
		PlanId                int `json:"plan_id" binding:"required"`
		GroupSize             int `json:"group_size"`
		RenewFromMembershipId int `json:"renew_from_membership_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.GroupSize == 0 {
		req.GroupSize = 1
	}
	order, membership, err := model.PurchaseVirtualMembershipWithBalanceRenewal(c.GetInt("id"), req.PlanId, req.GroupSize, req.RenewFromMembershipId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"order": order, "membership": virtualMembershipInstanceResponse(membership)})
}

func ListVirtualMembershipTokens(c *gin.Context) {
	membershipId, _ := strconv.Atoi(c.Param("id"))
	tokens, err := model.ListUserVirtualMembershipTokens(c.GetInt("id"), membershipId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"data": tokens})
}

func ReplaceVirtualMembershipTokens(c *gin.Context) {
	membershipId, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		TokenIds []int `json:"token_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := model.ReplaceUserVirtualMembershipTokens(c.GetInt("id"), membershipId, req.TokenIds); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminListVirtualMembershipPlans(c *gin.Context) {
	plans, err := model.ListVirtualMembershipPlans(true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]gin.H, 0, len(plans))
	for _, plan := range plans {
		items = append(items, virtualMembershipPlanResponse(plan))
	}
	common.ApiSuccess(c, items)
}

func AdminSaveVirtualMembershipPlan(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	plan := &model.VirtualMembershipPlan{Id: id, Enabled: true}
	var req virtualMembershipPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	plan.Code, plan.Title, plan.Subtitle, plan.Description = req.Code, req.Title, req.Subtitle, req.Description
	plan.OriginalPriceAmount, plan.PriceAmount = req.OriginalPriceAmount, req.PriceAmount
	plan.TwoGroupOriginalPrice, plan.TwoGroupPrice = req.TwoGroupOriginalPrice, req.TwoGroupPrice
	plan.ThreeGroupOriginalPrice, plan.ThreeGroupPrice = req.ThreeGroupOriginalPrice, req.ThreeGroupPrice
	plan.FourGroupOriginalPrice, plan.FourGroupPrice = req.FourGroupOriginalPrice, req.FourGroupPrice
	plan.Currency, plan.DurationDays, plan.WeeklyQuota = req.Currency, req.DurationDays, req.WeeklyQuota
	plan.FiveHourEnabled, plan.FiveHourQuota = req.FiveHourEnabled, req.FiveHourQuota
	plan.ConcurrencyLimit, plan.RPMLimit = req.ConcurrencyLimit, req.RPMLimit
	plan.AllowedModels, plan.AllowedGroup, plan.Recommended, plan.SortOrder = req.AllowedModels, req.AllowedGroup, req.Recommended, req.SortOrder
	if req.Enabled != nil {
		plan.Enabled = *req.Enabled
	}
	if id > 0 {
		var current model.VirtualMembershipPlan
		if err := model.DB.First(&current, id).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		plan.CreatedAt = current.CreatedAt
	}
	if err := model.SaveVirtualMembershipPlan(plan); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, virtualMembershipPlanResponse(plan))
}

func AdminGetVirtualMembershipSetting(c *gin.Context) {
	setting, err := model.GetVirtualMembershipSetting()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, setting)
}

func AdminSaveVirtualMembershipSetting(c *gin.Context) {
	var req struct {
		Announcement string `json:"announcement"`
		Enabled      *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	setting, err := model.GetVirtualMembershipSetting()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	setting.Announcement = req.Announcement
	if req.Enabled != nil {
		setting.Enabled = *req.Enabled
	}
	if err := model.SaveVirtualMembershipSetting(setting); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, setting)
}

func AdminResetVirtualMemberships(c *gin.Context) {
	var req struct {
		MembershipId int    `json:"membership_id"`
		UserId       int    `json:"user_id"`
		PlanCode     string `json:"plan_code"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			common.ApiErrorMsg(c, "参数错误")
			return
		}
	}
	affected, err := model.ResetVirtualMemberships(model.VirtualMembershipResetScope{
		MembershipId: req.MembershipId, UserId: req.UserId, PlanCode: req.PlanCode,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"affected": affected, "next_reset_at": common.GetTimestamp() + 7*86400})
}

func AdminListVirtualMemberships(c *gin.Context) {
	records, err := model.ListAdminVirtualMemberships()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]gin.H, 0, len(records))
	for _, record := range records {
		if record == nil || record.Membership == nil {
			continue
		}
		item := virtualMembershipInstanceResponse(record.Membership)
		item["user_id"] = record.Membership.UserId
		item["username"] = record.Username
		item["display_name"] = record.DisplayName
		item["email"] = record.Email
		item["user_deleted"] = record.UserDeleted
		items = append(items, item)
	}
	common.ApiSuccess(c, items)
}

func AdminGrantVirtualMembership(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	var req struct {
		UserId    int `json:"user_id"`
		PlanId    int `json:"plan_id"`
		GroupSize int `json:"group_size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.UserId <= 0 || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.GroupSize == 0 {
		req.GroupSize = 1
	}
	order, membership, err := model.AdminGrantVirtualMembership(req.UserId, req.PlanId, req.GroupSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	item := virtualMembershipInstanceResponse(membership)
	item["user_id"] = req.UserId
	common.ApiSuccess(c, gin.H{"order_id": order.Id, "membership": item})
}

func AdminGrantVirtualMembershipResetCredits(c *gin.Context) {
	membershipId, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Credits int `json:"credits"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Credits <= 0 {
		common.ApiErrorMsg(c, "主动重置次数必须大于 0")
		return
	}
	membership, err := model.GrantVirtualMembershipResetCredits(membershipId, req.Credits)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"membership": virtualMembershipInstanceResponse(membership), "credits": membership.ActiveResetCredits})
}

func AdminDeleteVirtualMembership(c *gin.Context) {
	membershipId, _ := strconv.Atoi(c.Param("id"))
	if membershipId <= 0 {
		common.ApiErrorMsg(c, "无效的虚拟会员ID")
		return
	}
	unboundTokens, err := model.AdminDeleteVirtualMembership(membershipId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"unbound_tokens": unboundTokens})
}

func AdminRenewVirtualMembership(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	membershipId, _ := strconv.Atoi(c.Param("id"))
	if membershipId <= 0 {
		common.ApiErrorMsg(c, "无效的虚拟会员ID")
		return
	}
	order, membership, err := model.AdminRenewVirtualMembership(membershipId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"order_id": order.Id, "membership": virtualMembershipInstanceResponse(membership)})
}

func AdminSetVirtualMembershipVisibility(c *gin.Context) {
	membershipId, _ := strconv.Atoi(c.Param("id"))
	if membershipId <= 0 {
		common.ApiErrorMsg(c, "无效的虚拟会员ID")
		return
	}
	var req struct {
		Hidden *bool `json:"hidden"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Hidden == nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := model.SetVirtualMembershipHidden(membershipId, *req.Hidden); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"hidden": *req.Hidden})
}

func AdminListVirtualMembershipOrders(c *gin.Context) {
	userId, _ := strconv.Atoi(c.Query("user_id"))
	orders, err := model.ListVirtualMembershipOrders(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"data": orders})
}
