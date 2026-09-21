package controller

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func resolveSubscriptionPlanForPayment(userId, requestedPlanId, renewFromSubscriptionId int) (*model.SubscriptionPlan, error) {
	if renewFromSubscriptionId <= 0 {
		return model.GetSubscriptionPlanById(requestedPlanId)
	}
	_, plan, _, err := model.ResolveSubscriptionRenewalPlan(userId, renewFromSubscriptionId)
	if err != nil {
		return nil, err
	}
	if plan.Id != requestedPlanId {
		return nil, errors.New("续费套餐已变化，请刷新后重新确认")
	}
	return plan, nil
}

func configureSubscriptionOrderRenewal(order *model.SubscriptionOrder, userId, renewFromSubscriptionId int) error {
	if renewFromSubscriptionId <= 0 {
		return nil
	}
	return model.ConfigureSubscriptionOrderRenewal(order, userId, renewFromSubscriptionId)
}

type updateSubscriptionRemarkRequest struct {
	Remark string `json:"remark"`
}

type updateSubscriptionSpendLimitRequest struct {
	Period string `json:"period"`
	Quota  int64  `json:"quota"`
}

func UpdateSelfSubscriptionRemark(c *gin.Context) {
	userId := c.GetInt("id")
	subscriptionId, err := strconv.Atoi(c.Param("id"))
	if err != nil || subscriptionId <= 0 {
		common.ApiErrorMsg(c, "订阅实例编号无效")
		return
	}
	var req updateSubscriptionRemarkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	sub, err := model.UpdateUserSubscriptionRemark(userId, subscriptionId, req.Remark)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, sub)
}

// UpdateSelfSubscriptionSpendLimit updates an optional per-instance guardrail.
// Sending an empty period or zero quota disables the limit.
func UpdateSelfSubscriptionSpendLimit(c *gin.Context) {
	userId := c.GetInt("id")
	subscriptionId, err := strconv.Atoi(c.Param("id"))
	if err != nil || subscriptionId <= 0 {
		common.ApiErrorMsg(c, "订阅实例编号无效")
		return
	}
	var req updateSubscriptionSpendLimitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.Period != "" && req.Period != model.SubscriptionSpendLimitHour && req.Period != model.SubscriptionSpendLimitDay {
		common.ApiErrorMsg(c, "消费限额周期无效")
		return
	}
	if req.Period == "" {
		req.Quota = 0
	}
	if req.Period != "" && req.Quota <= 0 {
		common.ApiErrorMsg(c, "消费限额必须大于 0")
		return
	}
	sub, err := model.UpdateUserSubscriptionSpendLimit(userId, subscriptionId, req.Period, req.Quota)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, sub)
}

func GetSelfSubscriptionRenewalPreview(c *gin.Context) {
	userId := c.GetInt("id")
	subscriptionId, err := strconv.Atoi(c.Param("id"))
	if err != nil || subscriptionId <= 0 {
		common.ApiErrorMsg(c, "订阅实例编号无效")
		return
	}
	preview, err := model.GetSubscriptionRenewalPreview(userId, subscriptionId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, preview)
}

func ListSelfSubscriptionTokenBindings(c *gin.Context) {
	userId := c.GetInt("id")
	subscriptionId, err := strconv.Atoi(c.Param("id"))
	if err != nil || subscriptionId <= 0 {
		common.ApiErrorMsg(c, "订阅实例编号无效")
		return
	}
	items, err := model.ListUserTokensForSubscriptionBinding(userId, subscriptionId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func ReplaceSelfSubscriptionTokenBindings(c *gin.Context) {
	userId := c.GetInt("id")
	subscriptionId, err := strconv.Atoi(c.Param("id"))
	if err != nil || subscriptionId <= 0 {
		common.ApiErrorMsg(c, "订阅实例编号无效")
		return
	}
	var req model.BatchTokenSubscriptionBindingInput
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	req.SubscriptionId = subscriptionId
	if err := model.ReplaceSubscriptionTokenBindings(userId, req); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func GetSelfTokenSubscriptionHistory(c *gin.Context) {
	userId := c.GetInt("id")
	tokenId, err := strconv.Atoi(c.Param("id"))
	if err != nil || tokenId <= 0 {
		common.ApiErrorMsg(c, "API Key 编号无效")
		return
	}
	if _, err := model.GetTokenByIds(tokenId, userId); err != nil {
		common.ApiError(c, err)
		return
	}
	history, err := model.ListTokenSubscriptionBindingHistory(userId, tokenId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, history)
}
