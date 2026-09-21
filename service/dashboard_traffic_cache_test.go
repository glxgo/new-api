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

func setupDashboardTrafficCacheTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldSQLite, oldMySQL, oldPostgres := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	oldRedisEnabled, oldRDB := common.RedisEnabled, common.RDB
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = true, false, false
	common.RedisEnabled, common.RDB = false, nil
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	model.DB, model.LOG_DB = db, db
	dashboardTrafficHotCache.Purge()
	dashboardTrafficStaleCache.Purge()
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = oldSQLite, oldMySQL, oldPostgres
		common.RedisEnabled, common.RDB = oldRedisEnabled, oldRDB
		dashboardTrafficHotCache.Purge()
		dashboardTrafficStaleCache.Purge()
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestGetDashboardTrafficResultCachesFinalAggregate(t *testing.T) {
	db := setupDashboardTrafficCacheTestDB(t)
	start := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC).Unix()
	end := start + 3600
	require.NoError(t, db.Create(&model.Log{
		UserId: 17, ChannelId: 9, CreatedAt: start + 60,
		Type: model.LogTypeConsume, UseTime: 2, Quota: 120, Cost: 40,
	}).Error)

	first, err := GetDashboardTrafficResultWithContext(context.Background(), 17, start, end, time.UTC, false)
	require.NoError(t, err)
	require.EqualValues(t, 1, first.Summary.RequestCount)
	require.EqualValues(t, 120, first.Summary.BilledQuota)

	// The final aggregate is cached; changing the source row does not alter a
	// same-key read during the short hot-cache window.
	require.NoError(t, db.Model(&model.Log{}).Where("user_id = ?", 17).Updates(map[string]interface{}{"quota": 999}).Error)
	second, err := GetDashboardTrafficResultWithContext(context.Background(), 17, start, end, time.UTC, false)
	require.NoError(t, err)
	require.EqualValues(t, 120, second.Summary.BilledQuota)
}
