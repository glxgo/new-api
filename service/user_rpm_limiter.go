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

const userRPMWindow = time.Minute

var userRPMSequence atomic.Uint64

var acquireRollingRPMScript = `
local key = KEYS[1]
local max_requests = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local request_id = ARGV[3]
local redis_time = redis.call('TIME')
local now_ms = tonumber(redis_time[1]) * 1000 + math.floor(tonumber(redis_time[2]) / 1000)
redis.call('ZREMRANGEBYSCORE', key, '-inf', now_ms - window_ms)
local current = redis.call('ZCARD', key)
if current >= max_requests then
  redis.call('PEXPIRE', key, window_ms)
  return {0, current}
end
redis.call('ZADD', key, now_ms, request_id)
redis.call('PEXPIRE', key, window_ms)
return {1, current + 1}
`

type localRPMBucket struct {
	requests []time.Time
}

var localUserRPM = struct {
	sync.Mutex
	buckets map[string]*localRPMBucket
}{buckets: make(map[string]*localRPMBucket)}

func userRPMKey(userId int) string {
	return "rate_limit:user_rpm:" + strconv.Itoa(userId)
}

// UserRPMKey is the stable Redis/local key for a user's normal RPM pool.
func UserRPMKey(userId int) string {
	return userRPMKey(userId)
}

// VirtualMembershipRPMKey is intentionally separate from the normal user
// pool so the two limits never consume each other's requests.
func VirtualMembershipRPMKey(userId, membershipId int) string {
	return userRPMKey(userId) + ":virtual:" + strconv.Itoa(membershipId)
}

func nextUserRPMRequestId() string {
	return fmt.Sprintf("%d-%d-%d", time.Now().UnixNano(), os.Getpid(), userRPMSequence.Add(1))
}

// AcquireUserRPM counts one authenticated external API request in a rolling
// one-minute window. Channel retries happen after this middleware and therefore
// do not consume additional RPM slots.
func AcquireUserRPM(userId, limit int) (bool, int) {
	return AcquireUserRPMByKey(userRPMKey(userId), limit)
}

// AcquireUserRPMByKey acquires one request from an arbitrary rolling-RPM
// pool. Keeping the pool key separate lets a membership key have its own RPM
// budget without sharing the user's normal wallet/API-key budget.
func AcquireUserRPMByKey(key string, limit int) (bool, int) {
	if limit <= 0 {
		return true, 0
	}
	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		result, err := common.RDB.Eval(ctx, acquireRollingRPMScript, []string{key}, limit, userRPMWindow.Milliseconds(), nextUserRPMRequestId()).Int64Slice()
		cancel()
		if err == nil && len(result) == 2 {
			return result[0] == 1, int(result[1])
		}
		logger.LogWarn(nil, fmt.Sprintf("Redis user RPM limiter unavailable for %s, using local fallback: %v", key, err))
	}
	return acquireLocalUserRPMByKey(key, limit, time.Now())
}

func acquireLocalUserRPM(userId, limit int, now time.Time) (bool, int) {
	return acquireLocalUserRPMByKey(userRPMKey(userId), limit, now)
}

func acquireLocalUserRPMByKey(key string, limit int, now time.Time) (bool, int) {
	localUserRPM.Lock()
	defer localUserRPM.Unlock()
	bucket := localUserRPM.buckets[key]
	if bucket == nil {
		bucket = &localRPMBucket{}
		localUserRPM.buckets[key] = bucket
	}
	cutoff := now.Add(-userRPMWindow)
	kept := bucket.requests[:0]
	for _, requestedAt := range bucket.requests {
		if requestedAt.After(cutoff) {
			kept = append(kept, requestedAt)
		}
	}
	bucket.requests = kept
	if len(bucket.requests) >= limit {
		return false, len(bucket.requests)
	}
	bucket.requests = append(bucket.requests, now)
	return true, len(bucket.requests)
}

func GetUserRPM(userId int) int {
	return GetRPMByKey(userRPMKey(userId))
}

// GetRPMByKey returns the current rolling request count for an arbitrary
// capacity pool.
func GetRPMByKey(key string) int {
	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		now := time.Now().UnixMilli()
		pipe := common.RDB.TxPipeline()
		pipe.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(now-userRPMWindow.Milliseconds(), 10))
		countCmd := pipe.ZCard(ctx, key)
		_, err := pipe.Exec(ctx)
		cancel()
		if err == nil {
			return int(countCmd.Val())
		}
	}
	return getLocalUserRPMCountByKey(key, time.Now())
}

func getLocalUserRPMCount(userId int, now time.Time) int {
	return getLocalUserRPMCountByKey(userRPMKey(userId), now)
}

func getLocalUserRPMCountByKey(key string, now time.Time) int {
	localUserRPM.Lock()
	defer localUserRPM.Unlock()
	bucket := localUserRPM.buckets[key]
	if bucket == nil {
		return 0
	}
	cutoff := now.Add(-userRPMWindow)
	kept := bucket.requests[:0]
	for _, requestedAt := range bucket.requests {
		if requestedAt.After(cutoff) {
			kept = append(kept, requestedAt)
		}
	}
	bucket.requests = kept
	return len(bucket.requests)
}

func resetLocalUserRPMForTest() {
	localUserRPM.Lock()
	localUserRPM.buckets = make(map[string]*localRPMBucket)
	localUserRPM.Unlock()
}
