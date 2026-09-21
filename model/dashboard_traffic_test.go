package model

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetDashboardTrafficRecordsUsesSuccessfulConsumeLogsOnly(t *testing.T) {
	truncateTables(t)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    11,
		CreatedAt: 100,
		Type:      LogTypeConsume,
		ChannelId: 8,
		UseTime:   3,
		Quota:     120,
		Cost:      50,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    11,
		CreatedAt: 101,
		Type:      LogTypeError,
		ChannelId: 8,
	}).Error)
	require.NoError(t, LOG_DB.Create(&Log{
		UserId:    12,
		CreatedAt: 102,
		Type:      LogTypeConsume,
		ChannelId: 31,
	}).Error)

	records, err := GetDashboardTrafficRecords(11, 90, 110)
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, 11, records[0].UserId)
	require.Equal(t, 8, records[0].ChannelId)
	require.Equal(t, 120, records[0].Quota)
	require.Equal(t, 50, records[0].Cost)
}

func TestDashboardTrafficIndexesSupportAdminAndUserRanges(t *testing.T) {
	// Composite query indexes are an explicit maintenance migration rather than
	// an implicit part of Log AutoMigrate on a large production table.
	require.NoError(t, MigrateLogQueryIndexes(context.Background(), LOG_DB))
	require.True(t, LOG_DB.Migrator().HasIndex(&Log{}, "idx_type_created_at"))
	require.True(t, LOG_DB.Migrator().HasIndex(&Log{}, "idx_user_type_created_at"))
	require.True(t, LOG_DB.Migrator().HasIndex(&Log{}, "idx_username_created_at"))
	require.True(t, LOG_DB.Migrator().HasIndex(&Log{}, "idx_username_type_created_at"))
}

func TestDashboardTrafficQueryHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := queryDashboardTrafficRecords(ctx, 11, 90, 110, 0, 0, false)
	require.ErrorIs(t, err, context.Canceled)
}

func TestDashboardTrafficUnionFiltersBothSourcesByUser(t *testing.T) {
	truncateTables(t)
	require.NoError(t, LOG_DB.AutoMigrate(&UsageLogDailyAggregate{}))
	require.NoError(t, LOG_DB.Where("1 = 1").Delete(&UsageLogDailyAggregate{}).Error)
	for _, userID := range []int{11, 12} {
		require.NoError(t, LOG_DB.Create(&Log{UserId: userID, Type: LogTypeConsume, CreatedAt: 110, Quota: userID}).Error)
		require.NoError(t, LOG_DB.Create(&UsageLogDailyAggregate{UserId: userID, Type: LogTypeConsume, BucketStart: 90, LastLogAt: 95, RequestCount: 3, Quota: int64(userID * 3)}).Error)
	}
	rows, err := queryDashboardTrafficRecords(context.Background(), 11, 90, 120, 90, 100, true)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.Equal(t, 11, row.UserId)
	}
	all, err := queryDashboardTrafficRecords(context.Background(), 0, 90, 120, 90, 100, true)
	require.NoError(t, err)
	require.Len(t, all, 4)
}
