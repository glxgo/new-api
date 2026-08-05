package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type apiIngressProfileRequest struct {
	Code                 string `json:"code"`
	DisplayName          string `json:"display_name"`
	PublicBaseURL        string `json:"public_base_url"`
	NetworkMode          string `json:"network_mode"`
	Description          string `json:"description"`
	BillingMultiplierPPM int64  `json:"billing_multiplier_ppm"`
	Enabled              *bool  `json:"enabled"`
	Visible              *bool  `json:"visible"`
	Default              *bool  `json:"default"`
	ProbeEnabled         *bool  `json:"probe_enabled"`
	SortOrder            int    `json:"sort_order"`
}

func apiIngressProfileJSON(profile *model.APIIngressProfile) gin.H {
	if profile == nil {
		return gin.H{}
	}
	return gin.H{
		"id": profile.Id, "code": profile.Code, "display_name": profile.DisplayName,
		"public_base_url": profile.PublicBaseURL, "network_mode": profile.NetworkMode,
		"description": profile.Description, "billing_multiplier_ppm": profile.BillingMultiplierPPM,
		"multiplier": float64(profile.BillingMultiplierPPM) / float64(model.APIIngressPPMOne),
		"enabled":    profile.Enabled, "visible": profile.Visible, "default": profile.Default,
		"probe_enabled": profile.ProbeEnabled, "sort_order": profile.SortOrder,
	}
}

func GetAPIIngressProfiles(c *gin.Context) {
	profiles, err := model.ListAPIIngressProfiles(false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]gin.H, 0, len(profiles))
	for _, profile := range profiles {
		items = append(items, apiIngressProfileJSON(profile))
	}
	common.ApiSuccess(c, items)
}

func APIIngressPing(c *gin.Context) {
	common.ApiSuccess(c, gin.H{
		"data": gin.H{
			"ingress":      c.GetString("api_ingress_code"),
			"display_name": c.GetString("api_ingress_display_name"),
			"server_time":  common.GetTimestamp(),
		},
	})
}

func AdminListAPIIngressProfiles(c *gin.Context) {
	profiles, err := model.ListAPIIngressProfiles(true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]gin.H, 0, len(profiles))
	for _, profile := range profiles {
		items = append(items, apiIngressProfileJSON(profile))
	}
	common.ApiSuccess(c, items)
}

func mergeAPIIngressProfile(profile *model.APIIngressProfile, req apiIngressProfileRequest) {
	if req.Code != "" {
		profile.Code = req.Code
	}
	profile.DisplayName = req.DisplayName
	profile.PublicBaseURL = req.PublicBaseURL
	profile.NetworkMode = req.NetworkMode
	profile.Description = req.Description
	profile.BillingMultiplierPPM = req.BillingMultiplierPPM
	profile.SortOrder = req.SortOrder
	if req.Enabled != nil {
		profile.Enabled = *req.Enabled
	}
	if req.Visible != nil {
		profile.Visible = *req.Visible
	}
	if req.Default != nil {
		profile.Default = *req.Default
	}
	if req.ProbeEnabled != nil {
		profile.ProbeEnabled = *req.ProbeEnabled
	}
}

func AdminCreateAPIIngressProfile(c *gin.Context) {
	var req apiIngressProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	profile := &model.APIIngressProfile{Enabled: true, Visible: true, ProbeEnabled: true}
	mergeAPIIngressProfile(profile, req)
	if err := model.SaveAPIIngressProfile(profile); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, apiIngressProfileJSON(profile))
}

func AdminUpdateAPIIngressProfile(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "入口 ID 无效")
		return
	}
	var profile model.APIIngressProfile
	if err = model.DB.First(&profile, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var req apiIngressProfileRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	mergeAPIIngressProfile(&profile, req)
	if err = model.SaveAPIIngressProfile(&profile); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, apiIngressProfileJSON(&profile))
}

func AdminDeleteAPIIngressProfile(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "入口 ID 无效")
		return
	}
	var profile model.APIIngressProfile
	if err = model.DB.First(&profile, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if profile.Default {
		common.ApiErrorMsg(c, "默认入口不能删除，请先切换默认入口")
		return
	}
	if strings.TrimSpace(profile.Code) == model.APIIngressCodeOptimized {
		common.ApiErrorMsg(c, "三网优化入口不能删除")
		return
	}
	if err = model.DB.Delete(&profile).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	_ = model.EnsureDefaultAPIIngressProfiles()
	common.ApiSuccess(c, nil)
}
