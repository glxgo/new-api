package model

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUsageLogRetentionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB, oldLogDB := DB, LOG_DB
	oldSQLite, oldMySQL, oldPostgres := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = true, false, false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}, &UsageLogDailyAggregate{}, &UserSubscription{}))
	DB, LOG_DB = db, db
	t.Cleanup(func() {
		DB, LOG_DB = oldDB, oldLogDB
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = oldSQLite, oldMySQL, oldPostgres
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestArchiveDetailedUsageLogsRetainsAggregatesAndAuditRows(t *testing.T) {
	db := setupUsageLogRetentionTestDB(t)
	cutoff := int64(8 * 86400)
	logs := []Log{
		{UserId: 1, CreatedAt: 100, Type: LogTypeConsume, ModelName: "gpt-test", ChannelId: 7, TokenId: 9, Group: "wallet-a", BillingSource: "wallet", Quota: 120, PreDiscountQuota: 240, PromptTokens: 30, CacheTokens: 10, CompletionTokens: 5, Cost: 60, Settled: true},
		{UserId: 1, CreatedAt: 200, Type: LogTypeError, ModelName: "gpt-test", ChannelId: 7, TokenId: 9, Group: "wallet-a"},
		{UserId: 1, CreatedAt: 300, Type: LogTypeConsume, ModelName: "legacy", Quota: 50, Settled: false},
		{UserId: 1, CreatedAt: 400, Type: LogTypeTopup, Content: "audit"},
		{UserId: 1, CreatedAt: cutoff + 1, Type: LogTypeConsume, ModelName: "recent", Quota: 99, Settled: true},
	}
	require.NoError(t, db.Create(&logs).Error)

	archived, deleted, err := ArchiveDetailedUsageLogs(context.Background(), cutoff, 100)
	require.NoError(t, err)
	require.EqualValues(t, 2, archived)
	require.EqualValues(t, 2, deleted)

	var remaining []Log
	require.NoError(t, db.Order("id asc").Find(&remaining).Error)
	require.Len(t, remaining, 3)
	require.Equal(t, "legacy", remaining[0].ModelName, "unsettled legacy financial rows must remain auditable")
	require.Equal(t, LogTypeTopup, remaining[1].Type)
	require.Equal(t, "recent", remaining[2].ModelName)

	var aggregates []UsageLogDailyAggregate
	require.NoError(t, db.Order("type asc").Find(&aggregates).Error)
	require.Len(t, aggregates, 2)
	var requestCount, quota, preDiscountQuota, prompt, cache, completion, cost int64
	for _, item := range aggregates {
		requestCount += item.RequestCount
		quota += item.Quota
		preDiscountQuota += item.PreDiscountQuota
		prompt += item.PromptTokens
		cache += item.CacheTokens
		completion += item.CompletionTokens
		cost += item.Cost
	}
	require.EqualValues(t, 2, requestCount)
	require.EqualValues(t, 120, quota)
	require.EqualValues(t, 240, preDiscountQuota)
	require.EqualValues(t, 30, prompt)
	require.EqualValues(t, 10, cache)
	require.EqualValues(t, 5, completion)
	require.EqualValues(t, 60, cost)
}

func TestUsageStatisticsIncludesArchivedDailyAggregates(t *testing.T) {
	db := setupUsageLogRetentionTestDB(t)
	require.NoError(t, db.Create(&UsageLogDailyAggregate{
		BucketStart: 0, UserId: 1, Type: LogTypeConsume, ModelName: "archived-model",
		BillingSource: "subscription", SubscriptionId: 10, RequestCount: 2,
		Quota: 300, PromptTokens: 80, CacheTokens: 20, EffectivePromptTokens: 80, CompletionTokens: 10,
	}).Error)
	require.NoError(t, db.Create(&UserSubscription{Id: 10, UserId: 1, PlanTitle: "Archived plan", Status: "active"}).Error)

	stats, err := GetUserUsageStatistics(1, 1, 86400, 3600)
	require.NoError(t, err)
	require.EqualValues(t, 2, stats.Summary.RequestCount)
	require.EqualValues(t, 300, stats.Summary.Quota)
	require.EqualValues(t, 2, stats.Models[0].RequestCount)
	require.EqualValues(t, 2, stats.Subscriptions[0].RequestCount)
}
