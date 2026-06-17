package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/bytedance/gopkg/util/gopool"
)

var (
	dailySettleOnce    sync.Once
	dailySettleRunning atomic.Bool
)

// StartDailySettleTask 启动 T+1 分润结算定时任务(每日 SettleHour 点结算昨日全天)。
// 仅主节点运行; sync.Once 防重复启动; atomic.Bool 防单次执行重叠; recover 防异常拖死 ticker。
// 幂等可重跑(基于 log.settled + AffiliateSettle.Status)。
func StartDailySettleTask() {
	dailySettleOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			hour := operation_setting.GetAffiliateSettleHour()
			logger.LogInfo(context.Background(), fmt.Sprintf("daily settle task started: scheduled at %02d:00 daily", hour))
			// 先等到下一个结算时刻, 再 ticker 24h
			time.Sleep(untilNextSettleHour(hour))
			runDailySettleSafe()
			ticker := time.NewTicker(24 * time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				runDailySettleSafe()
			}
		})
	})
}

// runDailySettleSafe 单次结算执行(防重入 + recover + 开关检查)。
func runDailySettleSafe() {
	if !dailySettleRunning.CompareAndSwap(false, true) {
		return
	}
	defer dailySettleRunning.Store(false)
	defer func() {
		if r := recover(); r != nil {
			logger.LogError(context.Background(), fmt.Sprintf("daily settle panic recovered: %v", r))
		}
	}()
	if !operation_setting.IsAffiliateSettleEnabled() {
		return
	}
	// 结算昨日全天(本地时区)
	now := time.Now()
	yesterday := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, now.Location())
	dayStart := yesterday.Unix()
	dayEnd := yesterday.Add(24 * time.Hour).Unix()
	batchId := yesterday.Format("2006-01-02")
	if err := RunDailySettle(batchId, dayStart, dayEnd); err != nil {
		logger.LogError(context.Background(), fmt.Sprintf("daily settle %s failed: %v", batchId, err))
	}
}

// untilNextSettleHour 计算到下一个结算时刻(本地时区 hour:00)的等待时长。
func untilNextSettleHour(hour int) time.Duration {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return time.Until(next)
}
