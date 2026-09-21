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
)

const (
	detailedUsageLogRetentionInterval     = 1 * time.Hour
	detailedUsageLogRetentionBatch        = 1000
	detailedUsageLogRetentionMaxBatch     = 20
	detailedUsageLogRetentionBatchTimeout = 3 * time.Second
	detailedUsageLogRetentionBudget       = 10 * time.Second
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
		common.BackgroundCtxGo("maintenance", context.Background(), func() {
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
	if !common.BackgroundWorkAllowed() {
		logger.LogInfo(context.Background(), "detailed usage log retention paused while system protection is active")
		return
	}
	startedAt := time.Now()
	cutoff := common.GetTimestamp() - int64(model.DetailedUsageLogRetentionDays*24*60*60)
	var total int64
	for batch := 0; batch < detailedUsageLogRetentionMaxBatch; batch++ {
		if batch > 0 && time.Since(startedAt) >= detailedUsageLogRetentionBudget {
			logger.LogInfo(context.Background(), fmt.Sprintf(
				"detailed usage log retention paused at budget: archived=%d elapsed=%s",
				total, time.Since(startedAt).Round(time.Millisecond),
			))
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), detailedUsageLogRetentionBatchTimeout)
		archived, _, err := model.ArchiveDetailedUsageLogs(ctx, cutoff, detailedUsageLogRetentionBatch)
		cancel()
		if err != nil {
			logger.LogWarn(context.Background(), fmt.Sprintf("detailed usage log retention failed: %v", err))
			return
		}
		total += archived
		if archived < detailedUsageLogRetentionBatch {
			break
		}
		if batch == detailedUsageLogRetentionMaxBatch-1 {
			logger.LogInfo(context.Background(), fmt.Sprintf(
				"detailed usage log retention paused after %d rows; remaining backlog will continue next interval",
				total,
			))
		}
	}
	if total > 0 {
		logger.LogInfo(context.Background(), fmt.Sprintf("detailed usage log retention archived and deleted %d rows", total))
	}
}
