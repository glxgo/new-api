package middleware

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// HTTPStats 存储HTTP统计信息
type HTTPStats struct {
	activeConnections int64
}

var globalStats = &HTTPStats{}

// StatsMiddleware 统计中间件
func StatsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		routeClass := classifyPath(c.Request.URL.Path)
		common.RecordTrafficStart(routeClass)
		// 增加活跃连接数
		atomic.AddInt64(&globalStats.activeConnections, 1)

		// 确保在请求结束时减少连接数
		defer func() {
			atomic.AddInt64(&globalStats.activeConnections, -1)
			common.RecordTrafficEnd(routeClass, c.Writer.Status(), started, c.Request.ContentLength, int64(c.Writer.Size()), c.Request.Context().Err() == context.Canceled)
		}()

		c.Next()
	}
}

func classifyPath(path string) string {
	switch {
	case strings.HasPrefix(path, "/v1beta"):
		return "relay-gemini"
	case strings.HasPrefix(path, "/v1"):
		return "relay-v1"
	case strings.HasPrefix(path, "/mj") || strings.Contains(path, "/mj/"):
		return "relay-mj"
	case strings.HasPrefix(path, "/suno"):
		return "relay-suno"
	case strings.HasPrefix(path, "/api/log"):
		return "admin-log"
	case strings.HasPrefix(path, "/api/token"):
		return "token"
	case strings.HasPrefix(path, "/api"):
		return "api"
	default:
		return "web"
	}
}

// StatsInfo 统计信息结构
type StatsInfo struct {
	ActiveConnections int64 `json:"active_connections"`
}

// GetStats 获取统计信息
func GetStats() StatsInfo {
	return StatsInfo{
		ActiveConnections: atomic.LoadInt64(&globalStats.activeConnections),
	}
}
