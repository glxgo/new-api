package service

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
)

const concurrencySlotTTL = 30 * time.Minute

var concurrencySequence atomic.Uint64

var acquireConcurrencyScript = `
local key = KEYS[1]
local max_concurrency = tonumber(ARGV[1])
local ttl_seconds = tonumber(ARGV[2])
local request_id = ARGV[3]
local now = tonumber(redis.call('TIME')[1])
redis.call('ZREMRANGEBYSCORE', key, '-inf', now - ttl_seconds)
if redis.call('ZSCORE', key, request_id) then
  redis.call('ZADD', key, now, request_id)
  redis.call('EXPIRE', key, ttl_seconds)
  return 1
end
if redis.call('ZCARD', key) >= max_concurrency then
  return 0
end
redis.call('ZADD', key, now, request_id)
redis.call('EXPIRE', key, ttl_seconds)
return 1
`

type localConcurrencyBucket struct {
	members map[string]time.Time
}

var localConcurrency = struct {
	sync.Mutex
	buckets map[string]*localConcurrencyBucket
}{buckets: make(map[string]*localConcurrencyBucket)}

// ConcurrencyLease represents one active request. Release is idempotent and
// must be called after the complete response stream finishes.
type ConcurrencyLease struct {
	key       string
	requestId string
	redis     bool
	once      sync.Once
}

func (lease *ConcurrencyLease) Release() {
	if lease == nil {
		return
	}
	lease.once.Do(func() {
		if lease.redis && common.RedisEnabled && common.RDB != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			err := common.RDB.ZRem(ctx, lease.key, lease.requestId).Err()
			cancel()
			if err == nil {
				return
			}
			logger.LogWarn(nil, fmt.Sprintf("release Redis concurrency slot failed for %s: %v", lease.key, err))
		}
		localConcurrency.Lock()
		if bucket := localConcurrency.buckets[lease.key]; bucket != nil {
			delete(bucket.members, lease.requestId)
			if len(bucket.members) == 0 {
				delete(localConcurrency.buckets, lease.key)
			}
		}
		localConcurrency.Unlock()
	})
}

func nextConcurrencyRequestId() string {
	return fmt.Sprintf("%d-%d-%d", time.Now().UnixNano(), os.Getpid(), concurrencySequence.Add(1))
}

func acquireConcurrencySlot(key string, limit int) (*ConcurrencyLease, bool) {
	if limit <= 0 {
		return &ConcurrencyLease{}, true
	}
	requestId := nextConcurrencyRequestId()
	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		result, err := common.RDB.Eval(ctx, acquireConcurrencyScript, []string{key}, limit, int(concurrencySlotTTL.Seconds()), requestId).Int()
		cancel()
		if err == nil {
			if result == 1 {
				return &ConcurrencyLease{key: key, requestId: requestId, redis: true}, true
			}
			return nil, false
		}
		// Availability is preferable to rejecting every request during a Redis
		// incident. The per-process fallback still prevents local overload.
		logger.LogWarn(nil, fmt.Sprintf("Redis concurrency limiter unavailable for %s, using local fallback: %v", key, err))
	}
	return acquireLocalConcurrencySlot(key, limit, requestId)
}

func acquireLocalConcurrencySlot(key string, limit int, requestId string) (*ConcurrencyLease, bool) {
	now := time.Now()
	localConcurrency.Lock()
	defer localConcurrency.Unlock()
	bucket := localConcurrency.buckets[key]
	if bucket == nil {
		bucket = &localConcurrencyBucket{members: make(map[string]time.Time)}
		localConcurrency.buckets[key] = bucket
	}
	for member, acquiredAt := range bucket.members {
		if now.Sub(acquiredAt) >= concurrencySlotTTL {
			delete(bucket.members, member)
		}
	}
	if len(bucket.members) >= limit {
		return nil, false
	}
	bucket.members[requestId] = now
	return &ConcurrencyLease{key: key, requestId: requestId}, true
}

func AcquireUserConcurrency(userId, limit int) (*ConcurrencyLease, bool) {
	return acquireConcurrencySlot("concurrency:user:"+strconv.Itoa(userId), limit)
}

func AcquireChannelConcurrency(channelId, limit int) (*ConcurrencyLease, bool) {
	return acquireConcurrencySlot("concurrency:channel:"+strconv.Itoa(channelId), limit)
}

func getConcurrencyCount(key string) int {
	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		now := time.Now().Unix()
		pipe := common.RDB.TxPipeline()
		pipe.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(now-int64(concurrencySlotTTL.Seconds()), 10))
		countCmd := pipe.ZCard(ctx, key)
		_, err := pipe.Exec(ctx)
		cancel()
		if err == nil {
			return int(countCmd.Val())
		}
	}
	localConcurrency.Lock()
	defer localConcurrency.Unlock()
	bucket := localConcurrency.buckets[key]
	if bucket == nil {
		return 0
	}
	now := time.Now()
	for member, acquiredAt := range bucket.members {
		if now.Sub(acquiredAt) >= concurrencySlotTTL {
			delete(bucket.members, member)
		}
	}
	return len(bucket.members)
}

func GetUserConcurrency(userId int) int {
	return getConcurrencyCount("concurrency:user:" + strconv.Itoa(userId))
}

func GetChannelConcurrency(channelId int) int {
	return getConcurrencyCount("concurrency:channel:" + strconv.Itoa(channelId))
}

// resetLocalConcurrencyForTest isolates unit tests in this package.
func resetLocalConcurrencyForTest() {
	localConcurrency.Lock()
	localConcurrency.buckets = make(map[string]*localConcurrencyBucket)
	localConcurrency.Unlock()
}
