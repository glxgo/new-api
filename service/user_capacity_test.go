package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestLocalUserCapacitySnapshots(t *testing.T) {
	oldRedisEnabled, oldRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = false, nil
	t.Cleanup(func() {
		resetLocalConcurrencyForTest()
		resetLocalUserRPMForTest()
		common.RedisEnabled, common.RDB = oldRedisEnabled, oldRDB
	})
	resetLocalConcurrencyForTest()
	resetLocalUserRPMForTest()

	first, ok, _ := AcquireUserConcurrencyWithCount(23, 8)
	require.True(t, ok)
	second, ok, _ := AcquireUserConcurrencyWithCount(23, 8)
	require.True(t, ok)
	t.Cleanup(first.Release)
	t.Cleanup(second.Release)

	for expected := 1; expected <= 3; expected++ {
		acquired, current := AcquireUserRPM(23, 12)
		require.True(t, acquired)
		require.Equal(t, expected, current)
	}

	snapshots := GetUserCapacitySnapshots([]int{23, 23, 0, -1, 99})
	require.Equal(t, UserCapacitySnapshot{CurrentConcurrency: 2, CurrentRPM: 3}, snapshots[23])
	require.Equal(t, UserCapacitySnapshot{}, snapshots[99])
	require.NotContains(t, snapshots, 0)
	require.NotContains(t, snapshots, -1)
}
