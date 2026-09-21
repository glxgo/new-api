package model

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTokenUsageStatsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB, oldLogDB := DB, LOG_DB
	oldSQLite, oldMySQL, oldPostgres := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	oldRedisEnabled, oldRDB := common.RedisEnabled, common.RDB
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = true, false, false
	// Keep these unit tests deterministic and independent of any developer
	// Redis instance; the production path still exercises Redis when enabled.
	common.RedisEnabled, common.RDB = false, nil
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}, &UsageLogDailyAggregate{}))
	DB, LOG_DB = db, db
	tokenUsageStatsHotCache.Purge()
	tokenUsageStatsStaleCache.Purge()
	t.Cleanup(func() {
		DB, LOG_DB = oldDB, oldLogDB
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = oldSQLite, oldMySQL, oldPostgres
		common.RedisEnabled, common.RDB = oldRedisEnabled, oldRDB
		tokenUsageStatsHotCache.Purge()
		tokenUsageStatsStaleCache.Purge()
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestGetTokenUsageStatsCombinesSettledLiveAndArchivedRows(t *testing.T) {
	db := setupTokenUsageStatsTestDB(t)
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Date(2026, 8, 25, 15, 30, 0, 0, location)
	todayStart, _ := chinaDayRangeForTokenUsage(now)
	require.NoError(t, db.Create(&[]Log{
		{TokenId: 7, Type: LogTypeConsume, Settled: true, CreatedAt: todayStart + 60, Quota: 120},
		{TokenId: 7, Type: LogTypeConsume, Settled: true, CreatedAt: todayStart - 86400, Quota: 80},
		{TokenId: 7, Type: LogTypeConsume, Settled: false, CreatedAt: todayStart + 90, Quota: 999},
		{TokenId: 8, Type: LogTypeConsume, Settled: true, CreatedAt: todayStart + 120, Quota: 30},
	}).Error)
	require.NoError(t, db.Create(&UsageLogDailyAggregate{
		TokenId: 7, Type: LogTypeConsume, BucketStart: todayStart - 2*86400, Quota: 300,
	}).Error)

	stats, err := GetTokenUsageStats([]int{7, 8}, now)
	require.NoError(t, err)
	require.EqualValues(t, 120, stats[7].TodayUsedQuota)
	require.EqualValues(t, 500, stats[7].LifetimeUsedQuota)
	require.EqualValues(t, 30, stats[8].TodayUsedQuota)
	require.EqualValues(t, 30, stats[8].LifetimeUsedQuota)
}

func TestGetTokenUsageStatsInRangeUsesSelectedWindow(t *testing.T) {
	db := setupTokenUsageStatsTestDB(t)
	const start int64 = 1_000
	const end int64 = 2_000
	require.NoError(t, db.Create(&[]Log{
		{TokenId: 7, Type: LogTypeConsume, Settled: true, CreatedAt: 999, Quota: 10},
		{TokenId: 7, Type: LogTypeConsume, Settled: true, CreatedAt: 1_100, Quota: 20},
		{TokenId: 7, Type: LogTypeConsume, Settled: true, CreatedAt: 2_000, Quota: 30},
	}).Error)
	stats, err := GetTokenUsageStatsInRange(context.Background(), []int{7}, start, end)
	require.NoError(t, err)
	require.EqualValues(t, 20, stats[7].RangeUsedQuota)
}

func TestGetTokenUsageStatsUsesLocalCacheAndCanonicalTokenOrder(t *testing.T) {
	db := setupTokenUsageStatsTestDB(t)
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Date(2026, 8, 25, 15, 30, 7, 0, location)
	dayStart, _ := chinaDayRangeForTokenUsage(now)
	require.NoError(t, db.Create(&Log{TokenId: 101, Type: LogTypeConsume, Settled: true, CreatedAt: dayStart + 60, Quota: 10}).Error)

	first, err := GetTokenUsageStats([]int{101}, now)
	require.NoError(t, err)
	require.EqualValues(t, 10, first[101].TodayUsedQuota)
	require.EqualValues(t, 10, first[101].LifetimeUsedQuota)
	first[101] = TokenUsageStats{TodayUsedQuota: 999, LifetimeUsedQuota: 999}

	// A same-window read is served locally even if the source row changes;
	// this bounds repeated API-key polling to one grouped DB read per TTL.
	require.NoError(t, db.Model(&Log{}).Where("token_id = ?", 101).Updates(map[string]interface{}{"quota": 99}).Error)
	second, err := GetTokenUsageStats([]int{101}, now)
	require.NoError(t, err)
	require.EqualValues(t, 10, second[101].TodayUsedQuota)
	require.EqualValues(t, 10, second[101].LifetimeUsedQuota)

	// Token ID order is not part of the cache identity.
	require.NoError(t, db.Create(&Log{TokenId: 102, Type: LogTypeConsume, Settled: true, CreatedAt: dayStart + 70, Quota: 20}).Error)
	ordered, err := GetTokenUsageStats([]int{102, 101}, now)
	require.NoError(t, err)
	// The changed token remains cached; the newly requested token is not in the
	// previous key, so this call is a fresh query containing both IDs.
	require.EqualValues(t, 99, ordered[101].LifetimeUsedQuota)
	require.EqualValues(t, 20, ordered[102].LifetimeUsedQuota)
}

func TestGetTokenUsageStatsReturnsStaleSnapshotOnDatabaseFailure(t *testing.T) {
	db := setupTokenUsageStatsTestDB(t)
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Date(2026, 8, 25, 15, 30, 0, 0, location)
	dayStart, _ := chinaDayRangeForTokenUsage(now)
	require.NoError(t, db.Create(&Log{TokenId: 202, Type: LogTypeConsume, Settled: true, CreatedAt: dayStart + 60, Quota: 22}).Error)
	fresh, err := GetTokenUsageStats([]int{202}, now)
	require.NoError(t, err)
	require.False(t, fresh[202].Stale)

	// Force a miss in the hot cache while retaining the longer stale snapshot.
	tokenUsageStatsHotCache.Purge()
	sqlDB, dbErr := db.DB()
	require.NoError(t, dbErr)
	require.NoError(t, sqlDB.Close())
	stale, err := GetTokenUsageStats([]int{202}, now)
	require.NoError(t, err)
	require.True(t, stale[202].Stale)
	require.EqualValues(t, 22, stale[202].LifetimeUsedQuota)

	tokenUsageStatsHotCache.Purge()
	tokenUsageStatsStaleCache.Purge()
	oldLogDB := LOG_DB
	LOG_DB = nil
	_, err = GetTokenUsageStats([]int{202}, now)
	LOG_DB = oldLogDB
	require.ErrorContains(t, err, "log database is unavailable")
}

func TestGetTokenUsageStatsUsesRedisCacheWhenConfigured(t *testing.T) {
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("set TEST_REDIS_URL to run token usage Redis integration test")
	}
	options, err := redis.ParseURL(redisURL)
	require.NoError(t, err)
	client := redis.NewClient(options)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	require.NoError(t, client.Ping(ctx).Err())
	cancel()

	db := setupTokenUsageStatsTestDB(t)
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Date(2026, 8, 25, 15, 30, 0, 0, location)
	dayStart, _ := chinaDayRangeForTokenUsage(now)
	tokenID := int(time.Now().UnixNano() & 0x3fffffff)
	require.NoError(t, db.Create(&Log{TokenId: tokenID, Type: LogTypeConsume, Settled: true, CreatedAt: dayStart + 60, Quota: 31}).Error)

	oldRedisEnabled, oldRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = true, client
	_, remoteKey := tokenUsageStatsCacheKeys([]int{tokenID}, now)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = client.Del(cleanupCtx, remoteKey, remoteKey+":stale").Err()
		cleanupCancel()
		common.RedisEnabled, common.RDB = oldRedisEnabled, oldRDB
		_ = client.Close()
	})

	fresh, err := GetTokenUsageStats([]int{tokenID}, now)
	require.NoError(t, err)
	require.EqualValues(t, 31, fresh[tokenID].LifetimeUsedQuota)
	require.Eventually(t, func() bool {
		return client.Exists(context.Background(), remoteKey).Val() == 1
	}, time.Second, 10*time.Millisecond)

	// Drop the process-local entry; the next read should come from Redis even
	// though the source row has changed.
	require.NoError(t, db.Model(&Log{}).Where("token_id = ?", tokenID).Updates(map[string]interface{}{"quota": 99}).Error)
	tokenUsageStatsHotCache.Purge()
	fromRedis, err := GetTokenUsageStats([]int{tokenID}, now)
	require.NoError(t, err)
	require.EqualValues(t, 31, fromRedis[tokenID].LifetimeUsedQuota)
}
