package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const APIIngressHeader = "X-New-API-Ingress"

// APIIngressResolver binds the public entrance selected by the reverse proxy
// to the request context. A missing header uses the configured default entry
// so local development and the original URL keep working. Unknown or disabled
// entries are rejected instead of silently falling back to the wrong price.
func APIIngressResolver() gin.HandlerFunc {
	return func(c *gin.Context) {
		code := strings.TrimSpace(strings.ToLower(c.GetHeader(APIIngressHeader)))
		var profile *model.APIIngressProfile
		var err error
		// A configured public host is authoritative. This prevents a caller from
		// adding the header to the normal-priced URL to obtain the discounted
		// profile. The header remains useful behind an internal proxy where the
		// upstream Host is not the public hostname.
		hostProfile, hostErr := model.GetAPIIngressProfileByHost(c.Request.Host)
		if hostErr == nil && hostProfile != nil {
			profile, err = hostProfile, nil
			if !profile.Enabled {
				err = errors.New("API ingress disabled")
			}
		} else if code != "" {
			profile, err = model.GetAPIIngressProfileByCode(code)
			if err == nil && (profile == nil || !profile.Enabled) {
				err = errors.New("API ingress disabled")
			}
		}
		if err != nil && code == "" {
			profile, err = model.GetDefaultAPIIngressProfile()
		}
		if err != nil {
			// Keep setup/health endpoints usable before the first migration.
			profile = &model.APIIngressProfile{
				Code: model.APIIngressCodeOptimized, DisplayName: "三网优化 URL",
				NetworkMode: model.APIIngressNetworkLine, BillingMultiplierPPM: model.APIIngressPPMOne,
				Enabled: true, Visible: true, Default: true,
			}
			err = nil
		}
		if err != nil || profile == nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"success": false, "message": "API 入口不存在或未启用"})
			return
		}
		c.Set("api_ingress_code", profile.Code)
		c.Set("api_ingress_display_name", profile.DisplayName)
		c.Set("api_ingress_multiplier_ppm", profile.BillingMultiplierPPM)
		c.Set("api_ingress_network_mode", profile.NetworkMode)
		c.Set("api_ingress_profile", profile)
		c.Next()
	}
}
