package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	detailedUsageLogRetentionInterval = 1 * time.Hour
	detailedUsageLogRetentionBatch    = 1000
	detailedUsageLogRetentionMaxBatch = 20
)

var (
	detailedUsageLogRetentionOnce    sync.Once
	detailedUsageLogRetentionRunning atomic.Bool
)

func StartDetailedUsageLogRetentionTask() {
	detailedUsageLogRetentionOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf(
				"detailed usage log retention task started: detail_days=%d interval=%s",
				model.DetailedUsageLogRetentionDays, detailedUsageLogRetentionInterval,
			))
			runDetailedUsageLogRetentionOnce()
			ticker := time.NewTicker(detailedUsageLogRetentionInterval)
			defer ticker.Stop()
			for range ticker.C {
				runDetailedUsageLogRetentionOnce()
			}
		})
	})
}

func runDetailedUsageLogRetentionOnce() {
	if !detailedUsageLogRetentionRunning.CompareAndSwap(false, true) {
		return
	}
	defer detailedUsageLogRetentionRunning.Store(false)
	ctx := context.Background()
	cutoff := common.GetTimestamp() - int64(model.DetailedUsageLogRetentionDays*24*60*60)
	var total int64
	for batch := 0; batch < detailedUsageLogRetentionMaxBatch; batch++ {
		archived, _, err := model.ArchiveDetailedUsageLogs(ctx, cutoff, detailedUsageLogRetentionBatch)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("detailed usage log retention failed: %v", err))
			return
		}
		total += archived
		if archived < detailedUsageLogRetentionBatch {
			break
		}
		if batch == detailedUsageLogRetentionMaxBatch-1 {
			logger.LogInfo(ctx, fmt.Sprintf(
				"detailed usage log retention paused after %d rows; remaining backlog will continue next interval",
				total,
			))
		}
	}
	if total > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("detailed usage log retention archived and deleted %d rows", total))
	}
}
