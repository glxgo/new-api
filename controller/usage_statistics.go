package controller

import (
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type usageStatisticsPeriod struct {
	duration      time.Duration
	bucketSeconds int64
}

var usageStatisticsPeriods = map[string]usageStatisticsPeriod{
	"24h": {duration: 24 * time.Hour, bucketSeconds: int64(time.Hour / time.Second)},
	"7d":  {duration: 7 * 24 * time.Hour, bucketSeconds: int64(6 * time.Hour / time.Second)},
	"30d": {duration: 30 * 24 * time.Hour, bucketSeconds: int64(24 * time.Hour / time.Second)},
}

func GetUsageStatisticsSelf(c *gin.Context) {
	periodName := c.DefaultQuery("range", "7d")
	period, ok := usageStatisticsPeriods[periodName]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "range must be one of 24h, 7d, or 30d",
		})
		return
	}

	endTime := common.GetTimestamp()
	startTime := endTime - int64(period.duration/time.Second)
	stats, err := model.GetUserUsageStatistics(c.GetInt("id"), startTime, endTime, period.bucketSeconds)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"range":           periodName,
		"start_timestamp": startTime,
		"end_timestamp":   endTime,
		"bucket_seconds":  period.bucketSeconds,
		"summary":         stats.Summary,
		"series":          stats.Series,
		"models":          stats.Models,
		"subscriptions":   stats.Subscriptions,
	})
}
