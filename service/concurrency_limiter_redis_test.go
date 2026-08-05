package service

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func setupRedisConcurrencyIntegrationTest(t *testing.T) (*redis.Client, string) {
	t.Helper()
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("set TEST_REDIS_URL to run real Redis concurrency integration tests")
	}
	options, err := redis.ParseURL(redisURL)
	require.NoError(t, err)
	client := redis.NewClient(options)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	require.NoError(t, client.Ping(ctx).Err())
	cancel()

	oldRedisEnabled, oldRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = true, client
	key := fmt.Sprintf("concurrency:test:%d", time.Now().UnixNano())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = client.Del(ctx, key).Err()
		cancel()
		common.RedisEnabled, common.RDB = oldRedisEnabled, oldRDB
		_ = client.Close()
	})
	return client, key
}

func TestRedisConcurrencyAcquireIsAtomic(t *testing.T) {
	client, key := setupRedisConcurrencyIntegrationTest(t)

	const limit = 8
	var wg sync.WaitGroup
	leases := make(chan *ConcurrencyLease, 64)
	counts := make(chan int, 64)
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lease, ok, current := acquireConcurrencySlotWithTimingAndCount(key, limit, 10*time.Second, 2*time.Second)
			if ok {
				leases <- lease
				counts <- current
			}
		}()
	}
	wg.Wait()
	close(leases)
	close(counts)

	acquired := make([]*ConcurrencyLease, 0, limit)
	for lease := range leases {
		acquired = append(acquired, lease)
	}
	t.Cleanup(func() {
		for _, lease := range acquired {
			lease.Release()
		}
	})
	require.Len(t, acquired, limit)
	seenCounts := make(map[int]bool, limit)
	for current := range counts {
		seenCounts[current] = true
	}
	for current := 1; current <= limit; current++ {
		require.True(t, seenCounts[current], "atomic admission count %d should be returned once", current)
	}
	require.EqualValues(t, limit, client.ZCard(context.Background(), key).Val())

	for _, lease := range acquired {
		lease.Release()
	}
	require.Eventually(t, func() bool {
		return client.ZCard(context.Background(), key).Val() == 0
	}, 2*time.Second, 20*time.Millisecond)
}

func TestRedisConcurrencyHeartbeatKeepsLeaseAliveAndDoesNotResurrectRelease(t *testing.T) {
	client, key := setupRedisConcurrencyIntegrationTest(t)

	ttl := 2 * time.Second
	heartbeat := 200 * time.Millisecond
	lease, ok := acquireConcurrencySlotWithTiming(key, 1, ttl, heartbeat)
	require.True(t, ok)
	t.Cleanup(lease.Release)

	time.Sleep(ttl + time.Second)
	require.EqualValues(t, 1, client.ZCard(context.Background(), key).Val(),
		"the heartbeat must keep an active lease alive beyond its original TTL")
	require.Greater(t, client.TTL(context.Background(), key).Val(), time.Duration(0))

	lease.Release()
	require.Eventually(t, func() bool {
		return client.ZCard(context.Background(), key).Val() == 0
	}, 2*time.Second, 20*time.Millisecond)
	renewed, err := lease.renew()
	require.NoError(t, err)
	require.False(t, renewed, "renewal must be ownership-safe and never recreate a missing member")
	time.Sleep(2 * heartbeat)
	require.EqualValues(t, 0, client.ZCard(context.Background(), key).Val(),
		"a stopped heartbeat must not recreate a released lease")
}

func TestRedisConcurrencyCountCleansMembersFromDeadOwners(t *testing.T) {
	client, key := setupRedisConcurrencyIntegrationTest(t)

	deadOwner := fmt.Sprintf("dead-owner-%d", time.Now().UnixNano())
	deadMember := deadOwner + "|request-1"
	ctx := context.Background()
	require.NoError(t, client.ZAdd(ctx, key, &redis.Z{Score: float64(time.Now().Unix()), Member: deadMember}).Err())
	require.Equal(t, 0, getConcurrencyCount(key), "a lease from a process without a heartbeat must not count")

	liveOwner := fmt.Sprintf("live-owner-%d", time.Now().UnixNano())
	liveMember := liveOwner + "|request-1"
	liveOwnerKey := concurrencyOwnerKeyPrefix + liveOwner
	require.NoError(t, client.Set(ctx, liveOwnerKey, "1", concurrencyOwnerTTL).Err())
	t.Cleanup(func() { _ = client.Del(ctx, liveOwnerKey).Err() })
	require.NoError(t, client.ZAdd(ctx, key, &redis.Z{Score: float64(time.Now().Unix()), Member: liveMember}).Err())
	require.Equal(t, 1, getConcurrencyCount(key), "a lease from a live process must remain counted")

	require.NoError(t, client.Del(ctx, liveOwnerKey).Err())
	require.Equal(t, 0, getConcurrencyCount(key), "the lease must disappear after its owner heartbeat is gone")
}
