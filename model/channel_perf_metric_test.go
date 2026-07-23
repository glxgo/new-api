package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpsertAndQueryChannelPerfMetric(t *testing.T) {
	previousDB := DB
	db, err := gorm.Open(sqlite.Open("file:channel_perf_metric?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	t.Cleanup(func() { DB = previousDB })
	require.NoError(t, DB.AutoMigrate(&ChannelPerfMetric{}))

	first := &ChannelPerfMetric{
		ModelName:      "gpt-5",
		ChannelId:      8,
		BucketTs:       100,
		RequestCount:   2,
		SuccessCount:   1,
		TotalLatencyMs: 500,
	}
	second := &ChannelPerfMetric{
		ModelName:      "gpt-5",
		ChannelId:      8,
		BucketTs:       100,
		RequestCount:   3,
		SuccessCount:   3,
		TotalLatencyMs: 900,
	}
	require.NoError(t, UpsertChannelPerfMetric(first))
	require.NoError(t, UpsertChannelPerfMetric(second))

	rows, err := GetChannelPerfMetrics(90, 110, []int{8})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.EqualValues(t, 5, rows[0].RequestCount)
	require.EqualValues(t, 4, rows[0].SuccessCount)
	require.EqualValues(t, 1400, rows[0].TotalLatencyMs)
}
