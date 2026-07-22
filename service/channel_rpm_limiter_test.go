package service

import (
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func withLocalChannelCapacity(t *testing.T) {
	t.Helper()
	oldRedisEnabled, oldRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = false, nil
	resetLocalChannelRPMForTest()
	resetLocalConcurrencyForTest()
	t.Cleanup(func() {
		resetLocalChannelRPMForTest()
		resetLocalConcurrencyForTest()
		common.RedisEnabled, common.RDB = oldRedisEnabled, oldRDB
	})
}

func TestLocalChannelRPMLimitAndRollingExpiry(t *testing.T) {
	withLocalChannelCapacity(t)
	start := time.Unix(2_000, 0)
	for i := 0; i < 3; i++ {
		allowed, current := acquireLocalChannelRPM(8, 3, start.Add(time.Duration(i)*time.Second))
		require.True(t, allowed)
		require.Equal(t, i+1, current)
	}
	allowed, current := acquireLocalChannelRPM(8, 3, start.Add(59*time.Second))
	require.False(t, allowed)
	require.Equal(t, 3, current)

	allowed, current = acquireLocalChannelRPM(8, 3, start.Add(61*time.Second))
	require.True(t, allowed)
	require.Equal(t, 2, current)
}

func TestLocalChannelRPMAcquireIsAtomic(t *testing.T) {
	withLocalChannelCapacity(t)
	const limit = 10
	var wg sync.WaitGroup
	results := make(chan bool, 64)
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, _ := AcquireChannelRPM(31, limit)
			results <- allowed
		}()
	}
	wg.Wait()
	close(results)
	accepted := 0
	for allowed := range results {
		if allowed {
			accepted++
		}
	}
	require.Equal(t, limit, accepted)
}

func TestAcquireChannelCapacityUsesEitherLimitAndReleasesRejectedLease(t *testing.T) {
	withLocalChannelCapacity(t)

	firstLease, acquired, reason := AcquireChannelCapacity(32, 1, 1)
	require.True(t, acquired)
	require.Equal(t, ChannelCapacityAvailable, reason)
	require.Equal(t, 1, GetChannelConcurrency(32))

	_, acquired, reason = AcquireChannelCapacity(32, 1, 1)
	require.False(t, acquired)
	require.Equal(t, ChannelCapacityConcurrencyFull, reason)

	firstLease.Release()
	require.Equal(t, 0, GetChannelConcurrency(32))

	_, acquired, reason = AcquireChannelCapacity(32, 1, 1)
	require.False(t, acquired)
	require.Equal(t, ChannelCapacityRPMFull, reason)
	require.Equal(t, 0, GetChannelConcurrency(32), "an RPM rejection must release its concurrency lease")
}

func TestAcquireChannelCapacityZeroLimitsAreUnlimited(t *testing.T) {
	withLocalChannelCapacity(t)
	for i := 0; i < 100; i++ {
		lease, acquired, reason := AcquireChannelCapacity(8, 0, 0)
		require.True(t, acquired)
		require.Equal(t, ChannelCapacityAvailable, reason)
		lease.Release()
	}
}
