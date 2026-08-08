package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func AdminListTopUpCoupons(c *gin.Context) {
	coupons, err := model.ListTopUpCoupons()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, coupons)
}

func AdminSaveTopUpCoupon(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	coupon := &model.TopUpCoupon{Id: id, Enabled: true}
	if err := c.ShouldBindJSON(coupon); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	coupon.Id = id
	if id > 0 {
		var current model.TopUpCoupon
		if err := model.DB.First(&current, id).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		coupon.CreatedAt = current.CreatedAt
	}
	if err := model.SaveTopUpCoupon(coupon); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, coupon)
}

func AdminDeleteTopUpCoupon(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.DeleteTopUpCoupon(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func PreviewTopUpCoupon(c *gin.Context) {
	var req struct {
		Amount     int64  `json:"amount"`
		CouponCode string `json:"coupon_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Amount < getMinTopup() {
		common.ApiErrorMsg(c, "充值金额或优惠码无效")
		return
	}
	group, err := model.GetUserGroup(c.GetInt("id"), true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	quote, err := model.QuoteTopUpCoupon(c.GetInt("id"), req.CouponCode, getPayMoney(req.Amount, group))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if quote == nil {
		common.ApiErrorMsg(c, "请输入优惠码")
		return
	}
	common.ApiSuccess(c, quote)
}
