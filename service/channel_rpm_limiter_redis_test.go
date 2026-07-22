package service

import (
	"context"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func setupRedisChannelRPMIntegrationTest(t *testing.T) (*redis.Client, int) {
	t.Helper()
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("set TEST_REDIS_URL to run real Redis channel RPM integration tests")
	}
	options, err := redis.ParseURL(redisURL)
	require.NoError(t, err)
	client := redis.NewClient(options)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	require.NoError(t, client.Ping(ctx).Err())
	cancel()

	channelId := int(time.Now().UnixNano() & 0x3fffffff)
	oldRedisEnabled, oldRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = true, client
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = client.Del(ctx, channelRPMKey(channelId), "concurrency:channel:"+strconv.Itoa(channelId)).Err()
		cancel()
		common.RedisEnabled, common.RDB = oldRedisEnabled, oldRDB
		_ = client.Close()
	})
	return client, channelId
}

func TestRedisUnlimitedChannelCapacityIsStillObservable(t *testing.T) {
	_, channelId := setupRedisChannelRPMIntegrationTest(t)
	first, acquired, reason := AcquireChannelCapacity(channelId, 0, 0)
	require.True(t, acquired)
	require.Equal(t, ChannelCapacityAvailable, reason)
	second, acquired, reason := AcquireChannelCapacity(channelId, 0, 0)
	require.True(t, acquired)
	require.Equal(t, ChannelCapacityAvailable, reason)
	t.Cleanup(first.Release)
	t.Cleanup(second.Release)

	snapshot := GetChannelCapacitySnapshots([]int{channelId})[channelId]
	require.Equal(t, ChannelCapacitySnapshot{CurrentConcurrency: 2, CurrentRPM: 2}, snapshot)
}

func TestRedisChannelRPMAcquireIsAtomic(t *testing.T) {
	client, channelId := setupRedisChannelRPMIntegrationTest(t)
	const limit = 9
	var wg sync.WaitGroup
	results := make(chan bool, 64)
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, _ := AcquireChannelRPM(channelId, limit)
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
	require.EqualValues(t, limit, client.ZCard(context.Background(), channelRPMKey(channelId)).Val())
}
