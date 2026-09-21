package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUsageMetricWorkerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldSQLite, oldMySQL, oldPostgres := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = true, false, false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Log{},
		&model.UsageMetricBucket{},
		&model.UsageMetricCoverage{},
		&model.UsageMetricCheckpoint{},
		&model.UsageMetricBatch{},
	))
	model.DB, model.LOG_DB = db, db
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = oldSQLite, oldMySQL, oldPostgres
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestApplyUsageMetricSourceBatchReplacesSnapshotsWithoutDoubleCounting(t *testing.T) {
	db := setupUsageMetricWorkerTestDB(t)
	location := time.UTC
	base := time.Now().Add(-3 * time.Hour).Unix()
	base = base/3600*3600 + 12*60
	logs := []model.Log{
		{Id: 1, UserId: 7, CreatedAt: base, Type: model.LogTypeConsume, Quota: 100, PromptTokens: 10, CompletionTokens: 5, Group: "g", BillingSource: "wallet", Settled: true},
		{Id: 2, UserId: 7, CreatedAt: base + 20, Type: model.LogTypeError, Group: "g"},
		{Id: 3, UserId: 7, CreatedAt: base + 40, Type: model.LogTypeTopup, Quota: 999},
	}
	require.NoError(t, db.Create(&logs).Error)
	require.NoError(t, applyUsageMetricSourceBatch(context.Background(), location, "UTC", logs, 0, 3))

	var bucket model.UsageMetricBucket
	bucket = model.UsageMetricBucket{}
	require.NoError(t, db.Where("granularity = ? AND user_id = ? AND type = ?", model.UsageMetricGranularityHour, 7, model.LogTypeConsume).First(&bucket).Error)
	require.EqualValues(t, 1, bucket.RequestCount)
	require.EqualValues(t, 100, bucket.Quota)
	require.Equal(t, "UTC", bucket.Timezone)

	// Replaying the same source batch replaces the affected windows. It must
	// not add a second copy of the counters.
	require.NoError(t, applyUsageMetricSourceBatch(context.Background(), location, "UTC", logs, 0, 3))
	var count int64
	require.NoError(t, db.Model(&model.UsageMetricBucket{}).
		Where("granularity = ? AND user_id = ? AND type = ?", model.UsageMetricGranularityHour, 7, model.LogTypeConsume).
		Count(&count).Error)
	require.EqualValues(t, 1, count)
	bucket = model.UsageMetricBucket{}
	require.NoError(t, db.Where("granularity = ? AND user_id = ? AND type = ?", model.UsageMetricGranularityHour, 7, model.LogTypeConsume).First(&bucket).Error)
	require.EqualValues(t, 100, bucket.Quota)

	var coverage model.UsageMetricCoverage
	require.NoError(t, db.Where("granularity = ? AND bucket_start = ?", model.UsageMetricGranularityHour, base/3600*3600).First(&coverage).Error)
	require.EqualValues(t, 3, coverage.Watermark)
}

func TestRebuildUsageMetricBucketsReadsSelectedWindowsOnceAndHonorsWatermark(t *testing.T) {
	db := setupUsageMetricWorkerTestDB(t)
	oldBucketLimit, oldRebuildLimit := usageMetricWorkerMaxRowsPerBucket, usageMetricWorkerMaxRowsPerRebuild
	usageMetricWorkerMaxRowsPerBucket, usageMetricWorkerMaxRowsPerRebuild = 100, 100
	t.Cleanup(func() {
		usageMetricWorkerMaxRowsPerBucket, usageMetricWorkerMaxRowsPerRebuild = oldBucketLimit, oldRebuildLimit
	})

	location := time.UTC
	base := time.Now().Add(-3 * time.Hour).Unix()
	base = base/3600*3600 + 12*60
	logs := []model.Log{
		{Id: 1, UserId: 8, CreatedAt: base, Type: model.LogTypeConsume, Quota: 10, Settled: true},
		{Id: 2, UserId: 8, CreatedAt: base + 10, Type: model.LogTypeConsume, Quota: 20, Settled: true},
	}
	require.NoError(t, db.Create(&logs).Error)
	start := base / 3600 * 3600
	require.NoError(t, rebuildUsageMetricBuckets(context.Background(), location, "UTC", model.UsageMetricGranularityHour, []int64{start}, 1, base+7200))

	var bucket model.UsageMetricBucket
	require.NoError(t, db.Where("granularity = ? AND bucket_start = ? AND user_id = ?", model.UsageMetricGranularityHour, start, 8).First(&bucket).Error)
	require.EqualValues(t, 1, bucket.RequestCount)
	require.EqualValues(t, 10, bucket.Quota)

	// A later watermark includes the second source row without adding a second
	// copy of the first one.
	require.NoError(t, rebuildUsageMetricBuckets(context.Background(), location, "UTC", model.UsageMetricGranularityHour, []int64{start}, 2, base+7200))
	bucket = model.UsageMetricBucket{}
	require.NoError(t, db.Where("granularity = ? AND bucket_start = ? AND user_id = ?", model.UsageMetricGranularityHour, start, 8).First(&bucket).Error)
	require.EqualValues(t, 2, bucket.RequestCount)
	require.EqualValues(t, 30, bucket.Quota)
}

func TestRebuildUsageMetricBucketsMarksOversizedWindowIncompleteWithoutDeletingSnapshot(t *testing.T) {
	db := setupUsageMetricWorkerTestDB(t)
	oldBucketLimit, oldRebuildLimit := usageMetricWorkerMaxRowsPerBucket, usageMetricWorkerMaxRowsPerRebuild
	usageMetricWorkerMaxRowsPerBucket, usageMetricWorkerMaxRowsPerRebuild = 1, 10
	t.Cleanup(func() {
		usageMetricWorkerMaxRowsPerBucket, usageMetricWorkerMaxRowsPerRebuild = oldBucketLimit, oldRebuildLimit
	})

	location := time.UTC
	base := time.Now().Add(-3 * time.Hour).Unix()
	base = base/3600*3600 + 12*60
	start := base / 3600 * 3600
	// Seed an existing projection row to prove that the protective skip does
	// not delete a previously valid snapshot. Readers are blocked by the false
	// coverage marker and therefore fall back to the exact source path.
	seed := model.UsageMetricBucket{
		Granularity:   model.UsageMetricGranularityHour,
		BucketStart:   start,
		UserId:        9,
		Type:          model.LogTypeConsume,
		Settled:       true,
		RequestCount:  1,
		SuccessCount:  1,
		Quota:         99,
		ComputedAt:    base + 7200,
		Watermark:     1,
		SourceVersion: model.UsageMetricSourceVersionV1,
		Timezone:      "UTC",
		CoverageStart: start,
		CoverageEnd:   start + 3600,
		IsComplete:    true,
	}
	seed.RefreshDimensionHash()
	require.NoError(t, model.UpsertUsageMetricBuckets(db, []model.UsageMetricBucket{seed}))
	logs := []model.Log{
		{Id: 1, UserId: 9, CreatedAt: base, Type: model.LogTypeConsume, Quota: 10, Settled: true},
		{Id: 2, UserId: 9, CreatedAt: base + 10, Type: model.LogTypeConsume, Quota: 20, Settled: true},
	}
	require.NoError(t, db.Create(&logs).Error)

	require.NoError(t, rebuildUsageMetricBuckets(context.Background(), location, "UTC", model.UsageMetricGranularityHour, []int64{start}, 2, base+7200))
	var coverage model.UsageMetricCoverage
	require.NoError(t, db.Where("granularity = ? AND bucket_start = ?", model.UsageMetricGranularityHour, start).First(&coverage).Error)
	require.False(t, coverage.IsComplete)
	var bucket model.UsageMetricBucket
	require.NoError(t, db.Where("granularity = ? AND bucket_start = ? AND user_id = ?", model.UsageMetricGranularityHour, start, 9).First(&bucket).Error)
	require.EqualValues(t, 99, bucket.Quota)
}

func TestRebuildUsageMetricBucketsDoesNotEraseSnapshotForEmptyWindow(t *testing.T) {
	db := setupUsageMetricWorkerTestDB(t)
	oldBucketLimit, oldRebuildLimit := usageMetricWorkerMaxRowsPerBucket, usageMetricWorkerMaxRowsPerRebuild
	usageMetricWorkerMaxRowsPerBucket, usageMetricWorkerMaxRowsPerRebuild = 100, 100
	t.Cleanup(func() {
		usageMetricWorkerMaxRowsPerBucket, usageMetricWorkerMaxRowsPerRebuild = oldBucketLimit, oldRebuildLimit
	})

	location := time.UTC
	base := time.Now().Add(-3 * time.Hour).Unix()
	base = base/3600*3600 + 12*60
	start := base / 3600 * 3600
	seed := model.UsageMetricBucket{
		Granularity:   model.UsageMetricGranularityHour,
		BucketStart:   start,
		UserId:        10,
		Type:          model.LogTypeConsume,
		Settled:       true,
		RequestCount:  4,
		SuccessCount:  4,
		Quota:         123,
		ComputedAt:    base + 7200,
		Watermark:     10,
		SourceVersion: model.UsageMetricSourceVersionV1,
		Timezone:      "UTC",
		CoverageStart: start,
		CoverageEnd:   start + 3600,
		IsComplete:    true,
	}
	seed.RefreshDimensionHash()
	require.NoError(t, model.UpsertUsageMetricBuckets(db, []model.UsageMetricBucket{seed}))

	require.NoError(t, rebuildUsageMetricBuckets(context.Background(), location, "UTC", model.UsageMetricGranularityHour, []int64{start}, 10, base+7200))
	var coverage model.UsageMetricCoverage
	require.NoError(t, db.Where("granularity = ? AND bucket_start = ?", model.UsageMetricGranularityHour, start).First(&coverage).Error)
	require.False(t, coverage.IsComplete)
	var bucket model.UsageMetricBucket
	require.NoError(t, db.Where("granularity = ? AND bucket_start = ? AND user_id = ?", model.UsageMetricGranularityHour, start, 10).First(&bucket).Error)
	require.EqualValues(t, 123, bucket.Quota)
}

func TestFillMissingCurrentDayHourCoverageMarksOnlyProvenEmptyWindows(t *testing.T) {
	db := setupUsageMetricWorkerTestDB(t)
	location := time.UTC
	nowTime := time.Now().UTC()
	now := time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), 5, 30, 0, 0, location).Unix()
	dayStart := time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), 0, 0, 0, 0, location).Unix()
	// One hour contains a processed source row and therefore must not receive an
	// empty marker. The other sealed hours are eligible, but the helper is
	// explicitly capped to two checks per pass.
	require.NoError(t, db.Create(&model.Log{
		Id: 1, UserId: 7, CreatedAt: dayStart + 3600 + 10,
		Type: model.LogTypeConsume, Settled: true, Quota: 10,
	}).Error)

	require.NoError(t, fillMissingCurrentDayHourCoverage(context.Background(), location, "UTC", 1, now, 2))
	var rows []model.UsageMetricCoverage
	require.NoError(t, db.Where("granularity = ?", model.UsageMetricGranularityHour).Order("bucket_start ASC").Find(&rows).Error)
	require.Len(t, rows, 1)
	require.EqualValues(t, dayStart, rows[0].BucketStart)
	require.True(t, rows[0].IsComplete)
	require.EqualValues(t, dayStart+3600, rows[0].BucketEnd)

	// The second checked hour had a source row, so no marker was fabricated.
	var count int64
	require.NoError(t, db.Model(&model.UsageMetricCoverage{}).
		Where("granularity = ? AND bucket_start = ?", model.UsageMetricGranularityHour, dayStart+3600).
		Count(&count).Error)
	require.Zero(t, count)
}

func TestFillMissingCurrentDayHourCoveragePreservesExistingBucketWithoutCoverage(t *testing.T) {
	db := setupUsageMetricWorkerTestDB(t)
	location := time.UTC
	nowTime := time.Now().UTC()
	now := time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), 3, 30, 0, 0, location).Unix()
	dayStart := time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), 0, 0, 0, 0, location).Unix()
	seed := model.UsageMetricBucket{
		Granularity: model.UsageMetricGranularityHour, BucketStart: dayStart,
		UserId: 10, Type: model.LogTypeConsume, Settled: true,
		RequestCount: 1, SuccessCount: 1, Quota: 123,
		ComputedAt: now, Watermark: 10, SourceVersion: model.UsageMetricSourceVersionV1,
		Timezone: "UTC", CoverageStart: dayStart, CoverageEnd: dayStart + 3600, IsComplete: true,
	}
	seed.RefreshDimensionHash()
	require.NoError(t, model.UpsertUsageMetricBuckets(db, []model.UsageMetricBucket{seed}))

	require.NoError(t, fillMissingCurrentDayHourCoverage(context.Background(), location, "UTC", 10, now, 1))
	var count int64
	require.NoError(t, db.Model(&model.UsageMetricCoverage{}).
		Where("granularity = ? AND bucket_start = ?", model.UsageMetricGranularityHour, dayStart).
		Count(&count).Error)
	require.Zero(t, count, "an ambiguous existing snapshot must remain unavailable")
}

func TestFillMissingCurrentDayHourCoverageSkipsWhenSourceAdvancedPastWatermark(t *testing.T) {
	db := setupUsageMetricWorkerTestDB(t)
	location := time.UTC
	nowTime := time.Now().UTC()
	now := time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), 3, 30, 0, 0, location).Unix()
	dayStart := time.Date(nowTime.Year(), nowTime.Month(), nowTime.Day(), 0, 0, 0, 0, location).Unix()

	// The caller's batch ended at watermark 1, but a new eligible source row
	// arrived before empty-hour sealing. Even though the row is in a later hour,
	// the helper must leave every marker untouched and let the next pass rebuild
	// the affected window with the newer watermark.
	require.NoError(t, db.Create(&model.Log{
		Id: 2, UserId: 11, CreatedAt: dayStart + 3600 + 10,
		Type: model.LogTypeConsume, Settled: true, Quota: 20,
	}).Error)

	require.NoError(t, fillMissingCurrentDayHourCoverage(context.Background(), location, "UTC", 1, now, 2))
	var count int64
	require.NoError(t, db.Model(&model.UsageMetricCoverage{}).
		Where("granularity = ?", model.UsageMetricGranularityHour).
		Count(&count).Error)
	require.Zero(t, count, "a newer source row must prevent sealing any empty hour")
}

func TestUsageMetricReconcileIsDueWhileSourceContinues(t *testing.T) {
	now := time.Now().Unix()
	require.True(t, usageMetricReconcileDue(now, 0))
	require.True(t, usageMetricReconcileDue(now, now-int64(usageMetricWorkerReconcileEvery/time.Second)))
	require.False(t, usageMetricReconcileDue(now, now-int64(usageMetricWorkerReconcileEvery/time.Second)+1))
}
