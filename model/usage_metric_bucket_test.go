package model

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openUsageMetricBucketTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:usage-metric-bucket-%s-%d?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&UsageMetricBucket{}))
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func mustUsageMetricLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	location, err := time.LoadLocation(name)
	require.NoError(t, err)
	return location
}

func TestUsageMetricBucketStartUsesExplicitTimezoneAndGranularity(t *testing.T) {
	location := mustUsageMetricLocation(t, "Asia/Shanghai")
	utc := time.Date(2026, time.January, 1, 16, 37, 42, 0, time.UTC)
	hour, err := UsageMetricBucketStart(utc.Unix(), UsageMetricGranularityHour, location)
	require.NoError(t, err)
	day, err := UsageMetricBucketStart(utc.Unix(), UsageMetricGranularityDay, location)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, time.January, 2, 0, 0, 0, 0, location).Unix(), hour)
	require.Equal(t, hour, day)

	minute, err := UsageMetricBucketStart(utc.Unix(), UsageMetricGranularityMinute, location)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, time.January, 2, 0, 37, 0, 0, location).Unix(), minute)
	_, err = UsageMetricBucketStart(utc.Unix(), UsageMetricGranularity("week"), location)
	require.ErrorContains(t, err, "unsupported")
	_, err = UsageMetricBucketStart(0, UsageMetricGranularityHour, location)
	require.Error(t, err)
	_, err = UsageMetricBucketStart(utc.Unix(), UsageMetricGranularityHour, nil)
	require.Error(t, err)
}

func TestUsageMetricRangePartsSplitsPartialBoundaryBuckets(t *testing.T) {
	location := time.UTC
	base := time.Date(2026, time.January, 2, 0, 0, 0, 0, location).Unix()
	parts, err := usageMetricRangeParts(base+15, base+3*3600+30, UsageMetricGranularityHour, location)
	require.NoError(t, err)
	require.Equal(t, []int64{base + 3600, base + 2*3600}, parts.FullBucketStarts)
	require.Equal(t, [][2]int64{{base + 15, base + 3600}, {base + 3*3600, base + 3*3600 + 30}}, parts.RawRanges)

	// A range contained in one bucket has no projection portion and must stay
	// on the exact raw path.
	parts, err = usageMetricRangeParts(base+120, base+600, UsageMetricGranularityHour, location)
	require.NoError(t, err)
	require.Empty(t, parts.FullBucketStarts)
	require.Equal(t, [][2]int64{{base + 120, base + 600}}, parts.RawRanges)
}

func TestBuildUsageMetricBucketsAggregatesDimensionsCountersAndSettledState(t *testing.T) {
	location := mustUsageMetricLocation(t, "Asia/Shanghai")
	base := time.Date(2026, time.August, 25, 10, 0, 0, 0, location).Unix()
	logs := []Log{
		{
			Id: 11, UserId: 7, CreatedAt: base + 5, Type: LogTypeConsume, Settled: true,
			ModelName: "gpt-test", ChannelId: 3, TokenId: 9, Group: "vip", BillingSource: "wallet",
			Quota: 100, PreDiscountQuota: 150, PromptTokens: 10, CacheTokens: 20, CompletionTokens: 4,
			UseTime: 8, Cost: 6, PaidQuota: 90, PaidGiftQuota: 10, IsStream: true,
		},
		{
			Id: 12, UserId: 7, CreatedAt: base + 55, Type: LogTypeConsume, Settled: true,
			ModelName: "gpt-test", ChannelId: 3, TokenId: 9, Group: "vip", BillingSource: "wallet",
			Quota: 50, PromptTokens: 30, CacheTokens: 5, CompletionTokens: 6,
		},
		{
			Id: 13, UserId: 7, CreatedAt: base + 65, Type: LogTypeConsume, Settled: false,
			ModelName: "gpt-test", ChannelId: 3, TokenId: 9, Group: "vip", BillingSource: "wallet",
			Quota: 999,
		},
		{
			Id: 14, UserId: 7, CreatedAt: base + 70, Type: LogTypeError, Settled: false,
			ModelName: "gpt-test", ChannelId: 3, TokenId: 9, Group: "vip", BillingSource: "wallet",
		},
		{
			Id: 15, UserId: 7, CreatedAt: base + 3600, Type: LogTypeConsume, Settled: true,
			ModelName: "gpt-test", ChannelId: 3, TokenId: 9, Group: "vip", BillingSource: "wallet",
			Quota: 25,
		},
		// Financial/audit rows are not usage metric inputs.
		{Id: 99, UserId: 7, CreatedAt: base + 80, Type: LogTypeTopup, Settled: true, Quota: 10000},
	}

	buckets, err := BuildUsageMetricBuckets(logs, UsageMetricGranularityHour, location, UsageMetricSourceVersionV1, 1_700_000_000, 0)
	require.NoError(t, err)
	require.Len(t, buckets, 4)
	require.EqualValues(t, 15, buckets[0].Watermark)
	require.Equal(t, UsageMetricSourceVersionV1, buckets[0].SourceVersion)

	var settledConsume, pendingConsume, relayError, nextHour UsageMetricBucket
	for _, bucket := range buckets {
		switch {
		case bucket.Type == LogTypeConsume && bucket.Settled && bucket.BucketStart == base:
			settledConsume = bucket
		case bucket.Type == LogTypeConsume && !bucket.Settled && bucket.BucketStart == base:
			pendingConsume = bucket
		case bucket.Type == LogTypeError:
			relayError = bucket
		case bucket.BucketStart == base+3600:
			nextHour = bucket
		}
	}
	require.EqualValues(t, 2, settledConsume.RequestCount)
	require.EqualValues(t, 2, settledConsume.SuccessCount)
	require.EqualValues(t, 150, settledConsume.Quota)
	require.EqualValues(t, 200, settledConsume.PreDiscountQuota)
	require.EqualValues(t, 40, settledConsume.PromptTokens)
	require.EqualValues(t, 25, settledConsume.CacheTokens)
	// 10 prompt + 20 cache (cache > prompt), then 30 prompt (cache <= prompt).
	require.EqualValues(t, 60, settledConsume.EffectivePromptTokens)
	require.EqualValues(t, 10, settledConsume.CompletionTokens)
	require.EqualValues(t, 1, settledConsume.StreamCount)
	require.EqualValues(t, 8, settledConsume.UseTime)
	require.EqualValues(t, 6, settledConsume.Cost)
	require.EqualValues(t, 90, settledConsume.PaidQuota)
	require.EqualValues(t, 10, settledConsume.PaidGiftQuota)
	require.EqualValues(t, base+5, settledConsume.FirstLogAt)
	require.EqualValues(t, base+55, settledConsume.LastLogAt)
	require.NotEmpty(t, settledConsume.DimensionHash)
	require.Equal(t, UsageMetricDimensionHash(settledConsume), settledConsume.DimensionHash)

	require.EqualValues(t, 1, pendingConsume.RequestCount)
	require.EqualValues(t, 999, pendingConsume.Quota)
	require.EqualValues(t, 1, relayError.RequestCount)
	require.EqualValues(t, 1, relayError.ErrorCount)
	require.EqualValues(t, 1, nextHour.RequestCount)

	reversed := append([]Log(nil), logs...)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	bucketsReversed, err := BuildUsageMetricBuckets(reversed, UsageMetricGranularityHour, location, UsageMetricSourceVersionV1, 1_700_000_000, 0)
	require.NoError(t, err)
	require.True(t, reflect.DeepEqual(buckets, bucketsReversed), "bucket ordering/content must be deterministic")
}

func TestBuildUsageMetricBucketsDayBoundaryAndValidation(t *testing.T) {
	location := mustUsageMetricLocation(t, "Asia/Shanghai")
	first := time.Date(2026, time.January, 1, 23, 59, 0, 0, location).Unix()
	second := time.Date(2026, time.January, 2, 0, 1, 0, 0, location).Unix()
	logs := []Log{
		{Id: 1, UserId: 1, CreatedAt: first, Type: LogTypeConsume, Settled: true, Quota: 1},
		{Id: 2, UserId: 1, CreatedAt: second, Type: LogTypeConsume, Settled: true, Quota: 2},
	}
	buckets, err := BuildUsageMetricBuckets(logs, UsageMetricGranularityDay, location, "algorithm-42", 1_700_000_001, 2)
	require.NoError(t, err)
	require.Len(t, buckets, 2)
	require.NotEqual(t, buckets[0].BucketStart, buckets[1].BucketStart)
	require.EqualValues(t, 2, buckets[0].Watermark)

	_, err = BuildUsageMetricBuckets(logs, UsageMetricGranularityDay, location, "algorithm-42", 1, 1)
	require.ErrorContains(t, err, "watermark")
	_, err = BuildUsageMetricBuckets(nil, UsageMetricGranularityDay, location, "", 1, 0)
	require.ErrorContains(t, err, "source version")
	_, err = BuildUsageMetricBuckets(nil, UsageMetricGranularityDay, location, "v1", 0, 0)
	require.ErrorContains(t, err, "computed_at")
	empty, err := BuildUsageMetricBuckets(nil, UsageMetricGranularityDay, location, "v1", 1, 0)
	require.NoError(t, err)
	require.Empty(t, empty)
}

func TestBuildUsageMetricBucketsDayCoverageEndFollowsDSTCalendarBoundary(t *testing.T) {
	location := mustUsageMetricLocation(t, "America/New_York")
	// The 2026 spring-forward day is 23 elapsed hours, not 24.  CoverageEnd
	// must follow the next local midnight so readers and writers agree.
	createdAt := time.Date(2026, time.March, 8, 12, 0, 0, 0, location).Unix()
	buckets, err := BuildUsageMetricBuckets([]Log{
		{Id: 1, UserId: 1, CreatedAt: createdAt, Type: LogTypeConsume, Settled: true, Quota: 1},
	}, UsageMetricGranularityDay, location, UsageMetricSourceVersionV1, createdAt+3600, 1)
	require.NoError(t, err)
	require.Len(t, buckets, 1)
	expectedStart := time.Date(2026, time.March, 8, 0, 0, 0, 0, location).Unix()
	expectedEnd := time.Date(2026, time.March, 9, 0, 0, 0, 0, location).Unix()
	require.EqualValues(t, expectedStart, buckets[0].CoverageStart)
	require.EqualValues(t, expectedEnd, buckets[0].CoverageEnd)
	require.EqualValues(t, 23*60*60, buckets[0].CoverageEnd-buckets[0].CoverageStart)
}

func TestUsageMetricDimensionHashIncludesAllDimensions(t *testing.T) {
	base := UsageMetricBucket{UserId: 1, Type: LogTypeConsume, Settled: true, ModelName: "m", ChannelId: 2, TokenId: 3, GroupName: "g", BillingSource: "wallet", SubscriptionId: 4}
	base.RefreshDimensionHash()
	require.Len(t, base.DimensionHash, 64)
	require.Equal(t, base.DimensionHash, UsageMetricDimensionHash(base))
	for name, mutate := range map[string]func(*UsageMetricBucket){
		"user":         func(value *UsageMetricBucket) { value.UserId++ },
		"type":         func(value *UsageMetricBucket) { value.Type = LogTypeError },
		"settled":      func(value *UsageMetricBucket) { value.Settled = false },
		"model":        func(value *UsageMetricBucket) { value.ModelName = "other" },
		"channel":      func(value *UsageMetricBucket) { value.ChannelId++ },
		"token":        func(value *UsageMetricBucket) { value.TokenId++ },
		"group":        func(value *UsageMetricBucket) { value.GroupName = "other" },
		"billing":      func(value *UsageMetricBucket) { value.BillingSource = "subscription" },
		"subscription": func(value *UsageMetricBucket) { value.SubscriptionId++ },
	} {
		candidate := base
		mutate(&candidate)
		require.NotEqual(t, base.DimensionHash, UsageMetricDimensionHash(candidate), name)
	}
}

func TestUpsertAndReadUsageMetricBucketsAreIdempotentAndBounded(t *testing.T) {
	db := openUsageMetricBucketTestDB(t)
	location := mustUsageMetricLocation(t, "Asia/Shanghai")
	base := time.Date(2026, time.August, 25, 10, 0, 0, 0, location).Unix()
	logs := []Log{
		{Id: 1, UserId: 7, CreatedAt: base + 10, Type: LogTypeConsume, Settled: true, ModelName: "m", TokenId: 9, Quota: 10},
		{Id: 2, UserId: 7, CreatedAt: base + 3610, Type: LogTypeConsume, Settled: true, ModelName: "m", TokenId: 9, Quota: 20},
	}
	hourBuckets, err := BuildUsageMetricBuckets(logs, UsageMetricGranularityHour, location, "v1", 100, 0)
	require.NoError(t, err)
	dayBuckets, err := BuildUsageMetricBuckets(logs, UsageMetricGranularityDay, location, "v1", 100, 0)
	require.NoError(t, err)
	require.NoError(t, UpsertUsageMetricBuckets(db, hourBuckets))
	require.NoError(t, UpsertUsageMetricBuckets(db, hourBuckets))
	var count int64
	require.NoError(t, db.Model(&UsageMetricBucket{}).Count(&count).Error)
	require.EqualValues(t, len(hourBuckets), count)

	// Replaying a complete snapshot replaces the counters rather than adding
	// them, so a retry cannot double-count a source batch.
	updated := hourBuckets[0]
	updated.RequestCount = 99
	updated.SuccessCount = 99
	updated.Quota = 999
	updated.ComputedAt = 101
	updated.Watermark = 8
	require.NoError(t, UpsertUsageMetricBuckets(db, []UsageMetricBucket{updated}))
	var stored UsageMetricBucket
	require.NoError(t, db.Where("granularity = ? AND bucket_start = ? AND dimension_hash = ?", updated.Granularity, updated.BucketStart, updated.DimensionHash).First(&stored).Error)
	require.EqualValues(t, 99, stored.RequestCount)
	require.EqualValues(t, 999, stored.Quota)
	require.EqualValues(t, 101, stored.ComputedAt)
	require.EqualValues(t, 8, stored.Watermark)

	require.NoError(t, UpsertUsageMetricBuckets(db, dayBuckets))
	require.NoError(t, db.Model(&UsageMetricBucket{}).Count(&count).Error)
	require.EqualValues(t, len(hourBuckets)+len(dayBuckets), count)

	settled := true
	rows, err := ReadUsageMetricBuckets(context.Background(), db, UsageMetricBucketQuery{
		Granularity: UsageMetricGranularityHour,
		StartTime:   base,
		EndTime:     base + 3600,
		UserId:      7,
		Settled:     &settled,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.EqualValues(t, base, rows[0].BucketStart)

	available, err := UsageMetricBucketsAvailable(context.Background(), db)
	require.NoError(t, err)
	require.True(t, available)
	emptyDB := openUsageMetricBucketTestDB(t)
	available, err = UsageMetricBucketsAvailable(context.Background(), emptyDB)
	require.NoError(t, err)
	require.False(t, available)

	_, err = ListUsageMetricBuckets(context.Background(), db, UsageMetricBucketQuery{
		Granularity: UsageMetricGranularityHour,
		StartTime:   base + 3600,
		EndTime:     base,
	})
	require.Error(t, err)
}

func TestValidateUsageMetricBucketsRejectsTamperingAndDuplicateKeys(t *testing.T) {
	bucket := UsageMetricBucket{
		Granularity: UsageMetricGranularityHour, BucketStart: 100, UserId: 1, Type: LogTypeConsume,
		Settled: true, SourceVersion: "v1", ComputedAt: 200, Watermark: 3, FirstLogAt: 100, LastLogAt: 100,
	}
	bucket.RefreshDimensionHash()
	require.NoError(t, ValidateUsageMetricBuckets([]UsageMetricBucket{bucket}))

	tampered := bucket
	tampered.ModelName = "changed"
	require.ErrorContains(t, ValidateUsageMetricBuckets([]UsageMetricBucket{tampered}), "dimension hash")
	require.ErrorContains(t, ValidateUsageMetricBuckets([]UsageMetricBucket{bucket, bucket}), "duplicate")
	_, err := UsageMetricBucketsAvailable(context.Background(), nil)
	require.Error(t, err)
}
