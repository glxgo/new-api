package service

import (
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestLocalConcurrencyLimitAndRelease(t *testing.T) {
	oldRedisEnabled, oldRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = false, nil
	defer func() { common.RedisEnabled, common.RDB = oldRedisEnabled, oldRDB }()
	resetLocalConcurrencyForTest()

	first, ok := AcquireUserConcurrency(7, 1)
	require.True(t, ok)
	_, ok = AcquireUserConcurrency(7, 1)
	require.False(t, ok)
	first.Release()
	second, ok := AcquireUserConcurrency(7, 1)
	require.True(t, ok)
	second.Release()
}

func TestLocalUserConcurrencyReturnsAdmissionCount(t *testing.T) {
	oldRedisEnabled, oldRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = false, nil
	defer func() { common.RedisEnabled, common.RDB = oldRedisEnabled, oldRDB }()
	resetLocalConcurrencyForTest()

	first, ok, current := AcquireUserConcurrencyWithCount(17, 2)
	require.True(t, ok)
	require.Equal(t, 1, current)
	t.Cleanup(first.Release)

	second, ok, current := AcquireUserConcurrencyWithCount(17, 2)
	require.True(t, ok)
	require.Equal(t, 2, current)
	t.Cleanup(second.Release)

	third, ok, current := AcquireUserConcurrencyWithCount(17, 2)
	require.False(t, ok)
	require.Nil(t, third)
	require.Equal(t, 2, current)
}

func TestUnlimitedUserConcurrencyStillCountsAndReleases(t *testing.T) {
	oldEnabled, oldRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = false, nil
	t.Cleanup(func() { common.RedisEnabled, common.RDB = oldEnabled, oldRDB })
	resetLocalConcurrencyForTest()
	for _, key := range []string{UserConcurrencyKey(71), VirtualMembershipConcurrencyKey(71, 9)} {
		first, ok, count := AcquireConcurrencyWithCountByKey(key, 0)
		t.Cleanup(first.Release)
		require.True(t, ok)
		require.Equal(t, 1, count)
		second, ok, count := AcquireConcurrencyWithCountByKey(key, 0)
		t.Cleanup(second.Release)
		require.True(t, ok)
		require.Equal(t, 2, count)
		require.Equal(t, 2, GetConcurrencyByKey(key))
		first.Release()
		first.Release()
		require.Equal(t, 1, GetConcurrencyByKey(key))
		second.Release()
		require.Zero(t, GetConcurrencyByKey(key))
	}
}

func TestLocalConcurrencyAcquireIsAtomic(t *testing.T) {
	oldRedisEnabled, oldRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = false, nil
	defer func() { common.RedisEnabled, common.RDB = oldRedisEnabled, oldRDB }()
	resetLocalConcurrencyForTest()

	var wg sync.WaitGroup
	var acquired int
	var mu sync.Mutex
	leases := make(chan *ConcurrencyLease, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lease, ok := AcquireChannelConcurrency(8, 3)
			if ok {
				mu.Lock()
				acquired++
				mu.Unlock()
				leases <- lease
			}
		}()
	}
	wg.Wait()
	close(leases)
	require.Equal(t, 3, acquired)
	for lease := range leases {
		lease.Release()
	}
}

func TestLocalConcurrencyHeartbeatKeepsLongRequestCounted(t *testing.T) {
	oldRedisEnabled, oldRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = false, nil
	defer func() { common.RedisEnabled, common.RDB = oldRedisEnabled, oldRDB }()
	resetLocalConcurrencyForTest()

	ttl := 500 * time.Millisecond
	first, ok := acquireConcurrencySlotWithTiming("concurrency:test:local-heartbeat", 1, ttl, 50*time.Millisecond)
	require.True(t, ok)
	t.Cleanup(first.Release)

	time.Sleep(ttl + 250*time.Millisecond)
	_, ok = acquireConcurrencySlotWithTiming("concurrency:test:local-heartbeat", 1, ttl, 50*time.Millisecond)
	require.False(t, ok, "an active request must retain its slot after the original TTL")

	first.Release()
	second, ok := acquireConcurrencySlotWithTiming("concurrency:test:local-heartbeat", 1, ttl, 50*time.Millisecond)
	require.True(t, ok)
	second.Release()
}
