package model

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	// Usage metric workers deliberately leave a small tail open so late writes
	// cannot be mistaken for a sealed bucket.  Readers use the same boundary
	// as the writer and fail closed when a requested bucket is not yet sealed.
	usageMetricProjectionSealDelay = 60 * time.Second
	// A reader and the worker normally share a clock, but a small skew is safer
	// than rejecting an otherwise valid snapshot written just across a second
	// boundary.  Large/future timestamps remain invalid.
	usageMetricProjectionClockSkew = 60 * time.Second
)

// usageMetricFullBucketRange returns exact bucket boundaries covered by a
// query. A rollup is never used for a partial bucket; callers can then fall
// back to raw logs for the hot/edge portion without inventing precision.
func usageMetricFullBucketRange(startTime, endTime int64, granularity UsageMetricGranularity, location *time.Location) ([]int64, int64, bool, error) {
	if startTime <= 0 || endTime <= startTime || location == nil {
		return nil, 0, false, nil
	}
	startBucket, err := UsageMetricBucketStart(startTime, granularity, location)
	if err != nil || startBucket != startTime {
		return nil, 0, false, err
	}
	starts := make([]int64, 0)
	cursor := startTime
	for cursor < endTime {
		next, nextErr := usageMetricReadBucketEnd(cursor, granularity, location)
		if nextErr != nil {
			return nil, 0, false, nextErr
		}
		if next > endTime {
			return nil, 0, false, nil
		}
		starts = append(starts, cursor)
		cursor = next
		if len(starts) > 10000 {
			return nil, 0, false, errors.New("usage metric range is too large")
		}
	}
	if cursor != endTime || len(starts) == 0 {
		return nil, 0, false, nil
	}
	return starts, cursor, true, nil
}

func usageMetricReadBucketEnd(start int64, granularity UsageMetricGranularity, location *time.Location) (int64, error) {
	value := time.Unix(start, 0).In(location)
	switch granularity {
	case UsageMetricGranularityMinute:
		return value.Add(time.Minute).Unix(), nil
	case UsageMetricGranularityHour:
		return value.Add(time.Hour).Unix(), nil
	case UsageMetricGranularityDay:
		next := time.Date(value.Year(), value.Month(), value.Day()+1, 0, 0, 0, 0, location)
		return next.Unix(), nil
	default:
		return 0, fmt.Errorf("unsupported usage metric granularity %q", granularity)
	}
}

// usageMetricRangeParts splits a half-open query range into complete
// projection buckets and exact raw-log tails.  A caller must only use a
// bucket when the whole [bucketStart, bucketEnd) interval is inside the
// requested range; the first and last partial buckets are returned as raw
// ranges so no events outside the user's filter can leak into the result.
//
// The helper deliberately walks local calendar boundaries rather than adding
// a fixed number of seconds.  This keeps hour/day buckets correct across a
// timezone's daylight-saving transition (and makes the same planner usable by
// the statistics and cache-rate readers).
type usageMetricRangePlan struct {
	FullBucketStarts []int64
	RawRanges        [][2]int64
}

func usageMetricRangeParts(startTime, endTime int64, granularity UsageMetricGranularity, location *time.Location) (usageMetricRangePlan, error) {
	parts := usageMetricRangePlan{}
	if startTime <= 0 || endTime <= startTime {
		return parts, nil
	}
	if location == nil {
		return parts, errors.New("usage metric location is required")
	}
	if !validUsageMetricGranularity(granularity) {
		return parts, fmt.Errorf("unsupported usage metric granularity %q", granularity)
	}
	cursor, err := UsageMetricBucketStart(startTime, granularity, location)
	if err != nil {
		return parts, err
	}
	for cursor < endTime {
		next, nextErr := usageMetricReadBucketEnd(cursor, granularity, location)
		if nextErr != nil {
			return parts, nextErr
		}
		if next <= cursor {
			return parts, errors.New("usage metric bucket boundary did not advance")
		}
		overlapStart := startTime
		if cursor > overlapStart {
			overlapStart = cursor
		}
		overlapEnd := endTime
		if next < overlapEnd {
			overlapEnd = next
		}
		if overlapStart < overlapEnd {
			if cursor >= startTime && next <= endTime {
				parts.FullBucketStarts = append(parts.FullBucketStarts, cursor)
			} else {
				parts.RawRanges = append(parts.RawRanges, [2]int64{overlapStart, overlapEnd})
			}
		}
		cursor = next
		if len(parts.FullBucketStarts)+len(parts.RawRanges) > 10000 {
			return parts, errors.New("usage metric range is too large")
		}
	}
	// When the requested range is wholly inside one bucket, the loop above
	// emits one raw range.  Adjacent raw ranges can occur around unusual
	// timezone transitions; coalesce them so callers issue the fewest queries.
	if len(parts.RawRanges) > 1 {
		merged := make([][2]int64, 0, len(parts.RawRanges))
		for _, item := range parts.RawRanges {
			if len(merged) > 0 && merged[len(merged)-1][1] >= item[0] {
				if item[1] > merged[len(merged)-1][1] {
					merged[len(merged)-1][1] = item[1]
				}
				continue
			}
			merged = append(merged, item)
		}
		parts.RawRanges = merged
	}
	return parts, nil
}

// usageMetricCoverageRows reads and validates the durable coverage markers for
// exactly the requested bucket starts.  A count-only check is not sufficient:
// an old algorithm version, a partially written marker, or a marker with a
// stale watermark can otherwise make the projection look ready and silently
// under-count the raw source.
func usageMetricCoverageRows(ctx context.Context, starts []int64, granularity UsageMetricGranularity, timezone string) (map[int64]UsageMetricCoverage, bool, error) {
	if len(starts) == 0 {
		return nil, false, nil
	}
	if LOG_DB == nil {
		return nil, false, errors.New("log database is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// The planner emits unique starts.  Treat duplicates as an invalid plan
	// instead of allowing a count comparison to give a false positive.
	uniqueStarts := make(map[int64]struct{}, len(starts))
	for _, start := range starts {
		if start <= 0 {
			return nil, false, nil
		}
		if _, exists := uniqueStarts[start]; exists {
			return nil, false, nil
		}
		uniqueStarts[start] = struct{}{}
	}
	var rows []UsageMetricCoverage
	err := LOG_DB.WithContext(ctx).Model(&UsageMetricCoverage{}).
		Where("granularity = ? AND bucket_start IN ? AND timezone = ? AND is_complete = ?", granularity, starts, timezone, true).
		Find(&rows).Error
	if err != nil {
		return nil, false, err
	}
	if len(rows) != len(uniqueStarts) {
		return nil, false, nil
	}
	location, loadErr := time.LoadLocation(timezone)
	if loadErr != nil {
		return nil, false, loadErr
	}
	now := time.Now().Unix()
	result := make(map[int64]UsageMetricCoverage, len(rows))
	for _, row := range rows {
		if _, expected := uniqueStarts[row.BucketStart]; !expected {
			return nil, false, nil
		}
		if _, duplicate := result[row.BucketStart]; duplicate {
			return nil, false, nil
		}
		if !usageMetricCoverageRowUsable(row, granularity, timezone, location, now) {
			return nil, false, nil
		}
		result[row.BucketStart] = row
	}
	if len(result) != len(uniqueStarts) {
		return nil, false, nil
	}
	return result, true, nil
}

func usageMetricCoverageComplete(ctx context.Context, starts []int64, granularity UsageMetricGranularity, timezone string) (bool, error) {
	_, ready, err := usageMetricCoverageRows(ctx, starts, granularity, timezone)
	return ready, err
}

func usageMetricCoverageRowUsable(row UsageMetricCoverage, granularity UsageMetricGranularity, timezone string, location *time.Location, now int64) bool {
	if !validUsageMetricGranularity(granularity) || row.Granularity != granularity || row.BucketStart <= 0 {
		return false
	}
	if location == nil || row.Timezone != timezone || !row.IsComplete || row.BucketEnd <= row.BucketStart {
		return false
	}
	expectedEnd, err := usageMetricReadBucketEnd(row.BucketStart, granularity, location)
	if err != nil || expectedEnd != row.BucketEnd {
		return false
	}
	// A complete marker must describe a sealed bucket.  This prevents a
	// current/in-flight bucket from being served as if it were final.
	if row.BucketEnd > now-int64(usageMetricProjectionSealDelay/time.Second) {
		return false
	}
	if row.ComputedAt <= 0 || row.ComputedAt < row.BucketEnd || row.ComputedAt > now+int64(usageMetricProjectionClockSkew/time.Second) {
		return false
	}
	if row.Watermark < 0 || row.SourceVersion != UsageMetricSourceVersionV1 {
		return false
	}
	return true
}

// usageMetricBucketRowsUsable validates rows returned for one user/scope and
// ties their projection metadata back to the coverage markers.  Empty rows
// are valid: coverage explicitly distinguishes an empty sealed bucket from a
// bucket the worker has not processed yet.
func usageMetricBucketRowsUsable(rows []UsageMetricBucket, coverage map[int64]UsageMetricCoverage, granularity UsageMetricGranularity, timezone string, location *time.Location, now int64) bool {
	if location == nil {
		return false
	}
	seen := make(map[string]struct{}, len(rows))
	for _, bucket := range rows {
		marker, ok := coverage[bucket.BucketStart]
		if !ok || bucket.Granularity != granularity || bucket.Timezone != timezone || !bucket.IsComplete {
			return false
		}
		expectedEnd, err := usageMetricReadBucketEnd(bucket.BucketStart, granularity, location)
		if err != nil || bucket.CoverageStart != bucket.BucketStart || bucket.CoverageEnd != expectedEnd ||
			bucket.CoverageEnd != marker.BucketEnd || bucket.SourceVersion != marker.SourceVersion ||
			bucket.ComputedAt != marker.ComputedAt || bucket.Watermark != marker.Watermark ||
			!usageMetricCoverageRowUsable(UsageMetricCoverage{
				Granularity:   bucket.Granularity,
				BucketStart:   bucket.BucketStart,
				BucketEnd:     bucket.CoverageEnd,
				Timezone:      bucket.Timezone,
				ComputedAt:    bucket.ComputedAt,
				Watermark:     bucket.Watermark,
				SourceVersion: bucket.SourceVersion,
				IsComplete:    bucket.IsComplete,
			}, granularity, timezone, location, now) {
			return false
		}
		if bucket.RequestCount > 0 && bucket.Watermark == 0 {
			return false
		}
		if bucket.DimensionHash == "" || bucket.DimensionHash != UsageMetricDimensionHash(bucket) {
			return false
		}
		key := fmt.Sprintf("%d:%s", bucket.BucketStart, bucket.DimensionHash)
		if _, duplicate := seen[key]; duplicate {
			return false
		}
		seen[key] = struct{}{}
	}
	return true
}

func preferredUsageMetricGranularity(startTime, endTime int64) (UsageMetricGranularity, bool) {
	span := endTime - startTime
	if span <= 2*60*60 {
		return "", false
	}
	if span <= 7*24*60*60 {
		return UsageMetricGranularityHour, true
	}
	return UsageMetricGranularityDay, true
}

// readUsageStatisticsFromMetricBuckets reads complete projection buckets and
// exact raw-log tails for the two partial boundary buckets. The bool is false
// when the projection is not ready; callers must then use the precise
// raw/archive path. Projection errors are returned separately so operators
// can observe them without changing the old HTTP contract.
func readUsageStatisticsFromMetricBuckets(ctx context.Context, userID int, startTime, endTime, bucketSeconds int64) (UsageStatistics, bool, error) {
	if !UsageMetricReadEnabled() {
		return UsageStatistics{}, false, nil
	}
	granularity, ok := preferredUsageMetricGranularity(startTime, endTime)
	if !ok {
		return UsageStatistics{}, false, nil
	}
	location, timezone, err := UsageMetricLocation()
	if err != nil {
		return UsageStatistics{}, false, err
	}
	plan, err := usageMetricRangeParts(startTime, endTime, granularity, location)
	if err != nil {
		return UsageStatistics{}, false, err
	}
	if len(plan.FullBucketStarts) == 0 {
		// There is no complete bucket to accelerate. Returning false lets the
		// compatibility path decide whether raw logs or daily archives are the
		// only exact source for this small/edge-only range.
		return UsageStatistics{}, false, nil
	}
	coverage, ready, err := usageMetricCoverageRows(ctx, plan.FullBucketStarts, granularity, timezone)
	if err != nil || !ready {
		return UsageStatistics{}, false, err
	}
	var buckets []UsageMetricBucket
	err = LOG_DB.WithContext(ctx).Where("granularity = ? AND user_id = ? AND bucket_start IN ? AND timezone = ?", granularity, userID, plan.FullBucketStarts, timezone).Find(&buckets).Error
	if err != nil {
		return UsageStatistics{}, false, err
	}
	if !usageMetricBucketRowsUsable(buckets, coverage, granularity, timezone, location, time.Now().Unix()) {
		return UsageStatistics{}, false, nil
	}
	acc := newUsageStatisticsAccumulator()
	for _, bucket := range buckets {
		requestCount := bucket.RequestCount
		if requestCount <= 0 {
			requestCount = bucket.SuccessCount + bucket.ErrorCount
		}
		if requestCount <= 0 {
			continue
		}
		acc.add(usageStatisticsStreamRow{
			CreatedAt:             bucket.BucketStart,
			UserID:                bucket.UserId,
			Type:                  bucket.Type,
			Quota:                 bucket.Quota,
			PreDiscountQuota:      bucket.PreDiscountQuota,
			BillingSource:         bucket.BillingSource,
			PromptTokens:          bucket.PromptTokens,
			CacheTokens:           bucket.CacheTokens,
			CompletionTokens:      bucket.CompletionTokens,
			ModelName:             bucket.ModelName,
			SubscriptionID:        bucket.SubscriptionId,
			RequestCount:          requestCount,
			SuccessCount:          bucket.SuccessCount,
			ErrorCount:            bucket.ErrorCount,
			EffectivePromptTokens: bucket.EffectivePromptTokens,
		}, bucketSeconds)
	}
	for _, rawRange := range plan.RawRanges {
		_, rawErr := streamUsageStatisticsRowsFromSource(ctx, userID, rawRange[0], rawRange[1], 0, 0, 0, 0, false, func(row usageStatisticsStreamRow) error {
			acc.add(row, bucketSeconds)
			return nil
		})
		if rawErr != nil {
			return UsageStatistics{}, false, rawErr
		}
	}
	result, finishErr := acc.finish(userID, startTime, endTime, bucketSeconds)
	if finishErr != nil {
		return UsageStatistics{}, false, finishErr
	}
	return result, true, nil
}

func aggregateUsageStatisticsMetricRows(userID int, startTime, endTime, bucketSeconds int64, buckets []UsageMetricBucket) UsageStatistics {
	acc := newUsageStatisticsAccumulator()
	for _, bucket := range buckets {
		requestCount := bucket.RequestCount
		if requestCount <= 0 {
			requestCount = bucket.SuccessCount + bucket.ErrorCount
		}
		if requestCount <= 0 {
			continue
		}
		row := usageStatisticsStreamRow{
			CreatedAt:             bucket.BucketStart,
			UserID:                bucket.UserId,
			Type:                  bucket.Type,
			Quota:                 bucket.Quota,
			PreDiscountQuota:      bucket.PreDiscountQuota,
			BillingSource:         bucket.BillingSource,
			PromptTokens:          bucket.PromptTokens,
			CacheTokens:           bucket.CacheTokens,
			CompletionTokens:      bucket.CompletionTokens,
			ModelName:             bucket.ModelName,
			SubscriptionID:        bucket.SubscriptionId,
			RequestCount:          requestCount,
			SuccessCount:          bucket.SuccessCount,
			ErrorCount:            bucket.ErrorCount,
			EffectivePromptTokens: bucket.EffectivePromptTokens,
		}
		acc.add(row, bucketSeconds)
	}
	result, _ := acc.finish(userID, startTime, endTime, bucketSeconds)
	return result
}

// readUserCacheRateFromMetricBuckets returns complete projection buckets plus
// exact raw-log tails for partial boundary buckets. The bool is false when no
// complete bucket exists or the projection coverage is incomplete.
func readUserCacheRateFromMetricBuckets(ctx context.Context, userID int64, startTime, endTime int64) (cacheTokens, promptTokens int64, ready bool, err error) {
	if !UsageMetricReadEnabled() || endTime-startTime <= 2*60*60 {
		return 0, 0, false, nil
	}
	location, timezone, err := UsageMetricLocation()
	if err != nil {
		return 0, 0, false, err
	}
	plan, err := usageMetricRangeParts(startTime, endTime, UsageMetricGranularityHour, location)
	if err != nil {
		return 0, 0, false, err
	}
	if len(plan.FullBucketStarts) == 0 {
		return 0, 0, false, nil
	}
	coverage, ready, err := usageMetricCoverageRows(ctx, plan.FullBucketStarts, UsageMetricGranularityHour, timezone)
	if err != nil || !ready {
		return 0, 0, false, err
	}
	// Read the bounded projection rows once and validate each row's metadata
	// against the coverage marker before aggregating.  A scalar SUM with only
	// `is_complete=true` could silently ignore a stale/malformed dimension row.
	var buckets []UsageMetricBucket
	if err = LOG_DB.WithContext(ctx).Where("granularity = ? AND user_id = ? AND bucket_start IN ? AND timezone = ? AND type = ?", UsageMetricGranularityHour, userID, plan.FullBucketStarts, timezone, LogTypeConsume).Find(&buckets).Error; err != nil {
		return 0, 0, false, err
	}
	if !usageMetricBucketRowsUsable(buckets, coverage, UsageMetricGranularityHour, timezone, location, time.Now().Unix()) {
		return 0, 0, false, nil
	}
	for _, bucket := range buckets {
		if !bucket.IsComplete || bucket.PromptTokens <= 0 {
			continue
		}
		cacheTokens += bucket.CacheTokens
		promptTokens += bucket.EffectivePromptTokens
	}
	for _, rawRange := range plan.RawRanges {
		tailCache, tailPrompt, tailErr := queryUserCacheRateFromLogs(ctx, userID, rawRange[0], rawRange[1])
		if tailErr != nil {
			return 0, 0, false, tailErr
		}
		cacheTokens += tailCache
		promptTokens += tailPrompt
	}
	return cacheTokens, promptTokens, true, nil
}

// readDashboardTrafficFromMetricBuckets serves complete historical windows.
// Bucket rows are marked Aggregated so the dashboard preserves the existing
// rule that RPM/concurrency peaks are only exact for detailed hot-window logs.
func readDashboardTrafficFromMetricBuckets(ctx context.Context, userID int, startTime, endTime int64) ([]DashboardTrafficRecord, bool, error) {
	if !UsageMetricReadEnabled() {
		return nil, false, nil
	}
	granularity, ok := preferredUsageMetricGranularity(startTime, endTime)
	if !ok {
		return nil, false, nil
	}
	location, timezone, err := UsageMetricLocation()
	if err != nil {
		return nil, false, err
	}
	starts, _, full, err := usageMetricFullBucketRange(startTime, endTime, granularity, location)
	if err != nil || !full {
		return nil, false, err
	}
	coverage, ready, err := usageMetricCoverageRows(ctx, starts, granularity, timezone)
	if err != nil || !ready {
		return nil, false, err
	}
	query := LOG_DB.WithContext(ctx).Model(&UsageMetricBucket{}).
		Where("granularity = ? AND bucket_start IN ? AND timezone = ? AND type = ?", granularity, starts, timezone, LogTypeConsume)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	var buckets []UsageMetricBucket
	if err := query.Order("bucket_start ASC, channel_id ASC, dimension_hash ASC").Find(&buckets).Error; err != nil {
		return nil, false, err
	}
	if !usageMetricBucketRowsUsable(buckets, coverage, granularity, timezone, location, time.Now().Unix()) {
		return nil, false, nil
	}
	records := make([]DashboardTrafficRecord, 0, len(buckets))
	for _, bucket := range buckets {
		requestCount := bucket.RequestCount
		if requestCount <= 0 {
			requestCount = bucket.SuccessCount
		}
		if requestCount <= 0 {
			continue
		}
		records = append(records, DashboardTrafficRecord{
			UserId:       bucket.UserId,
			ChannelId:    bucket.ChannelId,
			CreatedAt:    bucket.BucketStart,
			UseTime:      safeInt64ToInt(bucket.UseTime / requestCount),
			Quota:        safeInt64ToInt(bucket.Quota),
			Cost:         safeInt64ToInt(bucket.Cost),
			RequestCount: requestCount,
			Aggregated:   true,
		})
	}
	return records, true, nil
}

func safeInt64ToInt(value int64) int {
	maxInt := int64(^uint(0) >> 1)
	minInt := -maxInt - 1
	if value > maxInt {
		return int(maxInt)
	}
	if value < minInt {
		return int(minInt)
	}
	return int(value)
}
