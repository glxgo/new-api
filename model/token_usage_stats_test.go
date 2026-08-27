package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTokenUsageStatsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB, oldLogDB := DB, LOG_DB
	oldSQLite, oldMySQL, oldPostgres := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = true, false, false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}, &UsageLogDailyAggregate{}))
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
