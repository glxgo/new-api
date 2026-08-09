package controller

import (
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const platformUsageSiteCacheTTL = 30 * time.Second

type platformUsageSiteCacheEntry struct {
	startTime int64
	updatedAt time.Time
	data      model.PlatformUsageToday
}

var (
	platformUsageSiteCacheMu sync.Mutex
	platformUsageSiteCache   platformUsageSiteCacheEntry
)

func chinaDayRange(now time.Time) (int64, int64) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	localNow := now.In(location)
	start := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	return start.Unix(), now.Unix()
}

func getCachedPlatformUsageToday(startTime, endTime int64) (model.PlatformUsageToday, error) {
	platformUsageSiteCacheMu.Lock()
	defer platformUsageSiteCacheMu.Unlock()
	if platformUsageSiteCache.startTime == startTime && time.Since(platformUsageSiteCache.updatedAt) < platformUsageSiteCacheTTL {
		return platformUsageSiteCache.data, nil
	}
	data, err := model.GetPlatformUsageToday(startTime, endTime)
	if err != nil {
		return data, err
	}
	platformUsageSiteCache = platformUsageSiteCacheEntry{startTime: startTime, updatedAt: time.Now(), data: data}
	return data, nil
}

// GetPlatformUsageOverview returns only site-level aggregates and a sanitized,
// periodically refreshed CPA snapshot. It never queries CPA on a page request.
func GetPlatformUsageOverview(c *gin.Context) {
	startTime, endTime := chinaDayRange(time.Now())
	siteUsage, err := getCachedPlatformUsageToday(startTime, endTime)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"timezone":        "Asia/Shanghai",
		"start_timestamp": startTime,
		"end_timestamp":   endTime,
		"site":            siteUsage,
		"cpa":             service.GetCPAUsageSnapshot(),
	})
}
