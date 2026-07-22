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

const channelRPMWindow = time.Minute

var channelRPMSequence atomic.Uint64

var localChannelRPM = struct {
	sync.Mutex
	buckets map[int]*localRPMBucket
}{buckets: make(map[int]*localRPMBucket)}

func channelRPMKey(channelId int) string {
	return "rate_limit:channel_rpm:" + strconv.Itoa(channelId)
}

func nextChannelRPMRequestId() string {
	return fmt.Sprintf("%d-%d-%d", time.Now().UnixNano(), os.Getpid(), channelRPMSequence.Add(1))
}

// AcquireChannelRPM reserves one upstream request in a rolling one-minute
// window. A rejected acquisition does not consume a slot.
func AcquireChannelRPM(channelId, limit int) (bool, int) {
	if limit <= 0 {
		return true, 0
	}
	key := channelRPMKey(channelId)
	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		result, err := common.RDB.Eval(ctx, acquireRollingRPMScript, []string{key}, limit, channelRPMWindow.Milliseconds(), nextChannelRPMRequestId()).Int64Slice()
		cancel()
		if err == nil && len(result) == 2 {
			return result[0] == 1, int(result[1])
		}
		logger.LogWarn(nil, fmt.Sprintf("Redis channel RPM limiter unavailable for %s, using local fallback: %v", key, err))
	}
	return acquireLocalChannelRPM(channelId, limit, time.Now())
}

func acquireLocalChannelRPM(channelId, limit int, now time.Time) (bool, int) {
	localChannelRPM.Lock()
	defer localChannelRPM.Unlock()
	bucket := localChannelRPM.buckets[channelId]
	if bucket == nil {
		bucket = &localRPMBucket{}
		localChannelRPM.buckets[channelId] = bucket
	}
	cutoff := now.Add(-channelRPMWindow)
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

func GetChannelRPM(channelId int) int {
	key := channelRPMKey(channelId)
	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		now := time.Now().UnixMilli()
		pipe := common.RDB.TxPipeline()
		pipe.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(now-channelRPMWindow.Milliseconds(), 10))
		countCmd := pipe.ZCard(ctx, key)
		_, err := pipe.Exec(ctx)
		cancel()
		if err == nil {
			return int(countCmd.Val())
		}
	}
	localChannelRPM.Lock()
	defer localChannelRPM.Unlock()
	bucket := localChannelRPM.buckets[channelId]
	if bucket == nil {
		return 0
	}
	cutoff := time.Now().Add(-channelRPMWindow)
	kept := bucket.requests[:0]
	for _, requestedAt := range bucket.requests {
		if requestedAt.After(cutoff) {
			kept = append(kept, requestedAt)
		}
	}
	bucket.requests = kept
	return len(bucket.requests)
}

func resetLocalChannelRPMForTest() {
	localChannelRPM.Lock()
	localChannelRPM.buckets = make(map[int]*localRPMBucket)
	localChannelRPM.Unlock()
}
