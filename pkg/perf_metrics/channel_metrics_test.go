package perfmetrics

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelMetricsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:channel-metrics-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ChannelPerfMetric{}, &model.PerfMetric{}))
	oldDB := model.DB
	model.DB = db
	hotBuckets, channelHotBuckets = sync.Map{}, sync.Map{}
	t.Cleanup(func() {
		model.DB = oldDB
		hotBuckets, channelHotBuckets = sync.Map{}, sync.Map{}
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	return db
}

func TestChannelMetricsWeightedBoundariesAndMissingSamples(t *testing.T) {
	db := setupChannelMetricsDB(t)
	start := time.Now().Add(-time.Hour).Unix() / 60 * 60
	end := start + 3600
	rows := []model.ChannelPerfMetric{
		{ModelName: "a", ChannelId: 1, BucketTs: start, BucketSeconds: 60, RequestCount: 9, SuccessCount: 8, TtftSumMs: 900, TtftCount: 3, CacheTokens: 100, PromptTokens: 200},
		{ModelName: "b", ChannelId: 1, BucketTs: start + 60, BucketSeconds: 60, RequestCount: 1, SuccessCount: 0, TtftSumMs: 100, TtftCount: 1, CacheTokens: 50, PromptTokens: 100},
		{ModelName: "a", ChannelId: 1, BucketTs: end, BucketSeconds: 60, RequestCount: 100, SuccessCount: 100},
		{ModelName: "a", ChannelId: 2, BucketTs: start - 3600, BucketSeconds: 60, RequestCount: 100},
		{ModelName: "a", ChannelId: 3, BucketTs: start - 300, BucketSeconds: 3600, RequestCount: 100},
	}
	require.NoError(t, db.Create(&rows).Error)
	b := &atomicBucket{}
	b.add(Sample{Success: true, HasTtft: true, TtftMs: 500, CacheTokens: 50, PromptTokens: 100})
	channelHotBuckets.Store(channelBucketKey{model: "a", channelId: 1, bucketTs: start + 120}, b)
	result, err := QueryChannelSummaries(context.Background(), start, end, []int{1, 2, 3})
	require.NoError(t, err)
	require.EqualValues(t, 11, result[1].RequestCount)
	require.InDelta(t, 900.0/11, *result[1].SuccessRate, 0.001)
	require.Equal(t, 50.0, *result[1].CacheRate)
	require.Equal(t, 300.0, *result[1].AvgTtftMs)
	require.EqualValues(t, 5, result[1].TtftCount)
	require.Nil(t, result[2].SuccessRate)
	require.Nil(t, result[2].CacheRate)
	require.Nil(t, result[2].AvgTtftMs)
	require.True(t, result[3].LegacyResolution)
	// Flush moves the same counts to SQL; queries must not double-count.
	flushCompletedBuckets()
	after, err := QueryChannelSummaries(context.Background(), start, end, []int{1, 2, 3})
	require.NoError(t, err)
	require.Equal(t, result, after)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = QueryChannelSummaries(ctx, start, end, []int{1})
	require.ErrorIs(t, err, context.Canceled)
}

func TestChannelMetricsConcurrentRecordSnapshotFlush(t *testing.T) {
	setupChannelMetricsDB(t)
	ts := time.Now().Add(-2*time.Minute).Unix() / 60 * 60
	b := &atomicBucket{}
	channelHotBuckets.Store(channelBucketKey{model: "a", channelId: 1, bucketTs: ts}, b)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 300 {
			channelMetricsMu.RLock()
			b.add(Sample{Success: true, HasTtft: true, TtftMs: 7})
			channelMetricsMu.RUnlock()
		}
	}()
	for range 10 {
		flushCompletedBuckets()
		r, err := QueryChannelSummaries(context.Background(), ts, ts+60, []int{1})
		require.NoError(t, err)
		require.Equal(t, r[1].RequestCount, r[1].SuccessCount)
		require.Equal(t, r[1].RequestCount, r[1].TtftCount)
	}
	wg.Wait()
	r, err := QueryChannelSummaries(context.Background(), ts, ts+60, []int{1})
	require.NoError(t, err)
	require.EqualValues(t, 300, r[1].RequestCount)
}

func TestChannelMetricsRecordUsesMinuteBuckets(t *testing.T) {
	setupChannelMetricsDB(t)
	before := time.Now().Unix() / 60 * 60
	Record(Sample{Model: "test-minute", ChannelId: 81, Success: true})
	after := time.Now().Unix() / 60 * 60
	seen := false
	channelHotBuckets.Range(func(key, value any) bool {
		k := key.(channelBucketKey)
		require.EqualValues(t, 0, k.bucketTs%60)
		require.GreaterOrEqual(t, k.bucketTs, before)
		require.LessOrEqual(t, k.bucketTs, after)
		require.EqualValues(t, 1, value.(*atomicBucket).snapshot().requestCount)
		seen = true
		return true
	})
	require.True(t, seen)
}
