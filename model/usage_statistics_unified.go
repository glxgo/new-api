package model

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/samber/hot"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

const (
	usageStatisticsCacheTTL       = 30 * time.Second
	usageStatisticsHistoricalTTL  = 5 * time.Minute
	usageStatisticsStaleTTL       = 5 * time.Minute
	usageStatisticsProjectionName = "usage-statistics-v2"
)

var (
	usageStatisticsHotCache = hot.NewHotCache[string, UsageStatistics](hot.LRU, 256).
				WithTTL(usageStatisticsCacheTTL).
				WithJanitor().
				Build()
	usageStatisticsStaleCache = hot.NewHotCache[string, UsageStatistics](hot.LRU, 256).
					WithTTL(usageStatisticsStaleTTL).
					WithJanitor().
					Build()
	usageStatisticsQueryGroup singleflight.Group
)

type usageStatisticsStreamRow struct {
	CreatedAt             int64  `gorm:"column:created_at"`
	UserID                int    `gorm:"column:user_id"`
	Type                  int    `gorm:"column:type"`
	Quota                 int64  `gorm:"column:quota"`
	PreDiscountQuota      int64  `gorm:"column:pre_discount_quota"`
	BillingSource         string `gorm:"column:billing_source"`
	PromptTokens          int64  `gorm:"column:prompt_tokens"`
	CacheTokens           int64  `gorm:"column:cache_tokens"`
	CompletionTokens      int64  `gorm:"column:completion_tokens"`
	ModelName             string `gorm:"column:model_name"`
	SubscriptionID        int    `gorm:"column:subscription_id"`
	RequestCount          int64  `gorm:"column:request_count"`
	SuccessCount          int64  `gorm:"column:success_count"`
	ErrorCount            int64  `gorm:"column:error_count"`
	EffectivePromptTokens int64  `gorm:"column:effective_prompt_tokens"`
}

// usageStatisticsAccumulator is shared by raw-log rows and metric-bucket
// rows.  Keeping one accounting implementation is important when a query is
// split into complete projected buckets plus two exact raw tails: the two
// sources must contribute to summary, series, model and subscription views in
// exactly the same way.
type usageStatisticsAccumulator struct {
	result        UsageStatistics
	series        map[int64]*UsageStatisticsPoint
	models        map[string]*UsageStatisticsModel
	subscriptions map[int]*UsageStatisticsSubscription
}

func newUsageStatisticsAccumulator() *usageStatisticsAccumulator {
	return &usageStatisticsAccumulator{
		result: UsageStatistics{
			Series:        make([]UsageStatisticsPoint, 0),
			Models:        make([]UsageStatisticsModel, 0),
			Subscriptions: make([]UsageStatisticsSubscription, 0),
		},
		series:        make(map[int64]*UsageStatisticsPoint),
		models:        make(map[string]*UsageStatisticsModel),
		subscriptions: make(map[int]*UsageStatisticsSubscription),
	}
}

func (acc *usageStatisticsAccumulator) add(row usageStatisticsStreamRow, bucketSeconds int64) {
	if acc == nil || bucketSeconds <= 0 {
		return
	}
	requestCount := row.RequestCount
	if requestCount <= 0 {
		requestCount = 1
	}
	bucket := row.CreatedAt / bucketSeconds * bucketSeconds
	point := acc.series[bucket]
	if point == nil {
		point = &UsageStatisticsPoint{Timestamp: bucket}
		acc.series[bucket] = point
	}
	point.RequestCount += requestCount
	switch row.Type {
	case LogTypeConsume:
		successCount := requestCount
		if row.SuccessCount > 0 {
			successCount = row.SuccessCount
		}
		point.SuccessCount += successCount
		point.Quota += row.Quota
		point.TotalTokens += row.PromptTokens + row.CompletionTokens
		if row.PromptTokens > 0 {
			point.CacheTokens += row.CacheTokens
		}
		point.EffectivePrompt += row.EffectivePromptTokens
		acc.result.Summary.SuccessCount += successCount
		acc.result.Summary.Quota += row.Quota
		acc.result.Summary.PreDiscountQuota += row.PreDiscountQuota
		acc.result.Summary.PromptTokens += row.PromptTokens
		if row.PromptTokens > 0 {
			acc.result.Summary.CacheTokens += row.CacheTokens
		}
		acc.result.Summary.EffectivePrompt += row.EffectivePromptTokens
		acc.result.Summary.CompletionTokens += row.CompletionTokens
		switch row.BillingSource {
		case "subscription":
			acc.result.Summary.SubscriptionQuota += row.Quota
		case "virtual_membership":
			acc.result.Summary.VirtualMembershipQuota += row.Quota
		case "", "wallet":
			acc.result.Summary.WalletQuota += row.Quota
		}
		modelName := row.ModelName
		if modelName == "" {
			modelName = "unknown"
		}
		model := acc.models[modelName]
		if model == nil {
			model = &UsageStatisticsModel{ModelName: modelName}
			acc.models[modelName] = model
		}
		model.RequestCount += requestCount
		model.Quota += row.Quota
		model.PromptTokens += row.PromptTokens
		if row.PromptTokens > 0 {
			model.CacheTokens += row.CacheTokens
		}
		model.CompletionTokens += row.CompletionTokens
		model.TotalTokens += row.PromptTokens + row.CompletionTokens
		if row.BillingSource == "subscription" {
			subscription := acc.subscriptions[row.SubscriptionID]
			if subscription == nil {
				subscription = &UsageStatisticsSubscription{SubscriptionId: row.SubscriptionID}
				acc.subscriptions[row.SubscriptionID] = subscription
			}
			subscription.RequestCount += requestCount
			subscription.Quota += row.Quota
		}
	case LogTypeError:
		errorCount := requestCount
		if row.ErrorCount > 0 {
			errorCount = row.ErrorCount
		}
		point.ErrorCount += errorCount
		acc.result.Summary.ErrorCount += errorCount
	}
}

func (acc *usageStatisticsAccumulator) finish(userID int, startTime, endTime, bucketSeconds int64) (UsageStatistics, error) {
	if acc == nil {
		return UsageStatistics{}, errors.New("usage statistics accumulator is nil")
	}
	result := acc.result
	result.Summary.RequestCount = result.Summary.SuccessCount + result.Summary.ErrorCount
	if result.Summary.RequestCount > 0 {
		result.Summary.SuccessRate = float64(result.Summary.SuccessCount) * 100 / float64(result.Summary.RequestCount)
	}
	result.Summary.TotalTokens = result.Summary.PromptTokens + result.Summary.CompletionTokens
	if result.Summary.EffectivePrompt > 0 {
		result.Summary.CacheHitRate = float64(result.Summary.CacheTokens) * 100 / float64(result.Summary.EffectivePrompt)
	}

	points := make([]UsageStatisticsPoint, 0, len(acc.series))
	for _, point := range acc.series {
		points = append(points, *point)
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Timestamp < points[j].Timestamp })
	result.Series = fillUsageStatisticsSeries(points, startTime, endTime, bucketSeconds)

	for _, item := range acc.models {
		result.Models = append(result.Models, *item)
	}
	sort.Slice(result.Models, func(i, j int) bool {
		if result.Models[i].RequestCount != result.Models[j].RequestCount {
			return result.Models[i].RequestCount > result.Models[j].RequestCount
		}
		return result.Models[i].ModelName < result.Models[j].ModelName
	})
	if len(result.Models) > 10 {
		result.Models = result.Models[:10]
	}
	for _, item := range acc.subscriptions {
		if item.Quota > 0 {
			result.Subscriptions = append(result.Subscriptions, *item)
		}
	}
	sort.Slice(result.Subscriptions, func(i, j int) bool {
		if result.Subscriptions[i].Quota != result.Subscriptions[j].Quota {
			return result.Subscriptions[i].Quota > result.Subscriptions[j].Quota
		}
		return result.Subscriptions[i].SubscriptionId < result.Subscriptions[j].SubscriptionId
	})
	if err := hydrateUsageStatisticsSubscriptionTitles(userID, result.Subscriptions); err != nil {
		return result, err
	}
	return result, nil
}

func cloneUsageStatistics(value UsageStatistics) UsageStatistics {
	value.Series = append([]UsageStatisticsPoint(nil), value.Series...)
	value.Models = append([]UsageStatisticsModel(nil), value.Models...)
	value.Subscriptions = append([]UsageStatisticsSubscription(nil), value.Subscriptions...)
	return value
}

func usageStatisticsCacheFilter(bucketSeconds int64) string {
	return fmt.Sprintf("projection=%s;bucket=%d", usageStatisticsProjectionName, bucketSeconds)
}

func usageStatisticsCacheTimesAreHot(endTime int64) bool {
	return endTime >= time.Now().Add(-2*time.Hour).Unix()
}

// usageStatisticsArchiveWindow returns the range of *complete local calendar
// days* that can safely be read from usage_log_daily_aggregates.  A daily
// aggregate has no hour-level information, so including the bucket that
// contains a partial range boundary would over-count the selected interval.
// The live-log query still covers both boundary tails exactly.
func usageStatisticsArchiveWindow(startTime, endTime int64) (int64, int64) {
	if startTime <= 0 || endTime <= startTime {
		return 0, 0
	}
	startBucket := usageLogAggregateBucketStart(startTime)
	endBucket := usageLogAggregateBucketStart(endTime)
	archiveStart := startBucket
	if startTime > startBucket {
		// API callers and old fixtures sometimes represent an exact midnight
		// with a one-second-open lower bound (for example [day+1s, next-day)).
		// Keep that historical boundary tolerance; any materially partial day
		// still starts at the following midnight and cannot over-count its daily
		// aggregate.
		if startTime-startBucket > 1 {
			archiveStart = usageStatisticsNextLocalDay(startBucket)
		}
	}
	// endBucket is already the exclusive boundary when endTime is exactly at
	// midnight; otherwise it excludes the current partial day by construction.
	archiveEnd := endBucket
	if archiveStart >= archiveEnd {
		return 0, 0
	}
	return archiveStart, archiveEnd
}

func usageStatisticsNextLocalDay(bucketStart int64) int64 {
	value := time.Unix(bucketStart, 0).In(time.Local)
	return time.Date(
		value.Year(), value.Month(), value.Day()+1,
		0, 0, 0, 0, time.Local,
	).Unix()
}

// usageStatisticsArchiveScanWindow includes the boundary buckets in the
// archive scan. They are filtered by FirstLogAt/LastLogAt below so a batch
// that is wholly inside a partial range can still be counted exactly, while a
// batch crossing the range boundary is left to the live/raw tail.
func usageStatisticsArchiveScanWindow(startTime, endTime int64) (int64, int64) {
	if startTime <= 0 || endTime <= startTime {
		return 0, 0
	}
	startBucket := usageLogAggregateBucketStart(startTime)
	endBucket := usageLogAggregateBucketStart(endTime)
	scanEnd := endBucket
	if endTime > endBucket {
		scanEnd = usageStatisticsNextLocalDay(endBucket)
	}
	if scanEnd <= startBucket {
		return 0, 0
	}
	return startBucket, scanEnd
}

// streamUsageStatisticsRowsFromSource executes one bounded SQL result stream.
// Keeping the live-only form separate lets callers retry safely when an old
// installation has not created usage_log_daily_aggregates yet.
func streamUsageStatisticsRowsFromSource(ctx context.Context, userID int, startTime, endTime, archiveScanStart, archiveScanEnd, archiveStart, archiveEnd int64, includeArchive bool, visit func(usageStatisticsStreamRow) error) (bool, error) {
	if LOG_DB == nil {
		return false, errors.New("log database is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var query *gorm.DB
	if includeArchive && archiveScanStart < archiveScanEnd {
		query = LOG_DB.WithContext(ctx).Raw(`
		SELECT created_at, user_id, type, quota,
			CASE WHEN pre_discount_quota > 0 THEN pre_discount_quota ELSE quota END AS pre_discount_quota,
			billing_source, prompt_tokens, cache_tokens, completion_tokens, model_name, subscription_id,
			1 AS request_count,
			CASE WHEN prompt_tokens > 0 THEN CASE WHEN cache_tokens > prompt_tokens THEN prompt_tokens + cache_tokens ELSE prompt_tokens END ELSE 0 END AS effective_prompt_tokens
		FROM logs
		WHERE user_id = ? AND created_at >= ? AND created_at < ? AND type IN (?, ?)
		UNION ALL
		SELECT bucket_start AS created_at, user_id, type, quota,
			CASE WHEN pre_discount_quota > 0 THEN pre_discount_quota ELSE quota END AS pre_discount_quota,
			billing_source, prompt_tokens, cache_tokens, completion_tokens, model_name, subscription_id,
			request_count, effective_prompt_tokens
		FROM usage_log_daily_aggregates
		WHERE user_id = ? AND bucket_start >= ? AND bucket_start < ? AND type IN (?, ?)
			AND (
				(bucket_start >= ? AND bucket_start < ?)
				OR (first_log_at >= ? AND last_log_at < ?)
				OR (bucket_start = ? AND first_log_at = 0 AND last_log_at = 0 AND ? <= 1)
			)`,
			userID, startTime, endTime, LogTypeConsume, LogTypeError,
			userID, archiveScanStart, archiveScanEnd, LogTypeConsume, LogTypeError,
			archiveStart, archiveEnd, startTime, endTime,
			usageLogAggregateBucketStart(startTime), startTime-usageLogAggregateBucketStart(startTime),
		)
	} else {
		query = LOG_DB.WithContext(ctx).Raw(`
		SELECT created_at, user_id, type, quota,
			CASE WHEN pre_discount_quota > 0 THEN pre_discount_quota ELSE quota END AS pre_discount_quota,
			billing_source, prompt_tokens, cache_tokens, completion_tokens, model_name, subscription_id,
			1 AS request_count,
			CASE WHEN prompt_tokens > 0 THEN CASE WHEN cache_tokens > prompt_tokens THEN prompt_tokens + cache_tokens ELSE prompt_tokens END ELSE 0 END AS effective_prompt_tokens
		FROM logs
		WHERE user_id = ? AND created_at >= ? AND created_at < ? AND type IN (?, ?)`,
			userID, startTime, endTime, LogTypeConsume, LogTypeError,
		)
	}
	rows, err := query.Rows()
	if err != nil {
		return false, err
	}
	defer rows.Close()
	visited := false
	for rows.Next() {
		var row usageStatisticsStreamRow
		if err := LOG_DB.ScanRows(rows, &row); err != nil {
			return visited, err
		}
		visited = true
		if err := visit(row); err != nil {
			return visited, err
		}
	}
	return visited, rows.Err()
}

// streamUsageStatisticsRows executes one bounded SQL result stream. The
// caller aggregates summary, series, models and subscriptions in one pass
// instead of rebuilding the logs UNION query four times. If the optional
// archive table is absent or unavailable, it retries against live logs only;
// this preserves the old/raw path for installations that have not completed
// the archive migration and avoids turning an informative page into a 500.
func streamUsageStatisticsRows(ctx context.Context, userID int, startTime, endTime int64, visit func(usageStatisticsStreamRow) error) error {
	archiveStart, archiveEnd := usageStatisticsArchiveWindow(startTime, endTime)
	archiveScanStart, archiveScanEnd := usageStatisticsArchiveScanWindow(startTime, endTime)
	includeArchive := archiveScanStart < archiveScanEnd
	visited, err := streamUsageStatisticsRowsFromSource(ctx, userID, startTime, endTime, archiveScanStart, archiveScanEnd, archiveStart, archiveEnd, includeArchive, visit)
	if err == nil {
		return nil
	}
	// If rows were already delivered, retrying would duplicate their counters.
	// In that case return the original error and let the caller use stale data
	// (if available) instead.
	if !includeArchive || visited {
		return err
	}
	_, rawErr := streamUsageStatisticsRowsFromSource(ctx, userID, startTime, endTime, 0, 0, 0, 0, false, visit)
	if rawErr == nil {
		return nil
	}
	return fmt.Errorf("archive usage statistics query failed: %v; live fallback failed: %w", err, rawErr)
}

func aggregateUsageStatistics(ctx context.Context, userID int, startTime, endTime, bucketSeconds int64) (UsageStatistics, error) {
	acc := newUsageStatisticsAccumulator()
	if err := streamUsageStatisticsRows(ctx, userID, startTime, endTime, func(row usageStatisticsStreamRow) error {
		acc.add(row, bucketSeconds)
		return nil
	}); err != nil {
		return acc.result, fmt.Errorf("query unified usage statistics: %w", err)
	}
	return acc.finish(userID, startTime, endTime, bucketSeconds)
}

// GetUserUsageStatisticsWithContext is the context-aware implementation used
// by HTTP handlers. It intentionally keeps the existing response shape.
func GetUserUsageStatisticsWithContext(ctx context.Context, userID int, startTime, endTime, bucketSeconds int64) (UsageStatistics, error) {
	result := UsageStatistics{Series: []UsageStatisticsPoint{}, Models: []UsageStatisticsModel{}, Subscriptions: []UsageStatisticsSubscription{}}
	if userID <= 0 {
		return result, errors.New("invalid user id")
	}
	if startTime <= 0 || endTime <= startTime || bucketSeconds <= 0 {
		return result, errors.New("invalid usage statistics range")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cacheStart, cacheEnd := normalizeHotUsageWindow(startTime, endTime)
	remoteKey := usageCacheKey("usage-statistics:user", userID, cacheStart, cacheEnd, bucketSeconds, usageMetricTimezoneName(), usageStatisticsCacheFilter(bucketSeconds))
	localKey := usageCacheLocalKey(remoteKey)
	hotTTL := usageStatisticsHistoricalTTL
	if usageStatisticsCacheTimesAreHot(endTime) {
		hotTTL = usageStatisticsCacheTTL
	}
	if cached, found := usageStatisticsHotCache.MustGet(localKey); found {
		return cloneUsageStatistics(cached), nil
	}
	var remote UsageStatistics
	if found, err := usageCacheGet(ctx, remoteKey, &remote); err == nil && found {
		usageStatisticsHotCache.SetWithTTL(localKey, cloneUsageStatistics(remote), hotTTL)
		usageStatisticsStaleCache.SetWithTTL(localKey, cloneUsageStatistics(remote), usageStatisticsStaleTTL)
		return cloneUsageStatistics(remote), nil
	}
	resultCh := usageStatisticsQueryGroup.DoChan(localKey, func() (any, error) {
		workCtx, workCancel := usageProjectionWorkContext(ctx)
		defer workCancel()
		if cached, found := usageStatisticsHotCache.MustGet(localKey); found {
			return cloneUsageStatistics(cached), nil
		}
		var queried UsageStatistics
		var queryErr error
		if projected, ready, projectionErr := readUsageStatisticsFromMetricBuckets(workCtx, userID, startTime, endTime, bucketSeconds); ready && projectionErr == nil {
			queried = projected
		} else {
			// Projection readiness/errors are intentionally non-fatal. During
			// rollout, missing coverage or a failed bucket read falls back to the
			// exact raw/archive stream and keeps the old API response available.
			queried, queryErr = aggregateUsageStatistics(workCtx, userID, startTime, endTime, bucketSeconds)
		}
		if queryErr != nil {
			if stale, found := usageStatisticsStaleCache.MustGet(localKey); found {
				return cloneUsageStatistics(stale), nil
			}
			var remoteStale UsageStatistics
			staleKey := remoteKey + ":stale"
			if found, remoteErr := usageCacheGet(workCtx, staleKey, &remoteStale); remoteErr == nil && found {
				usageStatisticsStaleCache.SetWithTTL(localKey, cloneUsageStatistics(remoteStale), usageStatisticsStaleTTL)
				return remoteStale, nil
			}
			return nil, queryErr
		}
		copyValue := cloneUsageStatistics(queried)
		usageStatisticsHotCache.SetWithTTL(localKey, copyValue, hotTTL)
		usageStatisticsStaleCache.SetWithTTL(localKey, cloneUsageStatistics(queried), usageStatisticsStaleTTL)
		_ = usageCacheSet(workCtx, remoteKey, queried, hotTTL)
		_ = usageCacheSet(workCtx, remoteKey+":stale", queried, usageStatisticsStaleTTL)
		return queried, nil
	})
	value, err := waitUsageProjectionResult(ctx, resultCh)
	if err != nil {
		return result, err
	}
	return cloneUsageStatistics(value.(UsageStatistics)), nil
}

// GetUserUsageStatisticsUnified is retained as an explicit name for internal
// callers that want to document the shared projection path.
func GetUserUsageStatisticsUnified(ctx context.Context, userID int, startTime, endTime, bucketSeconds int64) (UsageStatistics, error) {
	return GetUserUsageStatisticsWithContext(ctx, userID, startTime, endTime, bucketSeconds)
}
