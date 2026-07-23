package model

import (
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
