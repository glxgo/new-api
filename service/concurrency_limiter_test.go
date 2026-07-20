package service

import (
	"sync"
	"testing"

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
