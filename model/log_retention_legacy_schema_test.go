package model

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// legacyUsageLogDailyAggregate is the pre-v2 archive shape.  Keeping this
// fixture separate from UsageLogDailyAggregate makes the test fail if a
// normal startup path accidentally starts requiring last_log_id.
type legacyUsageLogDailyAggregate struct {
	Id                    int    `gorm:"primaryKey"`
	BucketStart           int64  `gorm:"type:bigint;not null;index:idx_usage_aggregate_bucket_user,priority:1"`
	UserId                int    `gorm:"not null;index:idx_usage_aggregate_bucket_user,priority:2;index"`
	Type                  int    `gorm:"not null;index"`
	ModelName             string `gorm:"type:varchar(191);not null;default:'';index"`
	ChannelId             int    `gorm:"not null;default:0;index"`
	TokenId               int    `gorm:"not null;default:0;index"`
	GroupName             string `gorm:"column:group_name;type:varchar(64);not null;default:'';index"`
	BillingSource         string `gorm:"type:varchar(32);not null;default:'';index"`
	SubscriptionId        int    `gorm:"not null;default:0;index"`
	RequestCount          int64  `gorm:"type:bigint;not null;default:0"`
	StreamCount           int64  `gorm:"type:bigint;not null;default:0"`
	Quota                 int64  `gorm:"type:bigint;not null;default:0"`
	PreDiscountQuota      int64  `gorm:"type:bigint;not null;default:0"`
	PromptTokens          int64  `gorm:"type:bigint;not null;default:0"`
	CacheTokens           int64  `gorm:"type:bigint;not null;default:0"`
	EffectivePromptTokens int64  `gorm:"type:bigint;not null;default:0"`
	CompletionTokens      int64  `gorm:"type:bigint;not null;default:0"`
	UseTime               int64  `gorm:"type:bigint;not null;default:0"`
	Cost                  int64  `gorm:"type:bigint;not null;default:0"`
	PaidQuota             int64  `gorm:"type:bigint;not null;default:0"`
	PaidGiftQuota         int64  `gorm:"type:bigint;not null;default:0"`
	BalanceAfter          *int64 `gorm:"default:null"`
	FirstLogAt            int64  `gorm:"type:bigint;not null"`
	LastLogAt             int64  `gorm:"type:bigint;not null"`
	CreatedAt             int64  `gorm:"type:bigint;not null"`
}

func (legacyUsageLogDailyAggregate) TableName() string {
	return "usage_log_daily_aggregates"
}

func openLegacyArchiveSchemaTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:legacy-archive-schema-%s?mode=memory&cache=shared", t.Name())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}, &legacyUsageLogDailyAggregate{}))
	previousDB, previousLogDB := DB, LOG_DB
	DB, LOG_DB = db, db
	return db, func() {
		DB, LOG_DB = previousDB, previousLogDB
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	}
}

func TestLegacyArchiveSchemaSkipsImplicitLastLogIDMigrationAndRetentionStillWrites(t *testing.T) {
	db, restore := openLegacyArchiveSchemaTestDB(t)
	defer restore()
	t.Setenv(UsageMetricSchemaMigrationEnv, "false")

	require.False(t, db.Migrator().HasColumn(&legacyUsageLogDailyAggregate{}, "last_log_id"))
	require.NoError(t, migrateUsageLogDailyAggregate(db))
	// Ordinary startup must not ALTER an existing pre-v2 archive table.
	require.False(t, db.Migrator().HasColumn(&legacyUsageLogDailyAggregate{}, "last_log_id"))

	balance := int64(123)
	require.NoError(t, db.Create(&Log{
		Id: 901, UserId: 7, Type: LogTypeConsume, Settled: true,
		CreatedAt: 100, Quota: 42, BalanceAfter: &balance,
	}).Error)
	archived, deleted, err := ArchiveDetailedUsageLogs(context.Background(), 200, 100)
	require.NoError(t, err)
	require.EqualValues(t, 1, archived)
	require.EqualValues(t, 1, deleted)
	require.False(t, db.Migrator().HasColumn(&legacyUsageLogDailyAggregate{}, "last_log_id"))
	var archiveCount int64
	require.NoError(t, db.Model(&legacyUsageLogDailyAggregate{}).Count(&archiveCount).Error)
	require.EqualValues(t, 1, archiveCount)
}

func TestExplicitArchiveSchemaMigrationAddsLastLogIDAndRetentionPersistsSourceID(t *testing.T) {
	db, restore := openLegacyArchiveSchemaTestDB(t)
	defer restore()

	// First exercise the legacy writer so the schema probe cache contains false.
	t.Setenv(UsageMetricSchemaMigrationEnv, "false")
	require.NoError(t, db.Create(&Log{
		Id: 911, UserId: 8, Type: LogTypeConsume, Settled: true,
		CreatedAt: 100, Quota: 10,
	}).Error)
	_, _, err := ArchiveDetailedUsageLogs(context.Background(), 200, 100)
	require.NoError(t, err)
	require.False(t, db.Migrator().HasColumn(&legacyUsageLogDailyAggregate{}, "last_log_id"))

	// Explicit migration is allowed to add the source boundary metadata.
	t.Setenv(UsageMetricSchemaMigrationEnv, "true")
	require.NoError(t, migrateUsageLogDailyAggregate(db))
	require.True(t, db.Migrator().HasColumn(&UsageLogDailyAggregate{}, "last_log_id"))

	require.NoError(t, db.Create(&Log{
		Id: 912, UserId: 8, Type: LogTypeConsume, Settled: true,
		CreatedAt: 101, Quota: 20,
	}).Error)
	_, _, err = ArchiveDetailedUsageLogs(context.Background(), 200, 100)
	require.NoError(t, err)
	var row UsageLogDailyAggregate
	require.NoError(t, db.Where("user_id = ? AND last_log_at = ?", 8, 101).First(&row).Error)
	require.EqualValues(t, 912, row.LastLogId)
}
