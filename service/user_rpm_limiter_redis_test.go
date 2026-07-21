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

func setupRedisUserRPMIntegrationTest(t *testing.T) (*redis.Client, int) {
	t.Helper()
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("set TEST_REDIS_URL to run real Redis user RPM integration tests")
	}
	options, err := redis.ParseURL(redisURL)
	require.NoError(t, err)
	client := redis.NewClient(options)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	require.NoError(t, client.Ping(ctx).Err())
	cancel()

	oldRedisEnabled, oldRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = true, client
	userId := int(time.Now().UnixNano() & 0x3fffffff)
	key := userRPMKey(userId)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = client.Del(ctx, key).Err()
		cancel()
		common.RedisEnabled, common.RDB = oldRedisEnabled, oldRDB
		_ = client.Close()
	})
	return client, userId
}

func TestRedisUserRPMAcquireIsAtomic(t *testing.T) {
	client, userId := setupRedisUserRPMIntegrationTest(t)
	const limit = 12
	var wg sync.WaitGroup
	results := make(chan bool, 64)
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, _ := AcquireUserRPM(userId, limit)
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
	require.EqualValues(t, limit, client.ZCard(context.Background(), userRPMKey(userId)).Val(), fmt.Sprintf("Redis should retain exactly %d rolling-window requests", limit))
}
