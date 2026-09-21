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

func TestMigrateUsageMetricSchemaIsExplicitAndCreatesAllProjectionTables(t *testing.T) {
	oldDB, oldLogDB := DB, LOG_DB
	oldSQLite, oldMySQL, oldPostgres := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = true, false, false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB, LOG_DB = db, db
	t.Cleanup(func() {
		DB, LOG_DB = oldDB, oldLogDB
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = oldSQLite, oldMySQL, oldPostgres
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	// Normal startup is feature-off and must not create the projection table.
	t.Setenv(UsageMetricSchemaMigrationEnv, "false")
	require.NoError(t, MaybeMigrateUsageMetricSchema(context.Background()))
	require.False(t, db.Migrator().HasTable(&UsageMetricBucket{}))

	require.NoError(t, MigrateUsageMetricSchema(context.Background(), db))
	require.True(t, db.Migrator().HasColumn(&UsageLogDailyAggregate{}, "last_log_id"))
	require.True(t, db.Migrator().HasTable(&UsageMetricBucket{}))
	require.True(t, db.Migrator().HasTable(&UsageMetricCheckpoint{}))
	require.True(t, db.Migrator().HasTable(&UsageMetricBatch{}))
	require.True(t, db.Migrator().HasTable(&UsageMetricCoverage{}))
	require.True(t, db.Migrator().HasTable(&WalletConsumeDailyAggregate{}))
	require.True(t, db.Migrator().HasTable(&WalletConsumeDailyAggregateCoverage{}))
	require.True(t, db.Migrator().HasTable(&WalletConsumeAggregateCheckpoint{}))
}
