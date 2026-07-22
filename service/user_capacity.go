package service

import (
	"context"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
)

type UserCapacitySnapshot struct {
	CurrentConcurrency int `json:"current_concurrency"`
	CurrentRPM         int `json:"current_rpm"`
}

// GetUserCapacitySnapshots reads one user-list page with a single Redis
// roundtrip. Expired members are cleaned in the same pipeline before counts
// are returned. Redis failure falls back locally without N serial timeouts.
func GetUserCapacitySnapshots(userIds []int) map[int]UserCapacitySnapshot {
	snapshots := make(map[int]UserCapacitySnapshot, len(userIds))
	uniqueIds := make([]int, 0, len(userIds))
	seen := make(map[int]struct{}, len(userIds))
	for _, userId := range userIds {
		if userId <= 0 {
			continue
		}
		if _, ok := seen[userId]; ok {
			continue
		}
		seen[userId] = struct{}{}
		uniqueIds = append(uniqueIds, userId)
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
		for _, userId := range uniqueIds {
			concurrencyKey := "concurrency:user:" + strconv.Itoa(userId)
			rpmKey := userRPMKey(userId)
			pipe.ZRemRangeByScore(ctx, concurrencyKey, "-inf", strconv.FormatInt(now.Unix()-int64(concurrencySlotTTL.Seconds()), 10))
			concurrencyCount := pipe.ZCard(ctx, concurrencyKey)
			pipe.ZRemRangeByScore(ctx, rpmKey, "-inf", strconv.FormatInt(now.UnixMilli()-userRPMWindow.Milliseconds(), 10))
			rpmCount := pipe.ZCard(ctx, rpmKey)
			commands[userId] = countCommands{concurrency: concurrencyCount, rpm: rpmCount}
		}
		_, err := pipe.Exec(ctx)
		cancel()
		if err == nil {
			for userId, command := range commands {
				snapshots[userId] = UserCapacitySnapshot{
					CurrentConcurrency: int(command.concurrency.Val()),
					CurrentRPM:         int(command.rpm.Val()),
				}
			}
			return snapshots
		}
	}

	now := time.Now()
	for _, userId := range uniqueIds {
		snapshots[userId] = UserCapacitySnapshot{
			CurrentConcurrency: getLocalConcurrencyCount("concurrency:user:"+strconv.Itoa(userId), now),
			CurrentRPM:         getLocalUserRPMCount(userId, now),
		}
	}
	return snapshots
}
