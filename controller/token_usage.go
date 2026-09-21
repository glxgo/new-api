package controller

import (
	"context"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// GetTokenUsageStats keeps expensive log aggregates off the key-management
// critical path. Only owned, non-deleted tokens may reach the aggregate/cache.
func GetTokenUsageStats(c *gin.Context) {
	var request struct {
		IDs []int `json:"ids"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || len(request.IDs) == 0 || len(request.IDs) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请提供 1–100 个 API Key ID"})
		return
	}
	seen := make(map[int]bool, len(request.IDs))
	ids := make([]int, 0, len(request.IDs))
	for _, id := range request.IDs {
		if id <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的 API Key ID"})
			return
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	var owned []int
	if err := model.DB.WithContext(ctx).Model(&model.Token{}).
		Where("user_id = ? AND id IN ?", c.GetInt("id"), ids).Pluck("id", &owned).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if len(owned) != len(ids) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "API Key 不存在或无权访问"})
		return
	}
	// This endpoint explicitly serves a 30-second snapshot. Quantize BOTH the
	// query time and its cache identity; legacy exact-time readers are unchanged.
	asOf := time.Now().Truncate(30 * time.Second)
	stats, err := model.GetTokenUsageStatsWithContext(ctx, ids, asOf)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	type item struct {
		ID       int   `json:"id"`
		Today    int64 `json:"today_used_quota"`
		Lifetime int64 `json:"lifetime_used_quota"`
		Stale    bool  `json:"stale"`
	}
	items := make([]item, 0, len(ids))
	for _, id := range ids {
		value := stats[id]
		items = append(items, item{id, value.TodayUsedQuota, value.LifetimeUsedQuota, value.Stale})
	}
	common.ApiSuccess(c, gin.H{"items": items, "as_of": asOf.Unix()})
}
