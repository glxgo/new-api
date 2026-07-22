package service

import (
	"context"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
)

const unlimitedChannelTrackingLimit = 1 << 30

type ChannelCapacityReason string

const (
	ChannelCapacityAvailable       ChannelCapacityReason = ""
	ChannelCapacityConcurrencyFull ChannelCapacityReason = "concurrency"
	ChannelCapacityRPMFull         ChannelCapacityReason = "rpm"
)

type ChannelCapacitySnapshot struct {
	CurrentConcurrency int `json:"current_concurrency"`
	CurrentRPM         int `json:"current_rpm"`
}

// GetChannelCapacitySnapshots reads all channels visible on one admin page
// with one Redis pipeline. Unlimited channels are also tracked, so zero means
// genuinely idle rather than "counter disabled".
func GetChannelCapacitySnapshots(channelIds []int) map[int]ChannelCapacitySnapshot {
	snapshots := make(map[int]ChannelCapacitySnapshot, len(channelIds))
	uniqueIds := make([]int, 0, len(channelIds))
	seen := make(map[int]struct{}, len(channelIds))
	for _, channelId := range channelIds {
		if channelId <= 0 {
			continue
		}
		if _, ok := seen[channelId]; ok {
			continue
		}
		seen[channelId] = struct{}{}
		uniqueIds = append(uniqueIds, channelId)
	}
	if len(uniqueIds) == 0 {
		return snapshots
	}

	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		pipe := common.RDB.TxPipeline()
		now := time.Now()
		type countCommands struct {
			concurrency *redis.IntCmd
			rpm         *redis.IntCmd
		}
		commands := make(map[int]countCommands, len(uniqueIds))
		for _, channelId := range uniqueIds {
			concurrencyKey := "concurrency:channel:" + strconv.Itoa(channelId)
			rpmKey := channelRPMKey(channelId)
			pipe.ZRemRangeByScore(ctx, concurrencyKey, "-inf", strconv.FormatInt(now.Unix()-int64(concurrencySlotTTL.Seconds()), 10))
			concurrencyCount := pipe.ZCard(ctx, concurrencyKey)
			pipe.ZRemRangeByScore(ctx, rpmKey, "-inf", strconv.FormatInt(now.UnixMilli()-channelRPMWindow.Milliseconds(), 10))
			rpmCount := pipe.ZCard(ctx, rpmKey)
			commands[channelId] = countCommands{concurrency: concurrencyCount, rpm: rpmCount}
		}
		_, err := pipe.Exec(ctx)
		cancel()
		if err == nil {
			for channelId, command := range commands {
				snapshots[channelId] = ChannelCapacitySnapshot{
					CurrentConcurrency: int(command.concurrency.Val()),
					CurrentRPM:         int(command.rpm.Val()),
				}
			}
			return snapshots
		}
	}

	now := time.Now()
	for _, channelId := range uniqueIds {
		snapshots[channelId] = ChannelCapacitySnapshot{
			CurrentConcurrency: getLocalConcurrencyCount("concurrency:channel:"+strconv.Itoa(channelId), now),
			CurrentRPM:         getLocalChannelRPMCount(channelId, now),
		}
	}
	return snapshots
}

// AcquireChannelCapacity reserves both capacity dimensions. Concurrency is
// acquired first; if RPM is already full, its lease is immediately released
// so a skipped channel never leaks an active slot.
func AcquireChannelCapacity(channelId, concurrencyLimit, rpmLimit int) (*ConcurrencyLease, bool, ChannelCapacityReason) {
	lease, acquired := AcquireChannelConcurrency(channelId, concurrencyLimit)
	if !acquired {
		return nil, false, ChannelCapacityConcurrencyFull
	}
	rpmAcquired, _ := AcquireChannelRPM(channelId, rpmLimit)
	if !rpmAcquired {
		lease.Release()
		return nil, false, ChannelCapacityRPMFull
	}
	return lease, true, ChannelCapacityAvailable
}
