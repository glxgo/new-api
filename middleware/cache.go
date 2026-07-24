package middleware

import (
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var fingerprintedAssetPattern = regexp.MustCompile(`\.[0-9a-f]{8,}\.[^/]+$`)

func isStaticAssetPath(uri string) bool {
	if strings.HasPrefix(uri, "/static/") || strings.HasPrefix(uri, "/assets/") {
		return true
	}
	for _, suffix := range []string{
		".avif", ".gif", ".ico", ".jpeg", ".jpg", ".png", ".svg", ".webp",
		".woff", ".woff2",
	} {
		if strings.HasSuffix(uri, suffix) {
			return true
		}
	}
	return false
}

func Cache() func(c *gin.Context) {
	return func(c *gin.Context) {
		uri := c.Request.URL.Path
		// API(/api/ /v1/) 不缓存, 防 CF/CDN 缓存动态接口(新接口上线前会被缓存 404)
		if uri == "/" || strings.HasPrefix(uri, "/api/") || strings.HasPrefix(uri, "/v1/") {
			c.Header("Cache-Control", "no-cache")
		} else if strings.HasPrefix(uri, "/static/") && fingerprintedAssetPattern.MatchString(uri) {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else if isStaticAssetPath(uri) {
			c.Header("Cache-Control", "public, max-age=604800") // one week
		} else {
			// Client-side routes are HTML documents even when their path looks
			// file-like; they must always pick up the latest asset manifest.
			c.Header("Cache-Control", "no-cache")
		}
		c.Header("Cache-Version", "b688f2fb5be447c25e5aa3bd063087a83db32a288bf6a4f35f2d8db310e40b14")
		c.Next()
	}
}
