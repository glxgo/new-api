package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// SystemPerformanceCheck 检查系统性能中间件
func SystemPerformanceCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 仅检查 Relay 接口 (/v1, /v1beta 等)
		// 这里简单判断路径前缀，可以根据实际路由调整
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/v1/messages") {
			if err := checkSystemPerformance(); err != nil {
				c.JSON(err.StatusCode, gin.H{
					"error": err.ToClaudeError(),
				})
				c.Abort()
				return
			}
		} else {
			if err := checkSystemPerformance(); err != nil {
				c.JSON(err.StatusCode, gin.H{
					"error": err.ToOpenAIError(),
				})
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

// checkSystemPerformance 检查系统性能是否超过阈值
func checkSystemPerformance() *types.NewAPIError {
	config := common.GetPerformanceMonitorConfig()
	if !config.Enabled {
		return nil
	}

	status := common.GetSystemStatus()

	// 检查 CPU
	if config.CPUThreshold > 0 && int(status.CPUUsage) > config.CPUThreshold {
		return types.NewErrorWithStatusCode(
			fmt.Errorf(i18n.Translate(i18n.LangZhCN, i18n.MsgSystemCpuOverloaded, map[string]any{"Current": fmt.Sprintf("%.1f", status.CPUUsage), "Threshold": config.CPUThreshold})),
			"system_cpu_overloaded", http.StatusServiceUnavailable)
	}

	// 检查内存
	if config.MemoryThreshold > 0 && int(status.MemoryUsage) > config.MemoryThreshold {
		return types.NewErrorWithStatusCode(
			fmt.Errorf(i18n.Translate(i18n.LangZhCN, i18n.MsgSystemMemoryOverloaded, map[string]any{"Current": fmt.Sprintf("%.1f", status.MemoryUsage), "Threshold": config.MemoryThreshold})),
			"system_memory_overloaded", http.StatusServiceUnavailable)
	}

	// 检查磁盘
	if config.DiskThreshold > 0 && int(status.DiskUsage) > config.DiskThreshold {
		return types.NewErrorWithStatusCode(
			fmt.Errorf(i18n.Translate(i18n.LangZhCN, i18n.MsgSystemDiskOverloaded, map[string]any{"Current": fmt.Sprintf("%.1f", status.DiskUsage), "Threshold": config.DiskThreshold})),
			"system_disk_overloaded", http.StatusServiceUnavailable)
	}

	return nil
}
