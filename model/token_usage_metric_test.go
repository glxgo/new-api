package model

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReadTokenTodayUsageFromMetricBucketsMergesSealedHoursAndRawTail(t *testing.T) {
	db := setupUsageMetricReadTestDB(t)
	t.Setenv(UsageMetricReadEnableEnv, "true")
	t.Setenv("USAGE_METRIC_TIMEZONE", "Asia/Shanghai")
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	start := time.Date(2024, time.April, 1, 0, 0, 0, 0, location).Unix()
	end := start + 3*3600 + 30*60
	const tokenID = 771

	// The first three hours are sealed projection buckets. The final half hour
	// is intentionally left to the exact raw-tail query.
	for hour, quota := range []int64{100, 200, 300} {
		bucketStart := start + int64(hour)*3600
		coverage := UsageMetricCoverage{
			Granularity:   UsageMetricGranularityHour,
			BucketStart:   bucketStart,
			BucketEnd:     bucketStart + 3600,
			Timezone:      "Asia/Shanghai",
			ComputedAt:    bucketStart + 5*3600,
			Watermark:     900 + int64(hour),
			SourceVersion: UsageMetricSourceVersionV1,
			IsComplete:    true,
		}
		require.NoError(t, db.Create(&coverage).Error)
		bucket := UsageMetricBucket{
			Granularity:   UsageMetricGranularityHour,
			BucketStart:   bucketStart,
			UserId:        41,
			Type:          LogTypeConsume,
			Settled:       true,
			TokenId:       tokenID,
			RequestCount:  1,
			SuccessCount:  1,
			Quota:         quota,
			ComputedAt:    coverage.ComputedAt,
			Watermark:     coverage.Watermark,
			SourceVersion: coverage.SourceVersion,
			Timezone:      coverage.Timezone,
			CoverageStart: bucketStart,
			CoverageEnd:   coverage.BucketEnd,
			IsComplete:    true,
		}
		bucket.RefreshDimensionHash()
		require.NoError(t, db.Create(&bucket).Error)
	}
	require.NoError(t, db.Create(&Log{
		UserId: 41, TokenId: tokenID, Type: LogTypeConsume, Settled: true,
		CreatedAt: start + 3*3600 + 10*60, Quota: 25,
	}).Error)

	values, ready, err := readTokenTodayUsageFromMetricBuckets(context.Background(), []int{tokenID}, start, end)
	require.NoError(t, err)
	require.True(t, ready)
	require.EqualValues(t, 625, values[tokenID])
}
