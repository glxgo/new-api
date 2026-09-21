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

func setupWalletConsumeAggregateWorkerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldMaster := common.IsMasterNode
	oldSQLite, oldMySQL, oldPostgres := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	common.IsMasterNode = true
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = true, false, false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Log{},
		&model.UsageLogDailyAggregate{},
		&model.WalletConsumeDailyAggregate{},
		&model.WalletConsumeDailyAggregateCoverage{},
		&model.WalletConsumeAggregateCheckpoint{},
	))
	model.DB, model.LOG_DB = db, db
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.IsMasterNode = oldMaster
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = oldSQLite, oldMySQL, oldPostgres
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestRunWalletConsumeAggregatePersistsCheckpointAfterClosedDay(t *testing.T) {
	db := setupWalletConsumeAggregateWorkerTestDB(t)
	t.Setenv(model.WalletConsumeAggregateWorkerEnableEnv, "true")

	now := time.Now().Unix()
	currentDay := walletConsumeDayStartForTask(now)
	previousDay := previousWalletDay(currentDay)
	balance := int64(77)
	require.NoError(t, db.Create(&model.Log{
		Id: 501, UserId: 51, Type: model.LogTypeConsume,
		CreatedAt: previousDay + 10, Quota: 12, BalanceAfter: &balance,
	}).Error)
	// Start one day behind the open day and mark reconciliation as recently
	// completed. The run should process exactly the closed day, persist the
	// cursor, and leave the open day untouched.
	require.NoError(t, db.Create(&model.WalletConsumeAggregateCheckpoint{
		Name:             "wallet-consume-daily-v2",
		NextDayStart:     previousDay,
		LastReconciledAt: now,
		UpdatedAt:        now,
	}).Error)

	require.NoError(t, RunWalletConsumeAggregateOnce(context.Background()))

	var checkpoint model.WalletConsumeAggregateCheckpoint
	require.NoError(t, db.Where("name = ?", "wallet-consume-daily-v2").First(&checkpoint).Error)
	require.Equal(t, currentDay, checkpoint.NextDayStart)
	require.Equal(t, now, checkpoint.LastReconciledAt)
	var aggregate model.WalletConsumeDailyAggregate
	require.NoError(t, db.Where("user_id = ? AND day_start = ?", 51, previousDay).First(&aggregate).Error)
	require.EqualValues(t, 12, aggregate.Quota)
	var coverage model.WalletConsumeDailyAggregateCoverage
	require.NoError(t, db.Where("day_start = ?", previousDay).First(&coverage).Error)
	require.True(t, coverage.IsComplete)
	var openCoverageCount int64
	require.NoError(t, db.Model(&model.WalletConsumeDailyAggregateCoverage{}).Where("day_start = ?", currentDay).Count(&openCoverageCount).Error)
	require.Zero(t, openCoverageCount)
}

func TestRunWalletConsumeAggregateCancellationDoesNotAdvanceCheckpoint(t *testing.T) {
	db := setupWalletConsumeAggregateWorkerTestDB(t)
	t.Setenv(model.WalletConsumeAggregateWorkerEnableEnv, "true")

	now := time.Now().Unix()
	currentDay := walletConsumeDayStartForTask(now)
	previousDay := previousWalletDay(currentDay)
	require.NoError(t, db.Create(&model.WalletConsumeAggregateCheckpoint{
		Name:             "wallet-consume-daily-v2",
		NextDayStart:     previousDay,
		LastReconciledAt: now,
		UpdatedAt:        now,
	}).Error)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := RunWalletConsumeAggregateOnce(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
	var checkpoint model.WalletConsumeAggregateCheckpoint
	require.NoError(t, db.Where("name = ?", "wallet-consume-daily-v2").First(&checkpoint).Error)
	require.Equal(t, previousDay, checkpoint.NextDayStart)
	var coverageCount int64
	require.NoError(t, db.Model(&model.WalletConsumeDailyAggregateCoverage{}).Count(&coverageCount).Error)
	require.Zero(t, coverageCount)
}
