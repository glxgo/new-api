package model

import (
	"fmt"
	"strings"
	"testing"

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
	require.NoError(t, db.AutoMigrate(&Log{}, &UserSubscription{}))
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
			Quota: 100, PromptTokens: 100, CacheTokens: 30, CompletionTokens: 20,
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
