package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"net/http"
)

func ListClientIdentities(c *gin.Context) {
	page := common.GetPageQuery(c)
	status := c.DefaultQuery("status", "pending")
	if status != "pending" && status != "approved" && status != "rejected" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	var items []model.ClientIdentity
	var total int64
	tx := model.DB.WithContext(c.Request.Context()).Model(&model.ClientIdentity{}).Where("status = ?", status)
	if err := tx.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if err := tx.Order("last_seen desc, client_key asc").Limit(page.GetPageSize()).Offset(page.GetStartIdx()).Find(&items).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	page.SetTotal(int(total))
	page.SetItems(items)
	common.ApiSuccess(c, page)
}
func ReviewClientIdentity(c *gin.Context) {
	var input struct {
		ClientKey string `json:"client_key"`
		Status    string `json:"status"`
		Reason    string `json:"reason"`
		Revision  int64  `json:"revision"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.ReviewClient(c.Request.Context(), input.ClientKey, input.Status, input.Reason, c.GetInt("id"), input.Revision); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
func GetClientReviews(c *gin.Context) {
	page := common.GetPageQuery(c)
	var rows []model.ClientReview
	var total int64
	tx := model.DB.WithContext(c.Request.Context()).Model(&model.ClientReview{}).Where("client_key = ?", c.Query("client_key"))
	if err := tx.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if err := tx.Order("id desc").Offset(page.GetStartIdx()).Limit(page.GetPageSize()).Find(&rows).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	page.SetTotal(int(total))
	page.SetItems(rows)
	common.ApiSuccess(c, page)
}
func GetClientGroupPolicies(c *gin.Context) {
	var rows []model.ClientGroupPolicy
	if err := model.DB.WithContext(c.Request.Context()).Order("group_name asc").Find(&rows).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}
func UpdateClientGroupPolicy(c *gin.Context) {
	var input struct {
		model.ClientGroupPolicy
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.SaveClientGroupPolicy(c.Request.Context(), input.ClientGroupPolicy, input.Reason, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
func GetClientGroupPolicyReviews(c *gin.Context) {
	page := common.GetPageQuery(c)
	var rows []model.ClientGroupPolicyReview
	var total int64
	tx := model.DB.WithContext(c.Request.Context()).Model(&model.ClientGroupPolicyReview{}).Where("group_name = ?", c.Query("group_name"))
	if err := tx.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if err := tx.Order("id desc").Offset(page.GetStartIdx()).Limit(page.GetPageSize()).Find(&rows).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	page.SetTotal(int(total))
	page.SetItems(rows)
	common.ApiSuccess(c, page)
}
