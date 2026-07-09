package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

func Cache() func(c *gin.Context) {
	return func(c *gin.Context) {
		uri := c.Request.RequestURI
		// API(/api/ /v1/) 不缓存, 防 CF/CDN 缓存动态接口(新接口上线前会被缓存 404)
		if uri == "/" || strings.HasPrefix(uri, "/api/") || strings.HasPrefix(uri, "/v1/") {
			c.Header("Cache-Control", "no-cache")
		} else {
			c.Header("Cache-Control", "max-age=604800") // one week
		}
		c.Header("Cache-Version", "b688f2fb5be447c25e5aa3bd063087a83db32a288bf6a4f35f2d8db310e40b14")
		c.Next()
	}
}
