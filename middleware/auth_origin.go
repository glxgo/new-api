package middleware

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// SessionCookieOriginGuard protects cookie-authenticated logout. It is only
// active in Secure-cookie mode and is deliberately not installed on relay
// routes, where Origin is unrelated to provider authentication.
func SessionCookieOriginGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !common.SessionCookieSecure {
			c.Next()
			return
		}
		origin, ok := requestBrowserOrigin(c.Request)
		if !ok || !isAllowedSessionOrigin(c.Request, origin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"code":    "AUTH_ORIGIN_FORBIDDEN",
				"message": "request origin is not allowed",
			})
			return
		}
		c.Next()
	}
}

func requestBrowserOrigin(request *http.Request) (string, bool) {
	origins := request.Header.Values("Origin")
	if len(origins) > 1 || (len(origins) == 1 && strings.Contains(origins[0], ",")) {
		return "", false
	}
	if len(origins) == 1 {
		origin, err := common.NormalizeOrigin(origins[0])
		return origin, err == nil
	}
	referers := request.Header.Values("Referer")
	if len(referers) != 1 {
		return "", false
	}
	referer, err := url.Parse(strings.TrimSpace(referers[0]))
	if err != nil || referer.Scheme == "" || referer.Host == "" || referer.User != nil {
		return "", false
	}
	origin, err := common.NormalizeOrigin(referer.Scheme + "://" + referer.Host)
	return origin, err == nil
}

func isAllowedSessionOrigin(request *http.Request, origin string) bool {
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}
	requestOrigin, err := common.NormalizeOrigin(scheme + "://" + request.Host)
	if err == nil && subtle.ConstantTimeCompare([]byte(origin), []byte(requestOrigin)) == 1 {
		return true
	}
	for _, trustedOrigin := range common.SessionCookieTrustedURLs {
		if subtle.ConstantTimeCompare([]byte(origin), []byte(trustedOrigin)) == 1 {
			return true
		}
	}
	return false
}
