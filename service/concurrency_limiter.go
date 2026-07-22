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

const (
	concurrencySlotTTL               = 30 * time.Minute
	concurrencySlotHeartbeatInterval = 5 * time.Minute
)

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
  return {1, redis.call('ZCARD', key)}
end
local current = redis.call('ZCARD', key)
if current >= max_concurrency then
  return {0, current}
end
redis.call('ZADD', key, now, request_id)
redis.call('EXPIRE', key, ttl_seconds)
return {1, current + 1}
`

var renewConcurrencyScript = `
local key = KEYS[1]
local ttl_seconds = tonumber(ARGV[1])
local request_id = ARGV[2]
if not redis.call('ZSCORE', key, request_id) then
  return 0
end
local now = tonumber(redis.call('TIME')[1])
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
	key               string
	requestId         string
	redis             bool
	ttl               time.Duration
	heartbeatInterval time.Duration
	stopHeartbeat     chan struct{}
	once              sync.Once
}

func concurrencyTTLSeconds(ttl time.Duration) int {
	if ttl <= 0 {
		return 1
	}
	return int((ttl + time.Second - 1) / time.Second)
}

func newConcurrencyLease(key, requestId string, redis bool, ttl, heartbeatInterval time.Duration) *ConcurrencyLease {
	lease := &ConcurrencyLease{
		key:               key,
		requestId:         requestId,
		redis:             redis,
		ttl:               ttl,
		heartbeatInterval: heartbeatInterval,
	}
	if requestId != "" && heartbeatInterval > 0 {
		lease.stopHeartbeat = make(chan struct{})
		go lease.runHeartbeat()
	}
	return lease
}

func (lease *ConcurrencyLease) runHeartbeat() {
	ticker := time.NewTicker(lease.heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			renewed, err := lease.renew()
			if err != nil {
				logger.LogWarn(nil, fmt.Sprintf("renew concurrency slot failed for %s: %v", lease.key, err))
				continue
			}
			if !renewed {
				return
			}
		case <-lease.stopHeartbeat:
			return
		}
	}
}

func (lease *ConcurrencyLease) renew() (bool, error) {
	if lease == nil || lease.requestId == "" {
		return false, nil
	}
	if lease.redis {
		if !common.RedisEnabled || common.RDB == nil {
			return false, fmt.Errorf("Redis is unavailable")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		result, err := common.RDB.Eval(ctx, renewConcurrencyScript, []string{lease.key},
			concurrencyTTLSeconds(lease.ttl), lease.requestId).Int()
		cancel()
		if err != nil {
			return false, err
		}
		return result == 1, nil
	}

	localConcurrency.Lock()
	defer localConcurrency.Unlock()
	bucket := localConcurrency.buckets[lease.key]
	if bucket == nil {
		return false, nil
	}
	if _, ok := bucket.members[lease.requestId]; !ok {
		return false, nil
	}
	bucket.members[lease.requestId] = time.Now()
	return true, nil
}

func (lease *ConcurrencyLease) Release() {
	if lease == nil {
		return
	}
	lease.once.Do(func() {
		if lease.stopHeartbeat != nil {
			close(lease.stopHeartbeat)
		}
		if lease.requestId == "" {
			return
		}
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
	lease, acquired, _ := acquireConcurrencySlotWithTimingAndCount(key, limit, concurrencySlotTTL, concurrencySlotHeartbeatInterval)
	return lease, acquired
}

func acquireConcurrencySlotWithTiming(key string, limit int, ttl, heartbeatInterval time.Duration) (*ConcurrencyLease, bool) {
	lease, acquired, _ := acquireConcurrencySlotWithTimingAndCount(key, limit, ttl, heartbeatInterval)
	return lease, acquired
}

func acquireConcurrencySlotWithTimingAndCount(key string, limit int, ttl, heartbeatInterval time.Duration) (*ConcurrencyLease, bool, int) {
	if limit <= 0 {
		return &ConcurrencyLease{}, true, 0
	}
	if ttl <= 0 {
		ttl = concurrencySlotTTL
	}
	if heartbeatInterval <= 0 || heartbeatInterval >= ttl {
		heartbeatInterval = ttl / 2
		if heartbeatInterval <= 0 {
			heartbeatInterval = time.Nanosecond
		}
	}
	requestId := nextConcurrencyRequestId()
	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		result, err := common.RDB.Eval(ctx, acquireConcurrencyScript, []string{key}, limit, concurrencyTTLSeconds(ttl), requestId).Int64Slice()
		cancel()
		if err == nil && len(result) == 2 {
			if result[0] == 1 {
				return newConcurrencyLease(key, requestId, true, ttl, heartbeatInterval), true, int(result[1])
			}
			return nil, false, int(result[1])
		}
		// Availability is preferable to rejecting every request during a Redis
		// incident. The per-process fallback still prevents local overload.
		logger.LogWarn(nil, fmt.Sprintf("Redis concurrency limiter unavailable for %s, using local fallback: %v", key, err))
	}
	return acquireLocalConcurrencySlot(key, limit, requestId, ttl, heartbeatInterval)
}

func acquireLocalConcurrencySlot(key string, limit int, requestId string, ttl, heartbeatInterval time.Duration) (*ConcurrencyLease, bool, int) {
	now := time.Now()
	localConcurrency.Lock()
	defer localConcurrency.Unlock()
	bucket := localConcurrency.buckets[key]
	if bucket == nil {
		bucket = &localConcurrencyBucket{members: make(map[string]time.Time)}
		localConcurrency.buckets[key] = bucket
	}
	for member, acquiredAt := range bucket.members {
		if now.Sub(acquiredAt) >= ttl {
			delete(bucket.members, member)
		}
	}
	if len(bucket.members) >= limit {
		return nil, false, len(bucket.members)
	}
	bucket.members[requestId] = now
	return newConcurrencyLease(key, requestId, false, ttl, heartbeatInterval), true, len(bucket.members)
}

func AcquireUserConcurrency(userId, limit int) (*ConcurrencyLease, bool) {
	return acquireConcurrencySlot("concurrency:user:"+strconv.Itoa(userId), limit)
}

func AcquireUserConcurrencyWithCount(userId, limit int) (*ConcurrencyLease, bool, int) {
	return acquireConcurrencySlotWithTimingAndCount("concurrency:user:"+strconv.Itoa(userId), limit, concurrencySlotTTL, concurrencySlotHeartbeatInterval)
}

func AcquireChannelConcurrency(channelId, limit int) (*ConcurrencyLease, bool) {
	if limit <= 0 {
		limit = unlimitedChannelTrackingLimit
	}
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
	return getLocalConcurrencyCount(key, time.Now())
}

func getLocalConcurrencyCount(key string, now time.Time) int {
	localConcurrency.Lock()
	defer localConcurrency.Unlock()
	bucket := localConcurrency.buckets[key]
	if bucket == nil {
		return 0
	}
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
