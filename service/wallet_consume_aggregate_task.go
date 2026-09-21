/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
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
	walletConsumeAggregateInterval       = 5 * time.Minute
	walletConsumeAggregateBatchDays      = 1
	walletConsumeAggregateLookback       = 31 * 24 * time.Hour
	walletConsumeAggregateTimeout        = 8 * time.Second
	walletConsumeAggregateReconcileDays  = 2
	walletConsumeAggregateReconcileEvery = time.Hour
)

var (
	walletConsumeAggregateOnce    sync.Once
	walletConsumeAggregateRunning atomic.Bool
)

// StartWalletConsumeAggregateTask starts the optional wallet financial-flow
// projection worker.  It is intentionally feature-off unless
// WALLET_CONSUME_AGGREGATE_WORKER_ENABLE=true; creating the schema or shipping
// this code alone never adds background scans to an existing installation.
func StartWalletConsumeAggregateTask() {
	walletConsumeAggregateOnce.Do(func() {
		if !common.IsMasterNode || !model.WalletConsumeAggregateWorkerEnabled() {
			return
		}
		common.BackgroundCtxGo("maintenance", context.Background(), func() {
			logger.LogInfo(context.Background(), fmt.Sprintf(
				"wallet consume aggregate worker started: interval=%s lookback=%s",
				walletConsumeAggregateInterval, walletConsumeAggregateLookback,
			))
			runWalletConsumeAggregateOnce()
			ticker := time.NewTicker(walletConsumeAggregateInterval)
			defer ticker.Stop()
			for range ticker.C {
				runWalletConsumeAggregateOnce()
			}
		})
	})
}

// RunWalletConsumeAggregateOnce is an explicit bounded maintenance hook for
// local validation or an operator-controlled job.  It never runs unless the
// worker feature flag is enabled and the current process is the master node.
func RunWalletConsumeAggregateOnce(ctx context.Context) error {
	if !model.WalletConsumeAggregateWorkerEnabled() {
		return fmt.Errorf("wallet consume aggregate worker is disabled")
	}
	if !common.IsMasterNode {
		return fmt.Errorf("wallet consume aggregate worker requires the master node")
	}
	if model.LOG_DB == nil {
		return fmt.Errorf("log database is unavailable")
	}
	if !walletConsumeAggregateRunning.CompareAndSwap(false, true) {
		return fmt.Errorf("wallet consume aggregate worker is already running")
	}
	defer walletConsumeAggregateRunning.Store(false)
	if !common.BackgroundWorkAllowed() {
		return fmt.Errorf("wallet consume aggregate worker deferred while system protection is active")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > walletConsumeAggregateTimeout {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, walletConsumeAggregateTimeout)
		defer cancel()
	}
	return runWalletConsumeAggregate(ctx)
}

func runWalletConsumeAggregateOnce() {
	if !walletConsumeAggregateRunning.CompareAndSwap(false, true) {
		return
	}
	defer walletConsumeAggregateRunning.Store(false)
	if !common.BackgroundWorkAllowed() {
		logger.LogInfo(context.Background(), "wallet consume aggregate deferred while system protection is active")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), walletConsumeAggregateTimeout)
	defer cancel()
	if err := runWalletConsumeAggregate(ctx); err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("wallet consume aggregate failed: %v", err))
	}
}

func runWalletConsumeAggregate(ctx context.Context) error {
	now := time.Now().Unix()
	currentDay := walletConsumeDayStartForTask(now)
	lookbackStart := walletConsumeDayStartForTask(now - int64(walletConsumeAggregateLookback/time.Second))
	if currentDay <= lookbackStart {
		return nil
	}
	checkpoint, err := model.LoadWalletConsumeAggregateCheckpoint(ctx, model.LOG_DB)
	if err != nil {
		return fmt.Errorf("load wallet consume aggregate checkpoint: %w", err)
	}
	if checkpoint.Name == "" {
		checkpoint.Name = "wallet-consume-daily-v2"
	}
	// A checkpoint exactly at currentDay means the backlog is caught up.  Do
	// not reset it to lookbackStart on every tick, otherwise the worker would
	// replay the oldest day forever and never reach reconciliation.  A future
	// checkpoint can only come from a clock change or a malformed import and is
	// safely restarted from the bounded lookback.
	if checkpoint.NextDayStart < lookbackStart || checkpoint.NextDayStart > currentDay {
		checkpoint.NextDayStart = lookbackStart
	}

	processed := 0
	for processed < walletConsumeAggregateBatchDays && checkpoint.NextDayStart < currentDay {
		if err := ctx.Err(); err != nil {
			return err
		}
		day := checkpoint.NextDayStart
		if _, err := model.RebuildWalletConsumeDailyAggregates(ctx, day, modelWalletDayEnd(day)); err != nil {
			return fmt.Errorf("rebuild wallet consume day %d: %w", day, err)
		}
		checkpoint.NextDayStart = modelWalletDayEnd(day)
		// Persist the day cursor after every successful snapshot. If a later
		// reconciliation fails, the completed backlog day must not be replayed
		// on the next tick (the rebuild is idempotent, but the extra full-day
		// scan is avoidable load).
		checkpoint.UpdatedAt = time.Now().Unix()
		if err := model.SaveWalletConsumeAggregateCheckpoint(ctx, model.LOG_DB, checkpoint); err != nil {
			return fmt.Errorf("save wallet consume aggregate checkpoint: %w", err)
		}
		processed++
	}

	// Once the lookback backlog is caught up, periodically re-read the two most
	// recent sealed days. The interval keeps a busy log database from receiving
	// two full-day scans on every five-minute worker tick while still repairing
	// ordinary late-arriving logs without touching the hot current day.
	if checkpoint.NextDayStart >= currentDay &&
		(checkpoint.LastReconciledAt <= 0 || now-checkpoint.LastReconciledAt >= int64(walletConsumeAggregateReconcileEvery/time.Second)) {
		day := previousWalletDay(currentDay)
		reconcileComplete := true
		for i := 0; i < walletConsumeAggregateReconcileDays; i++ {
			if day < lookbackStart {
				break
			}
			if err := ctx.Err(); err != nil {
				reconcileComplete = false
				break
			}
			if _, err := model.RebuildWalletConsumeDailyAggregates(ctx, day, modelWalletDayEnd(day)); err != nil {
				return fmt.Errorf("reconcile wallet consume day %d: %w", day, err)
			}
			day = previousWalletDay(day)
		}
		// Do not advance the reconciliation watermark when cancellation stopped
		// the loop early; otherwise an unvisited day would be deferred for a full
		// hour and the checkpoint would falsely claim a successful pass.
		if reconcileComplete {
			checkpoint.LastReconciledAt = now
		}
	}
	checkpoint.UpdatedAt = now
	if err := model.SaveWalletConsumeAggregateCheckpoint(ctx, model.LOG_DB, checkpoint); err != nil {
		return fmt.Errorf("save wallet consume aggregate checkpoint: %w", err)
	}
	return nil
}

func walletConsumeDayStartForTask(timestamp int64) int64 {
	value := time.Unix(timestamp, 0).In(time.Local)
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.Local).Unix()
}

func modelWalletDayEnd(dayStart int64) int64 {
	value := time.Unix(dayStart, 0).In(time.Local)
	return time.Date(value.Year(), value.Month(), value.Day()+1, 0, 0, 0, 0, time.Local).Unix()
}

func previousWalletDay(dayStart int64) int64 {
	value := time.Unix(dayStart, 0).In(time.Local)
	return time.Date(value.Year(), value.Month(), value.Day()-1, 0, 0, 0, 0, time.Local).Unix()
}
