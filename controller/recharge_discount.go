package controller

import (
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"net/http"
)

func applyUserRechargeDiscount(c *gin.Context, money float64) (float64, bool) {
	discount, err := model.GetUserRechargeDiscount(c.GetInt("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取累计充值折扣失败"})
		return 0, false
	}
	return model.ApplyRechargeDiscount(money, discount.TotalCents), true
}
