package service

import (
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func withLocalUserRPMLimiter(t *testing.T) {
	t.Helper()
	oldRedisEnabled, oldRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = false, nil
	resetLocalUserRPMForTest()
	t.Cleanup(func() {
		resetLocalUserRPMForTest()
		common.RedisEnabled, common.RDB = oldRedisEnabled, oldRDB
	})
}

func TestUserRPMLimitRoundsHalfUp(t *testing.T) {
	require.Equal(t, 0, UserRPMLimit(0))
	require.Equal(t, 2, UserRPMLimit(1))
	require.Equal(t, 3, UserRPMLimit(2))
	require.Equal(t, 5, UserRPMLimit(3))
	require.Equal(t, 12, UserRPMLimit(8))
}

func TestLocalUserRPMLimitAndRollingExpiry(t *testing.T) {
	withLocalUserRPMLimiter(t)
	start := time.Unix(1_000, 0)
	for i := 0; i < 3; i++ {
		allowed, current := acquireLocalUserRPM(7, 3, start.Add(time.Duration(i)*time.Second))
		require.True(t, allowed)
		require.Equal(t, i+1, current)
	}
	allowed, current := acquireLocalUserRPM(7, 3, start.Add(59*time.Second))
	require.False(t, allowed)
	require.Equal(t, 3, current)

	allowed, current = acquireLocalUserRPM(7, 3, start.Add(61*time.Second))
	require.True(t, allowed)
	require.Equal(t, 2, current, "only requests still inside the rolling minute should remain")
}

func TestLocalUserRPMAcquireIsAtomic(t *testing.T) {
	withLocalUserRPMLimiter(t)
	const limit = 12
	var wg sync.WaitGroup
	results := make(chan bool, 64)
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, _ := AcquireUserRPM(9, limit)
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
