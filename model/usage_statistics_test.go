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

func setupUsageStatisticsTestDB(t *testing.T) *gorm.DB {
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
	require.NoError(t, db.AutoMigrate(&Log{}, &UsageLogDailyAggregate{}, &UserSubscription{}))
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

func TestGetUserUsageStatisticsAggregatesRequestHealthCacheAndBilling(t *testing.T) {
	db := setupUsageStatisticsTestDB(t)
	require.NoError(t, db.Create(&[]UserSubscription{
		{
			Id: 10, UserId: 1, PlanTitle: "Weekly plan", Remark: "Used plan",
			AmountTotal: 1_000, Status: "active",
		},
		{
			Id: 11, UserId: 1, PlanTitle: "Monthly plan", Remark: "Unused plan",
			AmountTotal: 2_000, Status: "active",
		},
	}).Error)
	logs := []Log{
		{
			UserId: 1, CreatedAt: 110, Type: LogTypeConsume, ModelName: "gpt-test",
			Quota: 100, PreDiscountQuota: 200, PromptTokens: 100, CacheTokens: 30, CompletionTokens: 20,
			BillingSource: "wallet",
		},
		{
			UserId: 1, CreatedAt: 170, Type: LogTypeConsume, ModelName: "gpt-test",
			Quota: 200, PromptTokens: 40, CacheTokens: 60, CompletionTokens: 10,
			BillingSource: "subscription", SubscriptionId: 10,
		},
		{
			UserId: 1, CreatedAt: 172, Type: LogTypeConsume, ModelName: "gpt-test",
			Quota: 50, PromptTokens: 20, CacheTokens: 5, CompletionTokens: 5,
			BillingSource: "virtual_membership",
		},
		{UserId: 1, CreatedAt: 175, Type: LogTypeError, ModelName: "gpt-test"},
		{UserId: 2, CreatedAt: 180, Type: LogTypeConsume, ModelName: "ignored", Quota: 999},
	}
	require.NoError(t, db.Create(&logs).Error)

	stats, err := GetUserUsageStatistics(1, 100, 220, 60)
	require.NoError(t, err)
	require.EqualValues(t, 4, stats.Summary.RequestCount)
	require.EqualValues(t, 3, stats.Summary.SuccessCount)
	require.EqualValues(t, 1, stats.Summary.ErrorCount)
	require.InDelta(t, 75, stats.Summary.SuccessRate, 0.001)
	require.EqualValues(t, 350, stats.Summary.Quota)
	require.EqualValues(t, 450, stats.Summary.PreDiscountQuota)
	require.EqualValues(t, 100, stats.Summary.WalletQuota)
	require.EqualValues(t, 200, stats.Summary.SubscriptionQuota)
	require.EqualValues(t, 50, stats.Summary.VirtualMembershipQuota)
	require.EqualValues(t, 160, stats.Summary.PromptTokens)
	require.EqualValues(t, 95, stats.Summary.CacheTokens)
	require.EqualValues(t, 220, stats.Summary.EffectivePrompt)
	require.EqualValues(t, 35, stats.Summary.CompletionTokens)
	require.EqualValues(t, 195, stats.Summary.TotalTokens)
	require.InDelta(t, 43.1818, stats.Summary.CacheHitRate, 0.001)

	require.Len(t, stats.Series, 3)
	require.EqualValues(t, 1, stats.Series[0].RequestCount)
	require.EqualValues(t, 3, stats.Series[1].RequestCount)
	require.Zero(t, stats.Series[2].RequestCount)

	require.Len(t, stats.Models, 1)
	require.Equal(t, "gpt-test", stats.Models[0].ModelName)
	require.EqualValues(t, 3, stats.Models[0].RequestCount)
	require.EqualValues(t, 195, stats.Models[0].TotalTokens)

	require.Len(t, stats.Subscriptions, 1)
	require.EqualValues(t, 10, stats.Subscriptions[0].SubscriptionId)
	require.Equal(t, "Used plan", stats.Subscriptions[0].Title)
	require.EqualValues(t, 200, stats.Subscriptions[0].Quota)
	require.EqualValues(t, 1, stats.Subscriptions[0].RequestCount)
}

func TestGetUserUsageStatisticsRejectsUnboundedRanges(t *testing.T) {
	setupUsageStatisticsTestDB(t)

	_, err := GetUserUsageStatistics(1, 0, 100, 60)
	require.Error(t, err)
	_, err = GetUserUsageStatistics(0, 1, 100, 60)
	require.Error(t, err)
}

func TestGetUserUsageStatisticsDoesNotOvercountPartialArchivedDay(t *testing.T) {
	db := setupUsageStatisticsTestDB(t)
	// Use a historical day so the cache key is not treated as a hot/live
	// projection. The aggregate represents the whole day; only the live row is
	// inside the selected one-hour tail.
	dayStart := usageLogAggregateBucketStart(time.Date(2024, 1, 10, 0, 0, 0, 0, time.Local).Unix())
	userID := 90101
	require.NoError(t, db.Create(&UsageLogDailyAggregate{
		BucketStart:  dayStart,
		UserId:       userID,
		Type:         LogTypeConsume,
		RequestCount: 9,
		Quota:        900,
		PromptTokens: 90,
	}).Error)
	require.NoError(t, db.Create(&UsageLogDailyAggregate{
		BucketStart:  dayStart,
		UserId:       userID,
		Type:         LogTypeConsume,
		ModelName:    "contained-batch",
		RequestCount: 1,
		Quota:        5,
		FirstLogAt:   dayStart + 3600,
		LastLogAt:    dayStart + 3610,
	}).Error)
	require.NoError(t, db.Create(&Log{
		UserId: userID, CreatedAt: dayStart + 2*3600, Type: LogTypeConsume,
		Quota: 40, PromptTokens: 4, CompletionTokens: 1,
	}).Error)

	stats, err := GetUserUsageStatisticsWithContext(context.Background(), userID, dayStart+3600, dayStart+3*3600, 3600)
	require.NoError(t, err)
	require.EqualValues(t, 45, stats.Summary.Quota)
	require.EqualValues(t, 2, stats.Summary.SuccessCount)
}

func TestGetUserUsageStatisticsFallsBackToLiveLogsWhenArchiveTableMissing(t *testing.T) {
	db := setupUsageStatisticsTestDB(t)
	dayStart := usageLogAggregateBucketStart(time.Date(2024, 2, 12, 0, 0, 0, 0, time.Local).Unix())
	userID := 90102
	require.NoError(t, db.Create(&Log{
		UserId: userID, CreatedAt: dayStart + 60, Type: LogTypeConsume,
		Quota: 55, PromptTokens: 5, CompletionTokens: 2,
	}).Error)
	require.NoError(t, db.Migrator().DropTable(&UsageLogDailyAggregate{}))

	stats, err := GetUserUsageStatisticsWithContext(context.Background(), userID, dayStart, dayStart+86400, 3600)
	require.NoError(t, err)
	require.EqualValues(t, 55, stats.Summary.Quota)
	require.EqualValues(t, 1, stats.Summary.SuccessCount)
}

func TestGetUserCacheRateUsesRawLogsWithoutArchiveTable(t *testing.T) {
	db := setupUsageStatisticsTestDB(t)
	start := usageLogAggregateBucketStart(time.Date(2024, 3, 8, 0, 0, 0, 0, time.Local).Unix())
	userID := 90103
	require.NoError(t, db.Create(&Log{
		UserId: userID, CreatedAt: start + 120, Type: LogTypeConsume,
		PromptTokens: 10, CacheTokens: 20,
	}).Error)
	require.NoError(t, db.Migrator().DropTable(&UsageLogDailyAggregate{}))

	cacheTokens, promptTokens, err := GetUserCacheRateWithContext(context.Background(), int64(userID), start, start+86400)
	require.NoError(t, err)
	require.EqualValues(t, 20, cacheTokens)
	require.EqualValues(t, 30, promptTokens)
}

func TestGetUserUsageStatisticsReadsCompleteMetricProjectionWhenEnabled(t *testing.T) {
	db := setupUsageStatisticsTestDB(t)
	t.Setenv(UsageMetricReadEnableEnv, "true")
	t.Setenv("USAGE_METRIC_TIMEZONE", "UTC")
	require.NoError(t, db.AutoMigrate(&UsageMetricBucket{}, &UsageMetricCoverage{}))
	userID := 90201
	start := time.Date(2024, 3, 4, 0, 0, 0, 0, time.UTC).Unix()
	var buckets []UsageMetricBucket
	var coverage []UsageMetricCoverage
	for hour := int64(0); hour < 3; hour++ {
		bucketStart := start + hour*3600
		bucket := UsageMetricBucket{
			Granularity:           UsageMetricGranularityHour,
			BucketStart:           bucketStart,
			UserId:                userID,
			Type:                  LogTypeConsume,
			Settled:               true,
			ModelName:             "projection-model",
			Quota:                 100 + hour,
			PromptTokens:          10,
			CompletionTokens:      5,
			EffectivePromptTokens: 10,
			RequestCount:          1,
			SuccessCount:          1,
			ComputedAt:            start + 4*3600,
			Watermark:             100 + hour,
			SourceVersion:         UsageMetricSourceVersionV1,
			Timezone:              "UTC",
			CoverageStart:         bucketStart,
			CoverageEnd:           bucketStart + 3600,
			IsComplete:            true,
		}
		bucket.RefreshDimensionHash()
		buckets = append(buckets, bucket)
		coverage = append(coverage, UsageMetricCoverage{
			Granularity: UsageMetricGranularityHour,
			BucketStart: bucketStart,
			BucketEnd:   bucketStart + 3600,
			Timezone:    "UTC", ComputedAt: start + 4*3600,
			Watermark: 100 + hour, SourceVersion: UsageMetricSourceVersionV1,
			IsComplete: true,
		})
	}
	require.NoError(t, db.Create(&buckets).Error)
	require.NoError(t, db.Create(&coverage).Error)

	stats, err := GetUserUsageStatisticsWithContext(context.Background(), userID, start, start+3*3600, 3600)
	require.NoError(t, err)
	require.EqualValues(t, 3, stats.Summary.RequestCount)
	require.EqualValues(t, 303, stats.Summary.Quota)
	require.Len(t, stats.Series, 3)
	require.Len(t, stats.Models, 1)
	require.Equal(t, "projection-model", stats.Models[0].ModelName)
}

func TestGetUserUsageStatisticsCombinesMetricBucketsWithPartialRawTails(t *testing.T) {
	db := setupUsageStatisticsTestDB(t)
	t.Setenv(UsageMetricReadEnableEnv, "true")
	t.Setenv("USAGE_METRIC_TIMEZONE", "UTC")
	require.NoError(t, db.AutoMigrate(&UsageMetricBucket{}, &UsageMetricCoverage{}))
	userID := 90202
	base := time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC).Unix()
	start := base + 15
	end := base + 3*3600 + 30

	// Only the two complete middle hours are represented by metric buckets.
	// Boundary events remain in logs and must be read exactly, without pulling
	// in the rest of either boundary hour.
	require.NoError(t, db.Create(&[]Log{
		{UserId: userID, CreatedAt: base + 30, Type: LogTypeConsume, ModelName: "raw-left", Quota: 10, PromptTokens: 2, CompletionTokens: 1},
		{UserId: userID, CreatedAt: base + 3*3600 + 15, Type: LogTypeConsume, ModelName: "raw-right", Quota: 20, PromptTokens: 3, CompletionTokens: 2},
	}).Error)

	buckets := make([]UsageMetricBucket, 0, 2)
	coverage := make([]UsageMetricCoverage, 0, 2)
	for offset, quota := range []int64{100, 200} {
		bucketStart := base + int64(offset+1)*3600
		bucket := UsageMetricBucket{
			Granularity:           UsageMetricGranularityHour,
			BucketStart:           bucketStart,
			UserId:                userID,
			Type:                  LogTypeConsume,
			Settled:               true,
			ModelName:             "projected",
			Quota:                 quota,
			PromptTokens:          10,
			CompletionTokens:      5,
			EffectivePromptTokens: 10,
			RequestCount:          1,
			SuccessCount:          1,
			ComputedAt:            end,
			Watermark:             int64(offset + 1),
			SourceVersion:         UsageMetricSourceVersionV1,
			Timezone:              "UTC",
			CoverageStart:         bucketStart,
			CoverageEnd:           bucketStart + 3600,
			IsComplete:            true,
		}
		bucket.RefreshDimensionHash()
		buckets = append(buckets, bucket)
		coverage = append(coverage, UsageMetricCoverage{
			Granularity:   UsageMetricGranularityHour,
			BucketStart:   bucketStart,
			BucketEnd:     bucketStart + 3600,
			Timezone:      "UTC",
			ComputedAt:    end,
			Watermark:     int64(offset + 1),
			SourceVersion: UsageMetricSourceVersionV1,
			IsComplete:    true,
		})
	}
	require.NoError(t, db.Create(&buckets).Error)
	require.NoError(t, db.Create(&coverage).Error)

	stats, err := GetUserUsageStatisticsWithContext(context.Background(), userID, start, end, 3600)
	require.NoError(t, err)
	require.EqualValues(t, 4, stats.Summary.RequestCount)
	require.EqualValues(t, 4, stats.Summary.SuccessCount)
	require.EqualValues(t, 330, stats.Summary.Quota)
	require.Len(t, stats.Series, 4)
	require.EqualValues(t, 10, stats.Series[0].Quota)
	require.EqualValues(t, 100, stats.Series[1].Quota)
	require.EqualValues(t, 200, stats.Series[2].Quota)
	require.EqualValues(t, 20, stats.Series[3].Quota)
}

func TestNormalizeHotUsageWindowPreservesExactRange(t *testing.T) {
	now := time.Now().Unix()
	start, end := now-2*3600, now
	gotStart, gotEnd := normalizeHotUsageWindow(start, end)
	require.Equal(t, start, gotStart)
	require.Equal(t, end, gotEnd)
	nearStart, nearEnd := normalizeHotUsageWindow(start, end-1)
	require.NotEqual(t, gotEnd, nearEnd)
	// The left boundary is unchanged for this one-second-shorter range;
	// preserving exact windows only requires the right boundary to differ.
	require.Equal(t, gotStart, nearStart)
}

func TestGetUserCacheRateCombinesMetricBucketsWithPartialRawTails(t *testing.T) {
	db := setupUsageStatisticsTestDB(t)
	t.Setenv(UsageMetricReadEnableEnv, "true")
	t.Setenv("USAGE_METRIC_TIMEZONE", "UTC")
	require.NoError(t, db.AutoMigrate(&UsageMetricBucket{}, &UsageMetricCoverage{}))
	userID := 90203
	base := time.Date(2024, 3, 6, 0, 0, 0, 0, time.UTC).Unix()
	start := base + 15
	end := base + 3*3600 + 30
	require.NoError(t, db.Create(&[]Log{
		{UserId: userID, CreatedAt: base + 30, Type: LogTypeConsume, PromptTokens: 10, CacheTokens: 20},
		{UserId: userID, CreatedAt: base + 3*3600 + 15, Type: LogTypeConsume, PromptTokens: 5, CacheTokens: 1},
	}).Error)

	buckets := make([]UsageMetricBucket, 0, 2)
	coverage := make([]UsageMetricCoverage, 0, 2)
	for offset := 1; offset <= 2; offset++ {
		bucketStart := base + int64(offset)*3600
		bucket := UsageMetricBucket{
			Granularity:           UsageMetricGranularityHour,
			BucketStart:           bucketStart,
			UserId:                userID,
			Type:                  LogTypeConsume,
			Settled:               true,
			PromptTokens:          10,
			CacheTokens:           20,
			EffectivePromptTokens: 30,
			RequestCount:          1,
			SuccessCount:          1,
			ComputedAt:            end,
			Watermark:             int64(offset),
			SourceVersion:         UsageMetricSourceVersionV1,
			Timezone:              "UTC",
			CoverageStart:         bucketStart,
			CoverageEnd:           bucketStart + 3600,
			IsComplete:            true,
		}
		bucket.RefreshDimensionHash()
		buckets = append(buckets, bucket)
		coverage = append(coverage, UsageMetricCoverage{
			Granularity:   UsageMetricGranularityHour,
			BucketStart:   bucketStart,
			BucketEnd:     bucketStart + 3600,
			Timezone:      "UTC",
			ComputedAt:    end,
			Watermark:     int64(offset),
			SourceVersion: UsageMetricSourceVersionV1,
			IsComplete:    true,
		})
	}
	require.NoError(t, db.Create(&buckets).Error)
	require.NoError(t, db.Create(&coverage).Error)

	cacheTokens, effectivePrompt, err := GetUserCacheRateWithContext(context.Background(), int64(userID), start, end)
	require.NoError(t, err)
	require.EqualValues(t, 61, cacheTokens)
	require.EqualValues(t, 95, effectivePrompt)
}
