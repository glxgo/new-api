package model

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/samber/hot"
	"golang.org/x/sync/singleflight"
)

// TokenUsageStats is the settled billing usage visible on the API-key page.
// The lifetime value is intentionally reconstructed from both live detail
// rows and archived daily aggregates so it remains stable after retention.
type TokenUsageStats struct {
	TodayUsedQuota    int64
	LifetimeUsedQuota int64
	// RangeUsedQuota is kept for internal range-query callers only. It is not
	// copied to model.Token, so the legacy API-key JSON contract remains
	// unchanged.
	RangeUsedQuota int64 `json:"-"`
	Stale          bool  `json:"-"`
}

// chinaDayRangeForTokenUsage returns the current Asia/Shanghai calendar day.
// This is deliberately independent from time.Local so the displayed "today"
// boundary is stable across hosts and containers.
func chinaDayRangeForTokenUsage(now time.Time) (int64, int64) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	localNow := now.In(location)
	start := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	return start.Unix(), now.Unix()
}

type tokenUsageQuotaRow struct {
	TokenId       int   `gorm:"column:token_id"`
	Quota         int64 `gorm:"column:quota"`
	TodayQuota    int64 `gorm:"column:today_quota"`
	LifetimeQuota int64 `gorm:"column:lifetime_quota"`
}

const (
	tokenUsageStatsCacheTTL       = 30 * time.Second
	tokenUsageStatsStaleTTL       = 5 * time.Minute
	tokenUsageStatsProjectionName = "token-usage-stats-v2"
)

var (
	tokenUsageStatsHotCache = hot.NewHotCache[string, map[int]TokenUsageStats](hot.LRU, 256).
				WithTTL(tokenUsageStatsCacheTTL).
				WithJanitor().Build()
	tokenUsageStatsStaleCache = hot.NewHotCache[string, map[int]TokenUsageStats](hot.LRU, 256).
					WithTTL(tokenUsageStatsStaleTTL).
					WithJanitor().Build()
	tokenUsageStatsQueryGroup singleflight.Group

	tokenUsageRangeCache = hot.NewHotCache[string, map[int]TokenUsageStats](hot.LRU, 256).
				WithTTL(30 * time.Second).
				WithJanitor().Build()
	tokenUsageRangeStaleCache = hot.NewHotCache[string, map[int]TokenUsageStats](hot.LRU, 256).
					WithTTL(5 * time.Minute).
					WithJanitor().Build()
	tokenUsageRangeQueryGroup singleflight.Group
)

// GetTokenUsageStats returns today's and lifetime settled consumption for a
// bounded set of token IDs. It performs two grouped queries for live detail
// rows and one grouped query for archived daily aggregates, avoiding an N+1
// query when the API-key list is rendered. Results use a short local/Redis
// cache and a longer stale cache so a transient log-database failure does not
// block the API-key page or turn an error into a fabricated zero value.
func GetTokenUsageStats(tokenIDs []int, now time.Time) (map[int]TokenUsageStats, error) {
	return GetTokenUsageStatsWithContext(context.Background(), tokenIDs, now)
}

// GetTokenUsageStatsWithContext is the context-aware implementation used by
// request handlers and tests. The legacy GetTokenUsageStats signature remains
// unchanged for callers that do not have a request context.
func GetTokenUsageStatsWithContext(ctx context.Context, tokenIDs []int, now time.Time) (map[int]TokenUsageStats, error) {
	result := makeTokenUsageStatsMap(tokenIDs)
	if len(result) == 0 {
		return result, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	localKey, remoteKey := tokenUsageStatsCacheKeys(tokenIDs, now)
	if cached, found := tokenUsageStatsHotCache.MustGet(localKey); found {
		if normalized, valid := normalizeTokenUsageStatsCacheValue(tokenIDs, cached); valid {
			return normalized, nil
		}
		tokenUsageStatsHotCache.Delete(localKey)
	}
	var remote map[int]TokenUsageStats
	if found, remoteErr := usageCacheGet(ctx, remoteKey, &remote); remoteErr == nil && found {
		if normalized, valid := normalizeTokenUsageStatsCacheValue(tokenIDs, remote); valid {
			for id, stats := range normalized {
				stats.Stale = false
				normalized[id] = stats
			}
			tokenUsageStatsHotCache.SetWithTTL(localKey, cloneTokenUsageStats(normalized), tokenUsageStatsCacheTTL)
			tokenUsageStatsStaleCache.SetWithTTL(localKey, cloneTokenUsageStats(normalized), tokenUsageStatsStaleTTL)
			return normalized, nil
		}
	}

	resultCh := tokenUsageStatsQueryGroup.DoChan(localKey, func() (any, error) {
		// A shared key-page query must survive a single waiter's cancellation,
		// but should not occupy the database for the generic two-minute budget.
		workCtx, workCancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer workCancel()
		if cached, found := tokenUsageStatsHotCache.MustGet(localKey); found {
			if normalized, valid := normalizeTokenUsageStatsCacheValue(tokenIDs, cached); valid {
				return normalized, nil
			}
			tokenUsageStatsHotCache.Delete(localKey)
		}
		// A second Redis check inside singleflight closes the small race where
		// another process populated the shared cache after the first miss.
		var shared map[int]TokenUsageStats
		if found, remoteErr := usageCacheGet(workCtx, remoteKey, &shared); remoteErr == nil && found {
			if normalized, valid := normalizeTokenUsageStatsCacheValue(tokenIDs, shared); valid {
				for id, stats := range normalized {
					stats.Stale = false
					normalized[id] = stats
				}
				tokenUsageStatsHotCache.SetWithTTL(localKey, cloneTokenUsageStats(normalized), tokenUsageStatsCacheTTL)
				tokenUsageStatsStaleCache.SetWithTTL(localKey, cloneTokenUsageStats(normalized), tokenUsageStatsStaleTTL)
				return normalized, nil
			}
		}

		queried, err := queryTokenUsageStats(workCtx, tokenIDs, now)
		if err != nil {
			if stale, found := tokenUsageStatsStaleCache.MustGet(localKey); found {
				if normalized, valid := normalizeTokenUsageStatsCacheValue(tokenIDs, stale); valid {
					for id, stats := range normalized {
						stats.Stale = true
						normalized[id] = stats
					}
					return normalized, nil
				}
			}
			var sharedStale map[int]TokenUsageStats
			if found, remoteErr := usageCacheGet(workCtx, remoteKey+":stale", &sharedStale); remoteErr == nil && found {
				if normalized, valid := normalizeTokenUsageStatsCacheValue(tokenIDs, sharedStale); valid {
					for id, stats := range normalized {
						stats.Stale = true
						normalized[id] = stats
					}
					return normalized, nil
				}
			}
			return nil, err
		}

		for id, stats := range queried {
			stats.Stale = false
			queried[id] = stats
		}
		fresh := cloneTokenUsageStats(queried)
		tokenUsageStatsHotCache.SetWithTTL(localKey, fresh, tokenUsageStatsCacheTTL)
		tokenUsageStatsStaleCache.SetWithTTL(localKey, cloneTokenUsageStats(queried), tokenUsageStatsStaleTTL)
		// Redis is an acceleration/fallback layer. A Redis write failure must
		// never turn a successful database query into an API error.
		_ = usageCacheSet(workCtx, remoteKey, queried, tokenUsageStatsCacheTTL)
		_ = usageCacheSet(workCtx, remoteKey+":stale", queried, tokenUsageStatsStaleTTL)
		return queried, nil
	})
	value, queryErr := waitUsageProjectionResult(ctx, resultCh)
	if queryErr != nil {
		return nil, queryErr
	}
	if cached, ok := value.(map[int]TokenUsageStats); ok {
		if normalized, valid := normalizeTokenUsageStatsCacheValue(tokenIDs, cached); valid {
			return normalized, nil
		}
		return nil, fmt.Errorf("invalid token usage cache value")
	}
	return nil, fmt.Errorf("invalid token usage cache value")
}

func tokenUsageStatsCacheKeys(tokenIDs []int, now time.Time) (localKey, remoteKey string) {
	ids := make([]int, 0, len(tokenIDs))
	seen := make(map[int]struct{}, len(tokenIDs))
	for _, tokenID := range tokenIDs {
		if tokenID <= 0 {
			continue
		}
		if _, exists := seen[tokenID]; exists {
			continue
		}
		seen[tokenID] = struct{}{}
		ids = append(ids, tokenID)
	}
	sort.Ints(ids)
	parts := make([]string, 0, len(ids))
	for _, tokenID := range ids {
		parts = append(parts, fmt.Sprint(tokenID))
	}
	start, end := chinaDayRangeForTokenUsage(now)
	// Keep the moving upper bound exact.  The SQL query uses the caller's
	// unquantized `now`; quantizing only the cache key could return a snapshot
	// that silently omitted the last few seconds of settled usage.  Requests in
	// the same Unix second still share a key, while correctness remains tied to
	// the half-open [dayStart, now) interval.
	filter := fmt.Sprintf("projection=%s;ids=%s", tokenUsageStatsProjectionName, strings.Join(parts, ","))
	remoteKey = usageCacheKey("token-usage-stats", 0, start, end, 0, "Asia/Shanghai", filter)
	localKey = usageCacheLocalKey(remoteKey)
	return localKey, remoteKey
}

func normalizeTokenUsageStatsCacheValue(tokenIDs []int, source map[int]TokenUsageStats) (map[int]TokenUsageStats, bool) {
	if source == nil {
		return nil, false
	}
	result := makeTokenUsageStatsMap(tokenIDs)
	for tokenID := range result {
		stats, found := source[tokenID]
		if !found {
			return nil, false
		}
		result[tokenID] = stats
	}
	return result, true
}

// readTokenTodayUsageFromMetricBuckets returns an exact Asia/Shanghai
// calendar-day projection when every complete hour in the requested window
// has a sealed, v1 coverage marker.  The current partial hour remains a raw
// tail, so the result never invents precision at the moving day boundary.
//
// A missing/invalid projection is deliberately reported as ready=false.  The
// API-key contract must then fall back to the established raw-log query rather
// than exposing a new error or silently returning a partial bucket sum.
func readTokenTodayUsageFromMetricBuckets(ctx context.Context, tokenIDs []int, startTime, endTime int64) (map[int]int64, bool, error) {
	result := make(map[int]int64, len(tokenIDs))
	for _, tokenID := range tokenIDs {
		if tokenID > 0 {
			result[tokenID] = 0
		}
	}
	if len(result) == 0 || !UsageMetricReadEnabled() || LOG_DB == nil {
		return result, false, nil
	}
	// API-key “today” is a fixed product contract.  Do not consume a bucket
	// produced in another timezone, even if its UTC interval happens to look
	// compatible.
	location, timezone, err := UsageMetricLocation()
	if err != nil {
		return result, false, err
	}
	if timezone != "Asia/Shanghai" {
		return result, false, nil
	}
	plan, err := usageMetricRangeParts(startTime, endTime, UsageMetricGranularityHour, location)
	if err != nil {
		return result, false, err
	}
	if len(plan.FullBucketStarts) == 0 {
		return result, false, nil
	}
	coverage, ready, err := usageMetricCoverageRows(ctx, plan.FullBucketStarts, UsageMetricGranularityHour, timezone)
	if err != nil || !ready {
		return result, false, err
	}
	var buckets []UsageMetricBucket
	if err := LOG_DB.WithContext(ctx).
		Where("granularity = ? AND bucket_start IN ? AND timezone = ? AND type = ? AND settled = ? AND token_id IN ?",
			UsageMetricGranularityHour, plan.FullBucketStarts, timezone, LogTypeConsume, true, keysFromTokenUsageMap(result)).
		Find(&buckets).Error; err != nil {
		return result, false, err
	}
	if !usageMetricBucketRowsUsable(buckets, coverage, UsageMetricGranularityHour, timezone, location, time.Now().Unix()) {
		return result, false, nil
	}
	for _, bucket := range buckets {
		if _, requested := result[bucket.TokenId]; requested {
			result[bucket.TokenId] += bucket.Quota
		}
	}

	// Add only the exact partial-hour tails.  Complete hours are represented by
	// the projection above, so this cannot double-count a log at a boundary.
	if len(plan.RawRanges) > 0 {
		conditions := make([]string, 0, len(plan.RawRanges))
		args := make([]interface{}, 0, len(plan.RawRanges)*2)
		for _, rawRange := range plan.RawRanges {
			conditions = append(conditions, "(created_at >= ? AND created_at < ?)")
			args = append(args, rawRange[0], rawRange[1])
		}
		var rawRows []tokenUsageQuotaRow
		whereArgs := []interface{}{keysFromTokenUsageMap(result), LogTypeConsume, true}
		whereArgs = append(whereArgs, args...)
		rawQuery := LOG_DB.WithContext(ctx).Model(&Log{}).
			Select("token_id, COALESCE(SUM(quota), 0) AS quota").
			Where("token_id IN ? AND type = ? AND settled = ? AND ("+strings.Join(conditions, " OR ")+")",
				whereArgs...)
		if err := rawQuery.Group("token_id").Find(&rawRows).Error; err != nil {
			return result, false, err
		}
		for _, row := range rawRows {
			if _, requested := result[row.TokenId]; requested {
				result[row.TokenId] += row.Quota
			}
		}
	}
	return result, true, nil
}

func keysFromTokenUsageMap(values map[int]int64) []int {
	keys := make([]int, 0, len(values))
	for tokenID := range values {
		keys = append(keys, tokenID)
	}
	sort.Ints(keys)
	return keys
}

func queryTokenUsageStats(ctx context.Context, tokenIDs []int, now time.Time) (map[int]TokenUsageStats, error) {
	result := makeTokenUsageStatsMap(tokenIDs)
	if len(result) == 0 {
		return result, nil
	}
	if LOG_DB == nil {
		return nil, fmt.Errorf("log database is unavailable")
	}
	ids := make([]int, 0, len(result))
	for tokenID := range result {
		ids = append(ids, tokenID)
	}
	todayStart, todayEnd := chinaDayRangeForTokenUsage(now)
	metricToday, metricReady, _ := readTokenTodayUsageFromMetricBuckets(ctx, tokenIDs, todayStart, todayEnd)

	// Today and lifetime are derived in one grouped pass over live detail rows
	// on the compatibility path.  When a complete metric projection is ready,
	// the raw scan can stop at today's Asia/Shanghai boundary: today's quota is
	// already covered by sealed hour buckets plus the exact hot tail.  This is
	// the only lifetime optimization enabled here; older archive precedence is
	// intentionally left unchanged until it has an explicit non-overlap proof.
	var liveRows []tokenUsageQuotaRow
	liveQuery := LOG_DB.WithContext(ctx).Model(&Log{})
	if metricReady {
		liveQuery = liveQuery.Select("token_id, COALESCE(SUM(quota), 0) AS lifetime_quota").
			Where("token_id IN ? AND type = ? AND settled = ? AND (created_at < ? OR created_at >= ?)", ids, LogTypeConsume, true, todayStart, todayEnd)
	} else {
		liveQuery = liveQuery.Select("token_id, COALESCE(SUM(CASE WHEN created_at >= ? AND created_at < ? THEN quota ELSE 0 END), 0) AS today_quota, COALESCE(SUM(quota), 0) AS lifetime_quota", todayStart, todayEnd).
			Where("token_id IN ? AND type = ? AND settled = ?", ids, LogTypeConsume, true)
	}
	if err := liveQuery.Group("token_id").Find(&liveRows).Error; err != nil {
		return nil, fmt.Errorf("query token detail usage: %w", err)
	}
	for _, row := range liveRows {
		stats := result[row.TokenId]
		stats.TodayUsedQuota = row.TodayQuota
		if metricReady {
			stats.TodayUsedQuota = metricToday[row.TokenId]
		}
		stats.LifetimeUsedQuota = row.LifetimeQuota
		result[row.TokenId] = stats
	}
	// A token with no live rows is still present in the requested map.  Apply a
	// ready projection value to that case as well (including an explicit zero).
	if metricReady {
		for tokenID, todayQuota := range metricToday {
			stats := result[tokenID]
			stats.TodayUsedQuota = todayQuota
			// The compatibility lifetime projection below intentionally scans
			// only rows before today's boundary when the bucket read is ready.
			// Include the projected today value here so lifetime semantics remain
			// identical to the old all-live-row query.
			stats.LifetimeUsedQuota += todayQuota
			result[tokenID] = stats
		}
	}
	var archivedRows []tokenUsageQuotaRow
	if err := LOG_DB.WithContext(ctx).Model(&UsageLogDailyAggregate{}).
		Select("token_id, COALESCE(SUM(quota), 0) AS quota").
		Where("token_id IN ? AND type = ?", ids, LogTypeConsume).
		Group("token_id").Find(&archivedRows).Error; err != nil {
		return nil, fmt.Errorf("query token archived usage: %w", err)
	}
	for _, row := range archivedRows {
		stats := result[row.TokenId]
		stats.LifetimeUsedQuota += row.Quota
		result[row.TokenId] = stats
	}
	return result, nil
}

func makeTokenUsageStatsMap(tokenIDs []int) map[int]TokenUsageStats {
	result := make(map[int]TokenUsageStats, len(tokenIDs))
	for _, tokenID := range tokenIDs {
		if tokenID > 0 {
			result[tokenID] = TokenUsageStats{}
		}
	}
	return result
}

// GetTokenUsageStatsInRange returns settled consume usage for the requested
// half-open time range. The detailed-log page is the source of truth for this
// view, so only live detail rows are included. Archived daily aggregates are
// intentionally excluded: a daily bucket cannot answer an arbitrary two-hour
// or partial-day range without over-counting. Lifetime projections continue to
// use archived aggregates through GetTokenUsageStats.
func GetTokenUsageStatsInRange(ctx context.Context, tokenIDs []int, startTimestamp, endTimestamp int64) (map[int]TokenUsageStats, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	idsForKey := append([]int(nil), tokenIDs...)
	sort.Ints(idsForKey)
	parts := make([]string, 0, len(idsForKey))
	for _, id := range idsForKey {
		parts = append(parts, fmt.Sprint(id))
	}
	cacheKey := usageCacheLocalKey(fmt.Sprintf("token-usage-range|%d|%d|%s", startTimestamp, endTimestamp, strings.Join(parts, ",")))
	if cached, found := tokenUsageRangeCache.MustGet(cacheKey); found {
		return cloneTokenUsageStats(cached), nil
	}
	resultCh := tokenUsageRangeQueryGroup.DoChan(cacheKey, func() (any, error) {
		workCtx, workCancel := usageProjectionWorkContext(ctx)
		defer workCancel()
		if cached, found := tokenUsageRangeCache.MustGet(cacheKey); found {
			return cloneTokenUsageStats(cached), nil
		}
		queried, queryErr := queryTokenUsageStatsInRange(workCtx, tokenIDs, startTimestamp, endTimestamp)
		if queryErr != nil {
			if stale, found := tokenUsageRangeStaleCache.MustGet(cacheKey); found {
				staleCopy := cloneTokenUsageStats(stale)
				for id, stats := range staleCopy {
					stats.Stale = true
					staleCopy[id] = stats
				}
				return staleCopy, nil
			}
			return nil, queryErr
		}
		for id, stats := range queried {
			stats.Stale = false
			queried[id] = stats
		}
		tokenUsageRangeCache.SetWithTTL(cacheKey, cloneTokenUsageStats(queried), 30*time.Second)
		tokenUsageRangeStaleCache.SetWithTTL(cacheKey, cloneTokenUsageStats(queried), 5*time.Minute)
		return cloneTokenUsageStats(queried), nil
	})
	value, err := waitUsageProjectionResult(ctx, resultCh)
	if err != nil {
		return nil, err
	}
	stats, ok := value.(map[int]TokenUsageStats)
	if !ok {
		return nil, fmt.Errorf("invalid token usage range cache value")
	}
	return stats, nil
}

func cloneTokenUsageStats(source map[int]TokenUsageStats) map[int]TokenUsageStats {
	if source == nil {
		return nil
	}
	copyValue := make(map[int]TokenUsageStats, len(source))
	for tokenID, stats := range source {
		copyValue[tokenID] = stats
	}
	return copyValue
}

func queryTokenUsageStatsInRange(ctx context.Context, tokenIDs []int, startTimestamp, endTimestamp int64) (map[int]TokenUsageStats, error) {
	result := makeTokenUsageStatsMap(tokenIDs)
	if len(tokenIDs) == 0 {
		return result, nil
	}
	if LOG_DB == nil {
		return nil, fmt.Errorf("log database is unavailable")
	}
	for _, tokenID := range tokenIDs {
		if tokenID > 0 {
			result[tokenID] = TokenUsageStats{}
		}
	}
	if len(result) == 0 {
		return result, nil
	}
	ids := make([]int, 0, len(result))
	for tokenID := range result {
		ids = append(ids, tokenID)
	}
	var liveRows []tokenUsageQuotaRow
	if err := LOG_DB.WithContext(ctx).Model(&Log{}).
		Select("token_id, COALESCE(SUM(quota), 0) AS quota").
		Where("token_id IN ? AND type = ? AND settled = ? AND created_at >= ? AND created_at < ?", ids, LogTypeConsume, true, startTimestamp, endTimestamp).
		Group("token_id").Find(&liveRows).Error; err != nil {
		return nil, fmt.Errorf("query token lifetime detail usage: %w", err)
	}
	for _, row := range liveRows {
		stats := result[row.TokenId]
		stats.TodayUsedQuota = row.Quota
		stats.RangeUsedQuota = row.Quota
		result[row.TokenId] = stats
	}

	return result, nil
}

// AttachTokenUsageStats enriches API-key response objects in one grouped read.
func AttachTokenUsageStats(tokens []*Token, now time.Time) error {
	if len(tokens) == 0 {
		return nil
	}
	ids := make([]int, 0, len(tokens))
	for _, token := range tokens {
		if token != nil {
			ids = append(ids, token.Id)
		}
	}
	stats, err := GetTokenUsageStats(ids, now)
	if err != nil {
		return err
	}
	for _, token := range tokens {
		if token == nil {
			continue
		}
		usage := stats[token.Id]
		token.TodayUsedQuota = usage.TodayUsedQuota
		token.LifetimeUsedQuota = usage.LifetimeUsedQuota
	}
	return nil
}
