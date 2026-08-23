package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/access_setting"
	"github.com/gin-gonic/gin"
)

// AccessPolicyRequest 管理端更新访问策略的请求体。指针字段表示“本次是否修改”。
type AccessPolicyRequest struct {
	BlockMainlandWebAccess *bool   `json:"block_mainland_web_access"`
	IncludeHkMoTW          *bool   `json:"include_hk_mo_tw"`
	GeoIPUnknownPolicy     *string `json:"geoip_unknown_policy"`
}

// GetAccessPolicy 返回当前访问策略、GeoIP 状态与进程内计数（AdminAuth）。
func GetAccessPolicy(c *gin.Context) {
	policy := access_setting.GetAccessPolicy()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"block_mainland_web_access": policy.BlockMainlandWebAccess,
			"include_hk_mo_tw":          policy.IncludeHkMoTW,
			"geoip_unknown_policy":      policy.GeoIPUnknownPolicy,
			"geoip_db_path":             access_setting.GetGeoIPDBPath(),
			"geoip_db_loaded":           middleware.GeoIPDBLoaded(),
			"geoip_db_version":          policy.GeoIPDBVersion,
			"config_version":            policy.ConfigVersion,
			"stats":                     middleware.GeoBlockStatsSnapshot(),
		},
	})
}

// UpdateAccessPolicy 更新访问策略，带版本号与上一版本快照（AdminAuth）。
func UpdateAccessPolicy(c *gin.Context) {
	var req AccessPolicyRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	current := access_setting.GetAccessPolicy()
	values := map[string]string{}
	if req.BlockMainlandWebAccess != nil {
		values["access_policy.block_mainland_web_access"] = strconv.FormatBool(*req.BlockMainlandWebAccess)
	}
	if req.IncludeHkMoTW != nil {
		values["access_policy.include_hk_mo_tw"] = strconv.FormatBool(*req.IncludeHkMoTW)
	}
	if req.GeoIPUnknownPolicy != nil {
		policyValue := strings.TrimSpace(*req.GeoIPUnknownPolicy)
		if policyValue != access_setting.GeoIPUnknownPolicyAllow && policyValue != access_setting.GeoIPUnknownPolicyDeny {
			common.ApiErrorMsg(c, "invalid geoip_unknown_policy, must be allow or deny")
			return
		}
		values["access_policy.geoip_unknown_policy"] = policyValue
	}
	if len(values) == 0 {
		common.ApiErrorMsg(c, "nothing to update")
		return
	}

	previous := map[string]string{
		"access_policy.block_mainland_web_access": strconv.FormatBool(current.BlockMainlandWebAccess),
		"access_policy.include_hk_mo_tw":          strconv.FormatBool(current.IncludeHkMoTW),
		"access_policy.geoip_unknown_policy":      current.GeoIPUnknownPolicy,
		"access_policy.geoip_db_path":             current.GeoIPDBPath,
		"access_policy.geoip_db_version":          current.GeoIPDBVersion,
		"access_policy.config_version":            strconv.FormatInt(current.ConfigVersion, 10),
	}
	prevJSON, err := common.Marshal(previous)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	nextVersion := current.ConfigVersion + 1
	values["access_policy.previous"] = string(prevJSON)
	values["access_policy.config_version"] = strconv.FormatInt(nextVersion, 10)
	if err := model.UpdateOptionsBulk(values); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "access_policy.update", map[string]interface{}{
		"version": nextVersion,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"config_version": nextVersion,
		},
	})
}

// RollbackAccessPolicy 恢复上一版本策略（AdminAuth）。
func RollbackAccessPolicy(c *gin.Context) {
	prevJSON, err := model.GetOptionValue("access_policy.previous")
	if err != nil || prevJSON == "" {
		common.ApiErrorMsg(c, "没有可回滚的上一版本")
		return
	}
	var previous map[string]string
	if err := common.UnmarshalJsonStr(prevJSON, &previous); err != nil {
		common.ApiError(c, err)
		return
	}
	if _, ok := previous["access_policy.block_mainland_web_access"]; !ok {
		common.ApiErrorMsg(c, "上一版本快照不完整")
		return
	}
	current := access_setting.GetAccessPolicy()
	nextVersion := current.ConfigVersion + 1
	previous["access_policy.config_version"] = strconv.FormatInt(nextVersion, 10)
	previous["access_policy.previous"] = prevJSON
	if err := model.UpdateOptionsBulk(previous); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "access_policy.rollback", map[string]interface{}{
		"version": nextVersion,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"config_version": nextVersion,
		},
	})
}
