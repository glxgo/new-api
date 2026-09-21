package controller

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
	"github.com/gin-gonic/gin"
)

var channelMetricsZone = time.FixedZone("Asia/Shanghai", 8*60*60)

func channelMetricsWindow(period, date string, now time.Time) (int64, int64, error) {
	end := now.Unix() / 60 * 60
	if period == "hour" {
		return end - 3600, end, nil
	}
	if period != "day" {
		return 0, 0, fmt.Errorf("period must be hour or day")
	}
	if date == "" {
		date = now.In(channelMetricsZone).Format("2006-01-02")
	}
	day, err := time.ParseInLocation("2006-01-02", date, channelMetricsZone)
	if err != nil || day.Unix() > now.Unix() {
		return 0, 0, fmt.Errorf("invalid date (expected YYYY-MM-DD, not in the future)")
	}
	dayEnd := day.AddDate(0, 0, 1).Unix()
	if dayEnd < end {
		end = dayEnd
	}
	return day.Unix(), end, nil
}

type adminChannelMetric struct {
	model.ChannelMetricIdentity
	perfmetrics.ChannelSummary
}

func GetChannelMetrics(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	period := c.DefaultQuery("period", "hour")
	start, end, err := channelMetricsWindow(period, c.Query("date"), time.Now())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	page := common.GetPageQuery(c)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	channels, total, err := model.GetChannelMetricPage(ctx, c.Query("keyword"), page.GetStartIdx(), page.PageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	ids := make([]int, 0, len(channels))
	for _, row := range channels {
		ids = append(ids, row.Id)
	}
	metrics, err := perfmetrics.QueryChannelSummaries(ctx, start, end, ids)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]adminChannelMetric, 0, len(channels))
	for _, row := range channels {
		items = append(items, adminChannelMetric{row, metrics[row.Id]})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"items": items, "total": total, "page": page.Page, "page_size": page.PageSize,
		"period": period, "start_ts": start, "end_ts": end, "timezone": "Asia/Shanghai", "bucket_seconds": 60,
		"enabled":                perf_metrics_setting.GetSetting().Enabled,
		"flush_interval_minutes": perf_metrics_setting.GetFlushIntervalMinutes(),
	}})
}
