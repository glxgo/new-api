package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type VirtualMembershipEpayPayRequest struct {
	PlanId        int    `json:"plan_id"`
	GroupSize     int    `json:"group_size"`
	PaymentMethod string `json:"payment_method"`
}

func VirtualMembershipRequestEpay(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	var req VirtualMembershipEpayPayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.GroupSize == 0 {
		req.GroupSize = 1
	}
	if !operation_setting.ContainsPayMethod(req.PaymentMethod) {
		common.ApiErrorMsg(c, "支付方式不存在")
		return
	}
	plan, err := model.GetVirtualMembershipPlanById(req.PlanId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !plan.Enabled {
		common.ApiErrorMsg(c, "虚拟会员方案未启用")
		return
	}
	price, _, _, err := model.VirtualMembershipVariantForDisplay(plan, req.GroupSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if price < 0.01 {
		common.ApiErrorMsg(c, "虚拟会员金额过低")
		return
	}

	callbackAddress := service.GetCallbackAddress()
	returnURL, err := url.Parse(paymentReturnPath("/console/virtual-membership?pay=success"))
	if err != nil {
		common.ApiErrorMsg(c, "回调地址配置错误")
		return
	}
	notifyURL, err := url.Parse(callbackAddress + "/api/virtual-membership/epay/notify")
	if err != nil {
		common.ApiErrorMsg(c, "回调地址配置错误")
		return
	}
	tradeNo := fmt.Sprintf("VMUSR%dNO%s", c.GetInt("id"), fmt.Sprintf("%s%d", common.GetRandomString(6), time.Now().Unix()))
	client := GetEpayClient()
	if client == nil {
		common.ApiErrorMsg(c, "当前管理员未配置支付信息")
		return
	}
	order, err := model.CreateVirtualMembershipEpayOrder(c.GetInt("id"), req.PlanId, req.GroupSize, tradeNo, req.PaymentMethod)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	uri, params, err := client.Purchase(&epay.PurchaseArgs{
		Type: req.PaymentMethod, ServiceTradeNo: tradeNo,
		Name: fmt.Sprintf("VM:%s", plan.Title), Money: fmt.Sprintf("%.2f", order.Money),
		Device: epay.PC, NotifyUrl: notifyURL, ReturnUrl: returnURL,
	})
	if err != nil {
		_ = model.ExpireVirtualMembershipOrder(tradeNo, model.PaymentProviderEpay)
		common.ApiErrorMsg(c, "拉起支付失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": params, "url": uri})
}

func readEpayCallbackParams(c *gin.Context) map[string]string {
	if c.Request.Method == http.MethodPost {
		if err := c.Request.ParseForm(); err != nil {
			return nil
		}
		return lo.Reduce(lo.Keys(c.Request.PostForm), func(result map[string]string, key string, _ int) map[string]string {
			result[key] = c.Request.PostForm.Get(key)
			return result
		}, map[string]string{})
	}
	return lo.Reduce(lo.Keys(c.Request.URL.Query()), func(result map[string]string, key string, _ int) map[string]string {
		result[key] = c.Request.URL.Query().Get(key)
		return result
	}, map[string]string{})
}

func VirtualMembershipEpayNotify(c *gin.Context) {
	params := readEpayCallbackParams(c)
	if len(params) == 0 {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	client := GetEpayClient()
	if client == nil {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	verifyInfo, err := client.Verify(params)
	if err != nil || !verifyInfo.VerifyStatus || verifyInfo.TradeStatus != epay.StatusTradeSuccess {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	LockOrder(verifyInfo.ServiceTradeNo)
	defer UnlockOrder(verifyInfo.ServiceTradeNo)
	actual, snapshotErr := model.NewPaymentSnapshotFromDisplayAmount(verifyInfo.Money, "CNY")
	if snapshotErr != nil {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	if err := model.CompleteVirtualMembershipOrder(verifyInfo.ServiceTradeNo, common.GetJsonString(verifyInfo), model.PaymentProviderEpay, verifyInfo.Type, actual); err != nil {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	_, _ = c.Writer.Write([]byte("success"))
}

func VirtualMembershipEpayReturn(c *gin.Context) {
	params := readEpayCallbackParams(c)
	failURL := paymentReturnPath("/console/virtual-membership?pay=fail")
	pendingURL := paymentReturnPath("/console/virtual-membership?pay=pending")
	successURL := paymentReturnPath("/console/virtual-membership?pay=success")
	if len(params) == 0 {
		c.Redirect(http.StatusFound, failURL)
		return
	}
	client := GetEpayClient()
	if client == nil {
		c.Redirect(http.StatusFound, failURL)
		return
	}
	verifyInfo, err := client.Verify(params)
	if err != nil || !verifyInfo.VerifyStatus {
		c.Redirect(http.StatusFound, failURL)
		return
	}
	if verifyInfo.TradeStatus != epay.StatusTradeSuccess {
		c.Redirect(http.StatusFound, pendingURL)
		return
	}
	LockOrder(verifyInfo.ServiceTradeNo)
	defer UnlockOrder(verifyInfo.ServiceTradeNo)
	actual, snapshotErr := model.NewPaymentSnapshotFromDisplayAmount(verifyInfo.Money, "CNY")
	if snapshotErr != nil {
		c.Redirect(http.StatusFound, paymentReturnPath("/console/virtual-membership?pay=fail"))
		return
	}
	if err := model.CompleteVirtualMembershipOrder(verifyInfo.ServiceTradeNo, common.GetJsonString(verifyInfo), model.PaymentProviderEpay, verifyInfo.Type, actual); err != nil {
		c.Redirect(http.StatusFound, failURL)
		return
	}
	c.Redirect(http.StatusFound, successURL)
}
