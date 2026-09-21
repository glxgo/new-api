package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/samber/hot"
	"golang.org/x/sync/singleflight"
)

const (
	dashboardTrafficCacheTTL     = 45 * time.Second
	dashboardTrafficStaleTTL     = 5 * time.Minute
	dashboardTrafficSharedBudget = 2 * time.Minute
	// Keep the lease longer than the shared query budget so a slow but bounded
	// database read cannot cause a second instance to start the same scan while
	// the first one is still publishing its result.
	dashboardTrafficLeaseTTL = dashboardTrafficSharedBudget + 15*time.Second
	// A waiter should not spend its whole request budget polling Redis. If the
	// holder does not publish quickly, stale/local fallback is preferable to
	// turning a cache coordination hiccup into a dashboard timeout.
	dashboardTrafficLeaseWait  = 5 * time.Second
	dashboardTrafficProjection = "dashboard-traffic-v1"
)

var (
	dashboardTrafficHotCache = hot.NewHotCache[string, DashboardTrafficResult](hot.LRU, 128).
					WithTTL(dashboardTrafficCacheTTL).
					WithJanitor().
					Build()
	dashboardTrafficStaleCache = hot.NewHotCache[string, DashboardTrafficResult](hot.LRU, 128).
					WithTTL(dashboardTrafficStaleTTL).
					WithJanitor().
					Build()
	dashboardTrafficQueryGroup         singleflight.Group
	dashboardTrafficLeaseReleaseScript = redis.NewScript(`
		if redis.call('get', KEYS[1]) == ARGV[1] then
			return redis.call('del', KEYS[1])
		end
		return 0
	`)
)

// GetDashboardTrafficResultWithContext reads, aggregates, and caches one
// dashboard response as a single projection.  The cache stores only the
// compact response (summary/daily/channel aggregates), never raw log rows.
// A process-local singleflight prevents duplicate work in one instance; when
// Redis is available a short lease lets independent instances converge on one
// database read during a cold-cache miss.
func GetDashboardTrafficResultWithContext(ctx context.Context, userID int, startTime, endTime int64, location *time.Location, includeChannels bool) (DashboardTrafficResult, error) {
	result := emptyDashboardTrafficResult()
	if startTime <= 0 || endTime <= startTime {
		return result, fmt.Errorf("invalid dashboard traffic range")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if location == nil {
		location = time.UTC
	}
	startZoneName, startZoneOffset := time.Unix(startTime, 0).In(location).Zone()
	endZoneName, endZoneOffset := time.Unix(endTime-1, 0).In(location).Zone()
	timezone := startZoneName + "/" + strconv.Itoa(startZoneOffset) + "->" + endZoneName + "/" + strconv.Itoa(endZoneOffset)
	filter := fmt.Sprintf("projection=%s;channels=%t", dashboardTrafficProjection, includeChannels)
	remoteKey := model.UsageProjectionCacheKey("dashboard-traffic", userID, startTime, endTime, 0, timezone, filter)
	localKey := model.UsageProjectionCacheLocalKey(remoteKey)

	if cached, found := dashboardTrafficHotCache.MustGet(localKey); found {
		return cloneDashboardTrafficResult(cached), nil
	}
	var remote DashboardTrafficResult
	if found, err := model.UsageProjectionCacheGet(ctx, remoteKey, &remote); err == nil && found {
		copyValue := cloneDashboardTrafficResult(remote)
		dashboardTrafficHotCache.SetWithTTL(localKey, copyValue, dashboardTrafficCacheTTL)
		dashboardTrafficStaleCache.SetWithTTL(localKey, cloneDashboardTrafficResult(copyValue), dashboardTrafficStaleTTL)
		return copyValue, nil
	}

	resultCh := dashboardTrafficQueryGroup.DoChan(localKey, func() (any, error) {
		workCtx, workCancel := context.WithTimeout(context.WithoutCancel(ctx), dashboardTrafficSharedBudget)
		defer workCancel()
		if cached, found := dashboardTrafficHotCache.MustGet(localKey); found {
			return cloneDashboardTrafficResult(cached), nil
		}
		var shared DashboardTrafficResult
		if found, err := model.UsageProjectionCacheGet(workCtx, remoteKey, &shared); err == nil && found {
			copyValue := cloneDashboardTrafficResult(shared)
			dashboardTrafficHotCache.SetWithTTL(localKey, copyValue, dashboardTrafficCacheTTL)
			dashboardTrafficStaleCache.SetWithTTL(localKey, cloneDashboardTrafficResult(copyValue), dashboardTrafficStaleTTL)
			return copyValue, nil
		}

		leaseKey := remoteKey + ":lock"
		leaseToken, leaseHeld := acquireDashboardTrafficLease(workCtx, leaseKey)
		if !leaseHeld {
			// Another instance is already doing the expensive read. Give it a
			// bounded chance to publish the result before falling back locally.
			if published, ok := waitDashboardTrafficSharedCache(workCtx, remoteKey); ok {
				copyValue := cloneDashboardTrafficResult(published)
				dashboardTrafficHotCache.SetWithTTL(localKey, copyValue, dashboardTrafficCacheTTL)
				dashboardTrafficStaleCache.SetWithTTL(localKey, cloneDashboardTrafficResult(copyValue), dashboardTrafficStaleTTL)
				return copyValue, nil
			}
			var stale DashboardTrafficResult
			if found, err := model.UsageProjectionCacheGet(workCtx, remoteKey+":stale", &stale); err == nil && found {
				copyValue := cloneDashboardTrafficResult(stale)
				dashboardTrafficHotCache.SetWithTTL(localKey, copyValue, dashboardTrafficCacheTTL)
				return copyValue, nil
			}
			// The lease holder may have failed before publishing. Proceed with a
			// bounded local query rather than waiting forever or changing the HTTP
			// error contract.
		}
		if leaseToken != "" {
			defer releaseDashboardTrafficLease(workCtx, leaseKey, leaseToken)
		}

		records, err := model.GetDashboardTrafficRecordsWithContext(workCtx, userID, startTime, endTime)
		if err != nil {
			if stale, found := dashboardTrafficStaleCache.MustGet(localKey); found {
				return cloneDashboardTrafficResult(stale), nil
			}
			var remoteStale DashboardTrafficResult
			if found, remoteErr := model.UsageProjectionCacheGet(workCtx, remoteKey+":stale", &remoteStale); remoteErr == nil && found {
				copyValue := cloneDashboardTrafficResult(remoteStale)
				dashboardTrafficStaleCache.SetWithTTL(localKey, copyValue, dashboardTrafficStaleTTL)
				return copyValue, nil
			}
			return nil, err
		}

		channelNames := map[int]string{}
		if includeChannels {
			channelSet := make(map[int]struct{})
			for _, record := range records {
				if record.ChannelId > 0 {
					channelSet[record.ChannelId] = struct{}{}
				}
			}
			channelIDs := make([]int, 0, len(channelSet))
			for channelID := range channelSet {
				channelIDs = append(channelIDs, channelID)
			}
			channelNames, err = model.GetDashboardChannelNamesWithContext(workCtx, channelIDs)
			if err != nil {
				if stale, found := dashboardTrafficStaleCache.MustGet(localKey); found {
					return cloneDashboardTrafficResult(stale), nil
				}
				return nil, err
			}
		}

		queried := BuildDashboardTraffic(records, channelNames, startTime, endTime, location, includeChannels)
		copyValue := cloneDashboardTrafficResult(queried)
		dashboardTrafficHotCache.SetWithTTL(localKey, copyValue, dashboardTrafficCacheTTL)
		dashboardTrafficStaleCache.SetWithTTL(localKey, cloneDashboardTrafficResult(copyValue), dashboardTrafficStaleTTL)
		_ = model.UsageProjectionCacheSet(workCtx, remoteKey, copyValue, dashboardTrafficCacheTTL)
		_ = model.UsageProjectionCacheSet(workCtx, remoteKey+":stale", copyValue, dashboardTrafficStaleTTL)
		return copyValue, nil
	})
	value, err := waitDashboardTrafficResult(ctx, resultCh)
	if err != nil {
		return result, err
	}
	queried, ok := value.(DashboardTrafficResult)
	if !ok {
		return result, fmt.Errorf("invalid dashboard traffic cache value")
	}
	return cloneDashboardTrafficResult(queried), nil
}

func waitDashboardTrafficResult(ctx context.Context, resultCh <-chan singleflight.Result) (any, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case result := <-resultCh:
		return result.Val, result.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func acquireDashboardTrafficLease(ctx context.Context, key string) (string, bool) {
	if !common.RedisEnabled || common.RDB == nil {
		return "", true
	}
	token := uuid.NewString()
	acquired, err := common.RDB.SetNX(ctx, key, token, dashboardTrafficLeaseTTL).Result()
	if err != nil {
		// Redis is an optional acceleration layer. A Redis outage must not turn
		// the legacy dashboard endpoint into a new failure mode.
		return "", true
	}
	return token, acquired
}

func releaseDashboardTrafficLease(ctx context.Context, key, token string) {
	if token == "" || !common.RedisEnabled || common.RDB == nil {
		return
	}
	_, _ = dashboardTrafficLeaseReleaseScript.Run(ctx, common.RDB, []string{key}, token).Result()
}

func waitDashboardTrafficSharedCache(ctx context.Context, remoteKey string) (DashboardTrafficResult, bool) {
	result := emptyDashboardTrafficResult()
	if !common.RedisEnabled || common.RDB == nil {
		return result, false
	}
	deadline := time.Now().Add(dashboardTrafficLeaseWait)
	if parentDeadline, ok := ctx.Deadline(); ok && parentDeadline.Before(deadline) {
		deadline = parentDeadline
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		var remote DashboardTrafficResult
		if found, err := model.UsageProjectionCacheGet(ctx, remoteKey, &remote); err == nil && found {
			return remote, true
		}
		if time.Now().After(deadline) {
			return result, false
		}
		select {
		case <-ctx.Done():
			return result, false
		case <-ticker.C:
		}
	}
}

func emptyDashboardTrafficResult() DashboardTrafficResult {
	return DashboardTrafficResult{
		Daily:    make([]DashboardTrafficDaily, 0),
		Channels: nil,
	}
}

func cloneDashboardTrafficResult(source DashboardTrafficResult) DashboardTrafficResult {
	result := source
	if source.Daily != nil {
		result.Daily = append([]DashboardTrafficDaily(nil), source.Daily...)
	}
	if source.Channels != nil {
		result.Channels = make([]DashboardChannelTraffic, len(source.Channels))
		for index, channel := range source.Channels {
			result.Channels[index] = channel
			if channel.Daily != nil {
				result.Channels[index].Daily = append([]DashboardTrafficDaily(nil), channel.Daily...)
			}
		}
	}
	return result
}
