package model

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUsageMetricCoverageRowUsableRejectsInvalidFreshnessAndMetadata(t *testing.T) {
	location := mustUsageMetricLocation(t, "UTC")
	now := time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC).Unix()
	start := now - 4*3600
	end := start + 3600
	valid := UsageMetricCoverage{
		Granularity:   UsageMetricGranularityHour,
		BucketStart:   start,
		BucketEnd:     end,
		Timezone:      "UTC",
		ComputedAt:    end,
		Watermark:     42,
		SourceVersion: UsageMetricSourceVersionV1,
		IsComplete:    true,
	}
	require.True(t, usageMetricCoverageRowUsable(valid, UsageMetricGranularityHour, "UTC", location, now))

	cases := map[string]func(*UsageMetricCoverage){
		"old source version":         func(value *UsageMetricCoverage) { value.SourceVersion = "v0" },
		"missing computed at":        func(value *UsageMetricCoverage) { value.ComputedAt = 0 },
		"computed before bucket end": func(value *UsageMetricCoverage) { value.ComputedAt = end - 1 },
		"computed in the future": func(value *UsageMetricCoverage) {
			value.ComputedAt = now + 2*int64(usageMetricProjectionClockSkew/time.Second)
		},
		"invalid watermark":  func(value *UsageMetricCoverage) { value.Watermark = -1 },
		"wrong coverage end": func(value *UsageMetricCoverage) { value.BucketEnd = end + 1 },
		"unsealed bucket":    func(value *UsageMetricCoverage) { value.BucketEnd = now + 1 },
		"incomplete marker":  func(value *UsageMetricCoverage) { value.IsComplete = false },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			require.False(t, usageMetricCoverageRowUsable(candidate, UsageMetricGranularityHour, "UTC", location, now))
		})
	}
}

func TestUsageMetricBucketRowsUsableRequiresCoverageMetadataAgreement(t *testing.T) {
	location := mustUsageMetricLocation(t, "UTC")
	now := time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC).Unix()
	start := now - 4*3600
	end := start + 3600
	marker := UsageMetricCoverage{
		Granularity:   UsageMetricGranularityHour,
		BucketStart:   start,
		BucketEnd:     end,
		Timezone:      "UTC",
		ComputedAt:    end,
		Watermark:     42,
		SourceVersion: UsageMetricSourceVersionV1,
		IsComplete:    true,
	}
	bucket := UsageMetricBucket{
		Granularity:   UsageMetricGranularityHour,
		BucketStart:   start,
		UserId:        7,
		Type:          LogTypeConsume,
		Settled:       true,
		RequestCount:  1,
		SuccessCount:  1,
		Quota:         100,
		ComputedAt:    marker.ComputedAt,
		Watermark:     marker.Watermark,
		SourceVersion: marker.SourceVersion,
		Timezone:      marker.Timezone,
		CoverageStart: start,
		CoverageEnd:   end,
		IsComplete:    true,
	}
	bucket.RefreshDimensionHash()
	coverage := map[int64]UsageMetricCoverage{start: marker}
	require.True(t, usageMetricBucketRowsUsable([]UsageMetricBucket{bucket}, coverage, UsageMetricGranularityHour, "UTC", location, now))

	cases := map[string]func(*UsageMetricBucket){
		"source version": func(value *UsageMetricBucket) { value.SourceVersion = "v0" },
		"computed at":    func(value *UsageMetricBucket) { value.ComputedAt++ },
		"watermark":      func(value *UsageMetricBucket) { value.Watermark++ },
		"coverage start": func(value *UsageMetricBucket) { value.CoverageStart++ },
		"coverage end":   func(value *UsageMetricBucket) { value.CoverageEnd++ },
		"dimension hash": func(value *UsageMetricBucket) { value.ModelName = "tampered" },
		"incomplete":     func(value *UsageMetricBucket) { value.IsComplete = false },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			candidate := bucket
			mutate(&candidate)
			require.False(t, usageMetricBucketRowsUsable([]UsageMetricBucket{candidate}, coverage, UsageMetricGranularityHour, "UTC", location, now))
		})
	}
}

func TestUsageMetricReadFallsBackWhenCoverageUsesOldProjection(t *testing.T) {
	db := setupUsageMetricReadTestDB(t)
	t.Setenv(UsageMetricReadEnableEnv, "true")
	t.Setenv("USAGE_METRIC_TIMEZONE", "UTC")
	userID := 90301
	base := time.Date(2024, time.April, 1, 0, 0, 0, 0, time.UTC).Unix()
	require.NoError(t, db.Create([]Log{
		{UserId: userID, CreatedAt: base + 300, Type: LogTypeConsume, ModelName: "raw", Quota: 11},
		{UserId: userID, CreatedAt: base + 2*3600 + 300, Type: LogTypeConsume, ModelName: "raw", Quota: 22},
	}).Error)
	coverage := make([]UsageMetricCoverage, 0, 3)
	for hour := int64(0); hour < 3; hour++ {
		start := base + hour*3600
		version := UsageMetricSourceVersionV1
		if hour == 1 {
			version = "v0"
		}
		coverage = append(coverage, UsageMetricCoverage{
			Granularity: UsageMetricGranularityHour,
			BucketStart: start,
			BucketEnd:   start + 3600,
			Timezone:    "UTC", ComputedAt: start + 4*3600,
			Watermark: 10 + hour, SourceVersion: version, IsComplete: true,
		})
	}
	require.NoError(t, db.Create(&coverage).Error)

	stats, err := GetUserUsageStatisticsWithContext(context.Background(), userID, base, base+3*3600, 3600)
	require.NoError(t, err)
	require.EqualValues(t, 2, stats.Summary.SuccessCount)
	require.EqualValues(t, 33, stats.Summary.Quota)
}

func TestUsageMetricReadFallsBackWhenBucketMetadataIsStale(t *testing.T) {
	db := setupUsageMetricReadTestDB(t)
	t.Setenv(UsageMetricReadEnableEnv, "true")
	t.Setenv("USAGE_METRIC_TIMEZONE", "UTC")
	userID := 90302
	base := time.Date(2024, time.April, 2, 0, 0, 0, 0, time.UTC).Unix()
	require.NoError(t, db.Create(&Log{
		UserId: userID, CreatedAt: base + 300, Type: LogTypeConsume, ModelName: "raw", Quota: 17,
	}).Error)
	coverage := UsageMetricCoverage{
		Granularity: UsageMetricGranularityHour, BucketStart: base,
		BucketEnd: base + 3600, Timezone: "UTC", ComputedAt: base + 4*3600,
		Watermark: 1, SourceVersion: UsageMetricSourceVersionV1, IsComplete: true,
	}
	require.NoError(t, db.Create(&coverage).Error)
	bucket := UsageMetricBucket{
		Granularity: UsageMetricGranularityHour, BucketStart: base, UserId: userID,
		Type: LogTypeConsume, Settled: true, RequestCount: 1, SuccessCount: 1,
		Quota: 999, ComputedAt: coverage.ComputedAt - 1, Watermark: coverage.Watermark,
		SourceVersion: coverage.SourceVersion, Timezone: coverage.Timezone,
		CoverageStart: base, CoverageEnd: base + 3600,
	}
	bucket.RefreshDimensionHash()
	require.NoError(t, db.Create(&bucket).Error)

	stats, err := GetUserUsageStatisticsWithContext(context.Background(), userID, base, base+3*3600, 3600)
	require.NoError(t, err)
	require.EqualValues(t, 1, stats.Summary.SuccessCount)
	require.EqualValues(t, 17, stats.Summary.Quota)
}

func TestUsageMetricRangePartsKeepRawTailsDisjointFromFullBuckets(t *testing.T) {
	location := time.UTC
	base := time.Date(2026, time.January, 2, 0, 0, 0, 0, location).Unix()
	parts, err := usageMetricRangeParts(base+15, base+3*3600+30, UsageMetricGranularityHour, location)
	require.NoError(t, err)
	for _, fullStart := range parts.FullBucketStarts {
		fullEnd := fullStart + 3600
		for _, raw := range parts.RawRanges {
			require.True(t, raw[1] <= fullStart || raw[0] >= fullEnd,
				"raw range [%d,%d) overlaps full bucket [%d,%d)", raw[0], raw[1], fullStart, fullEnd)
		}
	}
}

func setupUsageMetricReadTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := DB
	originalLogDB := LOG_DB
	originalSQLite := common.UsingSQLite
	originalMySQL := common.UsingMySQL
	originalPostgreSQL := common.UsingPostgreSQL

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}, &UsageLogDailyAggregate{}, &UserSubscription{}, &UsageMetricBucket{}, &UsageMetricCoverage{}))
	DB = db
	LOG_DB = db
	t.Cleanup(func() {
		DB = originalDB
		LOG_DB = originalLogDB
		common.UsingSQLite = originalSQLite
		common.UsingMySQL = originalMySQL
		common.UsingPostgreSQL = originalPostgreSQL
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}
