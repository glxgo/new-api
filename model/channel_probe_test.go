package model

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRecordChannelProbeRequiresStableFailureAndRecovery(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:channel_probe_state?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Channel{}, &ChannelProbeRecord{}, &ChannelProbeState{}))
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })
	require.NoError(t, db.Create(&Channel{Id: 8, Key: "local-only", Status: 1, Name: "probe-target"}).Error)

	base := time.Now().Unix()
	for attempt := 1; attempt <= 3; attempt++ {
		state, recordErr := RecordChannelProbe(&ChannelProbeRecord{
			ChannelId:     8,
			ProbeTs:       base + int64(attempt),
			Success:       false,
			ErrorCategory: "upstream",
		}, 3, 2)
		require.NoError(t, recordErr)
		if attempt < 3 {
			require.Equal(t, ChannelProbeStatusDegraded, state.Status)
		} else {
			require.Equal(t, ChannelProbeStatusUnhealthy, state.Status)
		}
		require.Equal(t, attempt, state.ConsecutiveFailures)
	}

	state, err := RecordChannelProbe(&ChannelProbeRecord{ChannelId: 8, ProbeTs: base + 4, Success: true}, 3, 2)
	require.NoError(t, err)
	require.Equal(t, ChannelProbeStatusChecking, state.Status)
	require.Equal(t, 1, state.ConsecutiveSuccesses)

	state, err = RecordChannelProbe(&ChannelProbeRecord{ChannelId: 8, ProbeTs: base + 5, Success: true}, 3, 2)
	require.NoError(t, err)
	require.Equal(t, ChannelProbeStatusHealthy, state.Status)
	require.Equal(t, 2, state.ConsecutiveSuccesses)
	require.Zero(t, state.ConsecutiveFailures)
	require.Empty(t, state.LastErrorCategory)
	var channel Channel
	require.NoError(t, db.First(&channel, 8).Error)
	require.Equal(t, 1, channel.Status, "probe state transitions must never disable the channel")
}

func TestChannelProbeRetentionDeletesOnlyExpiredHistory(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:channel_probe_retention?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ChannelProbeRecord{}, &ChannelProbeState{}))
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	require.NoError(t, db.Create(&[]ChannelProbeRecord{
		{ChannelId: 1, ProbeTs: 100, Success: true},
		{ChannelId: 1, ProbeTs: 200, Success: false},
	}).Error)
	require.NoError(t, DeleteChannelProbeRecordsBefore(150))
	records, err := GetChannelProbeRecordsSince(0)
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.EqualValues(t, 200, records[0].ProbeTs)
}
