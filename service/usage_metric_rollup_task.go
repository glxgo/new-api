package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

const (
	usageMetricWorkerTick             = 1 * time.Minute
	usageMetricWorkerBatchSize        = 500
	usageMetricWorkerTimeout          = 15 * time.Second
	usageMetricWorkerReconcileEvery   = 1 * time.Hour
	usageMetricWorkerReconcileHours   = 2
	usageMetricWorkerReconcileMaxEach = 96
	usageMetricWorkerEmptyHoursMax    = 8
	usageMetricWorkerSourceVersion    = model.UsageMetricSourceVersionV1
	usageMetricWorkerCheckpointName   = "usage-metric-worker-v1"
)

var (
	usageMetricWorkerOnce    sync.Once
	usageMetricWorkerRunning atomic.Bool

	// The projection is an optional accelerator.  Keep source reads bounded so
	// a high-volume day cannot make the background worker compete with request
	// traffic; oversized windows are marked incomplete and read-side callers
	// fall back to the exact raw/archive path.
	usageMetricWorkerMaxRowsPerBucket  int64 = 50_000
	usageMetricWorkerMaxRowsPerRebuild int64 = 100_000
	usageMetricWorkerCountTimeout            = 750 * time.Millisecond
	usageMetricWorkerRebuildBudget           = 4 * time.Second
)

// Minute buckets are not consumed by any current read path.  Avoid creating
// hundreds of tiny snapshots for every source batch until a minute-level
// reader and an incremental writer are available.
var usageMetricWorkerProjectionGranularities = []model.UsageMetricGranularity{
	model.UsageMetricGranularityHour,
	model.UsageMetricGranularityDay,
}

// StartUsageMetricRollupTask starts the optional projection worker. It is
// intentionally feature-off unless USAGE_METRIC_WORKER_ENABLE=true; enabling
// the schema alone never causes background scans.
func StartUsageMetricRollupTask() {
	usageMetricWorkerOnce.Do(func() {
		if !common.IsMasterNode || !model.UsageMetricWorkerEnabled() {
			return
		}
		common.BackgroundCtxGo("maintenance", context.Background(), func() {
			logger.LogInfo(context.Background(), fmt.Sprintf(
				"usage metric rollup worker started: tick=%s batch=%d",
				usageMetricWorkerTick, usageMetricWorkerBatchSize,
			))
			runUsageMetricRollupOnce()
			ticker := time.NewTicker(usageMetricWorkerTick)
			defer ticker.Stop()
			for range ticker.C {
				runUsageMetricRollupOnce()
			}
		})
	})
}

// RunUsageMetricRollupOnce is an explicit maintenance hook for local tests or
// an operator-controlled job. The normal scheduler uses the unexported wrapper
// below and remains disabled by default.
func RunUsageMetricRollupOnce(ctx context.Context) error {
	if !model.UsageMetricWorkerEnabled() {
		return errors.New("usage metric worker is disabled")
	}
	if !common.IsMasterNode {
		return errors.New("usage metric worker requires the master node")
	}
	if model.LOG_DB == nil {
		return errors.New("log database is unavailable")
	}
	if !usageMetricWorkerRunning.CompareAndSwap(false, true) {
		return errors.New("usage metric worker is already running")
	}
	defer usageMetricWorkerRunning.Store(false)
	if !common.BackgroundWorkAllowed() {
		return errors.New("usage metric worker deferred while system protection is active")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// The scheduled wrapper already supplies this deadline. Apply the same
	// upper bound to explicit maintenance calls so an operator cannot start an
	// unbounded rebuild against a large log table.
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > usageMetricWorkerTimeout {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, usageMetricWorkerTimeout)
		defer cancel()
	}
	return runUsageMetricRollup(ctx)
}

func runUsageMetricRollupOnce() {
	if !usageMetricWorkerRunning.CompareAndSwap(false, true) {
		return
	}
	defer usageMetricWorkerRunning.Store(false)
	if !common.BackgroundWorkAllowed() {
		logger.LogInfo(context.Background(), "usage metric rollup deferred while system protection is active")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), usageMetricWorkerTimeout)
	defer cancel()
	if err := runUsageMetricRollup(ctx); err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("usage metric rollup failed: %v", err))
	}
}

func runUsageMetricRollup(ctx context.Context) error {
	location, timezone, err := model.UsageMetricLocation()
	if err != nil {
		return fmt.Errorf("load usage metric timezone: %w", err)
	}
	checkpoint, err := model.LoadUsageMetricCheckpoint(ctx, model.LOG_DB, usageMetricWorkerCheckpointName)
	if err != nil {
		return fmt.Errorf("load usage metric checkpoint: %w", err)
	}
	if checkpoint.Name == "" {
		checkpoint.Name = usageMetricWorkerCheckpointName
	}

	logs, err := readUsageMetricSourceBatch(ctx, checkpoint.Watermark, usageMetricWorkerBatchSize)
	if err != nil {
		return fmt.Errorf("read usage metric source batch: %w", err)
	}
	processedSource := len(logs) > 0
	// A short batch proves there was no additional eligible source row at the
	// time of the indexed read. Empty-bucket coverage must only be produced
	// when this is true; otherwise a row still waiting behind the checkpoint
	// could be mistaken for an empty hour.
	sourceCaughtUp := len(logs) < usageMetricWorkerBatchSize
	if processedSource {
		maxID := int64(logs[len(logs)-1].Id)
		if err := applyUsageMetricSourceBatch(ctx, location, timezone, logs, checkpoint.Watermark, maxID); err != nil {
			return err
		}
		checkpoint.Watermark = maxID
	} else {
		// The source batch intentionally filters out non-usage/audit rows. Move
		// past an idle run of those rows so the worker does not rescan the same
		// top-up/login records forever while preserving the eligible-log order.
		maxID, idErr := latestUsageMetricSourceID(ctx, checkpoint.Watermark)
		if idErr != nil {
			return fmt.Errorf("read usage metric source watermark: %w", idErr)
		}
		checkpoint.Watermark = maxID
	}

	now := time.Now().Unix()
	// Persist source progress before optional reconciliation. A slow/failed
	// reconciliation must not replay an already-applied source batch on the next
	// minute and multiply the same bucket scans.
	checkpoint.UpdatedAt = now
	if err := model.SaveUsageMetricCheckpoint(ctx, model.LOG_DB, checkpoint); err != nil {
		return fmt.Errorf("save usage metric checkpoint: %w", err)
	}
	// Reconciliation is wall-clock scheduled, not idle scheduled.  A busy log
	// stream can keep producing source batches forever; waiting for an idle
	// tick would leave previously open buckets incomplete indefinitely and make
	// the historical projection unusable.  The rebuild itself is bounded and
	// still runs behind the same background-work gate and context deadline.
	if usageMetricReconcileDue(now, checkpoint.LastReconciledAt) {
		if err := reconcileUsageMetricWindows(ctx, location, timezone, checkpoint.Watermark, now); err != nil {
			return err
		}
		if sourceCaughtUp {
			if err := fillMissingCurrentDayHourCoverage(ctx, location, timezone, checkpoint.Watermark, now, usageMetricWorkerEmptyHoursMax); err != nil {
				return fmt.Errorf("fill empty usage metric hours: %w", err)
			}
		}
		checkpoint.LastReconciledAt = now
		checkpoint.UpdatedAt = time.Now().Unix()
		if err := model.SaveUsageMetricCheckpoint(ctx, model.LOG_DB, checkpoint); err != nil {
			return fmt.Errorf("save usage metric reconcile checkpoint: %w", err)
		}
	}
	return nil
}

func usageMetricReconcileDue(now, lastReconciledAt int64) bool {
	if now <= 0 {
		return false
	}
	if lastReconciledAt <= 0 {
		return true
	}
	return now-lastReconciledAt >= int64(usageMetricWorkerReconcileEvery/time.Second)
}

func usageMetricSourceSelect() string {
	groupColumn := "`group`"
	groupAlias := "`group`"
	if common.UsingPostgreSQL {
		groupColumn = `"group"`
		groupAlias = `"group"`
	}
	return strings.Join([]string{
		"id", "user_id", "created_at", "type", "model_name", "quota", "pre_discount_quota",
		"prompt_tokens", "cache_tokens", "completion_tokens", "use_time", "is_stream", "channel_id",
		"token_id", groupColumn + " AS " + groupAlias, "settled", "cost", "paid_quota", "paid_gift_quota",
		"billing_source", "subscription_id",
	}, ", ")
}

func readUsageMetricSourceBatch(ctx context.Context, afterID int64, limit int) ([]model.Log, error) {
	if model.LOG_DB == nil {
		return nil, errors.New("log database is unavailable")
	}
	if limit <= 0 || limit > 5000 {
		limit = usageMetricWorkerBatchSize
	}
	var logs []model.Log
	err := model.LOG_DB.WithContext(ctx).Model(&model.Log{}).
		Select(usageMetricSourceSelect()).
		Where("id > ? AND type IN (?, ?)", afterID, model.LogTypeConsume, model.LogTypeError).
		Order("id ASC").Limit(limit).Find(&logs).Error
	return logs, err
}

func latestUsageMetricSourceID(ctx context.Context, afterID int64) (int64, error) {
	if model.LOG_DB == nil {
		return afterID, errors.New("log database is unavailable")
	}
	var maxID int64
	err := model.LOG_DB.WithContext(ctx).Model(&model.Log{}).
		Select("COALESCE(MAX(id), ?)", afterID).
		Scan(&maxID).Error
	if err != nil {
		return afterID, err
	}
	if maxID < afterID {
		return afterID, nil
	}
	return maxID, nil
}

// latestUsageMetricEligibleSourceID returns the newest source row that the
// projection worker is responsible for after afterID.  This deliberately
// differs from latestUsageMetricSourceID: the latter advances the checkpoint
// past top-up/login/audit rows, while empty-hour sealing must only be allowed
// when no new consume/error row arrived after the watermark.
func latestUsageMetricEligibleSourceID(ctx context.Context, afterID int64) (int64, error) {
	if model.LOG_DB == nil {
		return afterID, errors.New("log database is unavailable")
	}
	var maxID int64
	err := model.LOG_DB.WithContext(ctx).Model(&model.Log{}).
		Select("COALESCE(MAX(id), ?)", afterID).
		Where("id > ? AND type IN (?, ?)", afterID, model.LogTypeConsume, model.LogTypeError).
		Scan(&maxID).Error
	if err != nil {
		return afterID, err
	}
	if maxID < afterID {
		return afterID, nil
	}
	return maxID, nil
}

func applyUsageMetricSourceBatch(ctx context.Context, location *time.Location, timezone string, logs []model.Log, fromWatermark, toWatermark int64) error {
	if toWatermark < fromWatermark {
		return errors.New("usage metric source watermark moved backwards")
	}
	for _, granularity := range usageMetricWorkerProjectionGranularities {
		starts := make(map[int64]struct{})
		for _, log := range logs {
			if log.Type != model.LogTypeConsume && log.Type != model.LogTypeError {
				continue
			}
			start, err := model.UsageMetricBucketStart(log.CreatedAt, granularity, location)
			if err != nil {
				return err
			}
			starts[start] = struct{}{}
		}
		if len(starts) == 0 {
			continue
		}
		ordered := sortedUsageMetricStarts(starts)
		if err := rebuildUsageMetricBuckets(ctx, location, timezone, granularity, ordered, toWatermark, time.Now().Unix()); err != nil {
			return fmt.Errorf("rebuild %s usage metric buckets: %w", granularity, err)
		}
	}
	batch := model.UsageMetricBatch{
		BatchKey:      fmt.Sprintf("%s:%d:%d", usageMetricWorkerSourceVersion, fromWatermark, toWatermark),
		FromWatermark: fromWatermark,
		ToWatermark:   toWatermark,
		AppliedAt:     time.Now().Unix(),
	}
	if err := model.RecordUsageMetricBatch(ctx, model.LOG_DB, batch); err != nil {
		return fmt.Errorf("record usage metric batch: %w", err)
	}
	return nil
}

func sortedUsageMetricStarts(values map[int64]struct{}) []int64 {
	result := make([]int64, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func usageMetricBucketEnd(start int64, granularity model.UsageMetricGranularity, location *time.Location) (int64, error) {
	if location == nil {
		return 0, errors.New("usage metric location is required")
	}
	value := time.Unix(start, 0).In(location)
	switch granularity {
	case model.UsageMetricGranularityMinute:
		return value.Add(time.Minute).Unix(), nil
	case model.UsageMetricGranularityHour:
		return value.Add(time.Hour).Unix(), nil
	case model.UsageMetricGranularityDay:
		next := time.Date(value.Year(), value.Month(), value.Day()+1, 0, 0, 0, 0, location)
		return next.Unix(), nil
	default:
		return 0, fmt.Errorf("unsupported usage metric granularity %q", granularity)
	}
}

type usageMetricRebuildWindow struct {
	start int64
	end   int64
	rows  int64
}

func usageMetricCoverageForWindow(granularity model.UsageMetricGranularity, start, end int64, timezone string, computedAt, watermark int64, complete bool) model.UsageMetricCoverage {
	return model.UsageMetricCoverage{
		Granularity:   granularity,
		BucketStart:   start,
		BucketEnd:     end,
		Timezone:      timezone,
		ComputedAt:    computedAt,
		Watermark:     watermark,
		SourceVersion: usageMetricWorkerSourceVersion,
		IsComplete:    complete,
	}
}

func countUsageMetricBucketLogs(ctx context.Context, granularity model.UsageMetricGranularity, start, end, watermark int64) (int64, error) {
	if model.LOG_DB == nil {
		return 0, errors.New("log database is unavailable")
	}
	// Keep granularity in the signature so the row-budget planner remains
	// explicit about which projection it is protecting. The source predicate is
	// the same for all granularities.
	_ = granularity
	var count int64
	query := model.LOG_DB.WithContext(ctx).Model(&model.Log{}).
		Where("created_at >= ? AND created_at < ? AND type IN (?, ?)", start, end, model.LogTypeConsume, model.LogTypeError)
	if watermark > 0 {
		query = query.Where("id <= ?", watermark)
	}
	err := query.
		Count(&count).Error
	return count, err
}

// fillMissingCurrentDayHourCoverage records explicit empty-hour markers for a
// small, bounded set of sealed hours.  It is intentionally limited to the
// current local day, where detailed retention cannot have moved source rows to
// usage_log_daily_aggregates.  Existing coverage or bucket rows are never
// overwritten: an old snapshot with no source rows is ambiguous and must keep
// failing closed until a normal rebuild can prove its replacement.
func fillMissingCurrentDayHourCoverage(ctx context.Context, location *time.Location, timezone string, watermark, now int64, limit int) error {
	if model.LOG_DB == nil {
		return errors.New("log database is unavailable")
	}
	if location == nil || now <= 0 || watermark < 0 || limit <= 0 {
		return nil
	}
	localNow := time.Unix(now, 0).In(location)
	dayStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location).Unix()
	currentHourStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), localNow.Hour(), 0, 0, 0, location).Unix()
	if currentHourStart <= dayStart {
		return nil
	}
	starts := make([]int64, 0, 24)
	for cursor := dayStart; cursor < currentHourStart; {
		end, err := usageMetricBucketEnd(cursor, model.UsageMetricGranularityHour, location)
		if err != nil {
			return err
		}
		// Match the reader/writer seal delay. The immediately previous hour can
		// still be inside its one-minute late-write window.
		if end <= now-60 {
			starts = append(starts, cursor)
		}
		cursor = end
	}
	if len(starts) == 0 {
		return nil
	}

	// The source batch can become stale while the worker is applying snapshots.
	// Do not turn an hour into a complete empty marker when a new eligible log
	// has arrived after the checkpoint; the next pass must rebuild that hour
	// from the raw source instead. This check is repeated immediately before the
	// write below because the bounded count probes themselves may take time.
	latestEligibleID, err := latestUsageMetricEligibleSourceID(ctx, watermark)
	if err != nil {
		return err
	}
	if latestEligibleID > watermark {
		return nil
	}

	existing := make(map[int64]struct{}, len(starts))
	var coverageStarts []int64
	if err := model.LOG_DB.WithContext(ctx).Model(&model.UsageMetricCoverage{}).
		Where("granularity = ? AND bucket_start IN ?", model.UsageMetricGranularityHour, starts).
		Pluck("bucket_start", &coverageStarts).Error; err != nil {
		return err
	}
	for _, start := range coverageStarts {
		existing[start] = struct{}{}
	}
	var bucketStarts []int64
	if err := model.LOG_DB.WithContext(ctx).Model(&model.UsageMetricBucket{}).
		Distinct("bucket_start").
		Where("granularity = ? AND bucket_start IN ?", model.UsageMetricGranularityHour, starts).
		Pluck("bucket_start", &bucketStarts).Error; err != nil {
		return err
	}
	for _, start := range bucketStarts {
		existing[start] = struct{}{}
	}

	coverage := make([]model.UsageMetricCoverage, 0, limit)
	checked := 0
	for _, start := range starts {
		if _, found := existing[start]; found {
			continue
		}
		if checked >= limit {
			break
		}
		checked++
		end, err := usageMetricBucketEnd(start, model.UsageMetricGranularityHour, location)
		if err != nil {
			return err
		}
		countCtx := ctx
		cancel := func() {}
		if usageMetricWorkerCountTimeout > 0 {
			countCtx, cancel = context.WithTimeout(ctx, usageMetricWorkerCountTimeout)
		}
		rowCount, countErr := countUsageMetricBucketLogs(countCtx, model.UsageMetricGranularityHour, start, end, watermark)
		cancel()
		if countErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(countErr, context.DeadlineExceeded) {
				continue
			}
			return countErr
		}
		if rowCount != 0 {
			continue
		}
		coverage = append(coverage, usageMetricCoverageForWindow(
			model.UsageMetricGranularityHour, start, end, timezone, now, watermark, true,
		))
	}
	// Re-check the indexed source watermark directly before sealing. A source
	// row that arrived during the count probes makes the result ambiguous, so
	// leave all markers untouched and let the next worker pass reconcile them.
	latestEligibleID, err = latestUsageMetricEligibleSourceID(ctx, watermark)
	if err != nil {
		return err
	}
	if latestEligibleID > watermark {
		return nil
	}
	return model.UpsertUsageMetricCoverage(ctx, model.LOG_DB, coverage)
}

func rebuildUsageMetricBuckets(ctx context.Context, location *time.Location, timezone string, granularity model.UsageMetricGranularity, starts []int64, watermark, now int64) error {
	if len(starts) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// Preflight each window using the indexed timestamp/type predicate. This
	// keeps the expensive source read bounded and lets the worker advance past a
	// pathological day while explicitly making that projection unavailable.
	windows := make([]usageMetricRebuildWindow, 0, len(starts))
	skippedCoverage := make([]model.UsageMetricCoverage, 0)
	seenStarts := make(map[int64]struct{}, len(starts))
	var selectedRows int64
	rebuildDeadline := time.Time{}
	if usageMetricWorkerRebuildBudget > 0 {
		rebuildDeadline = time.Now().Add(usageMetricWorkerRebuildBudget)
	}
	retentionCutoff := now - int64(model.DetailedUsageLogRetentionDays*24*60*60)
	for _, start := range starts {
		if _, seen := seenStarts[start]; seen {
			continue
		}
		seenStarts[start] = struct{}{}
		if err := ctx.Err(); err != nil {
			return err
		}
		end, err := usageMetricBucketEnd(start, granularity, location)
		if err != nil {
			return err
		}
		// Open buckets are intentionally left incomplete. Readers already merge
		// exact raw tails, so rebuilding a hot hour/day on every source batch only
		// creates write amplification.
		if end > now-60 {
			skippedCoverage = append(skippedCoverage, usageMetricCoverageForWindow(granularity, start, end, timezone, now, watermark, false))
			continue
		}
		// Detailed rows older than the retention horizon may already have been
		// compacted into usage_log_daily_aggregates. This worker cannot safely
		// reconstruct such a bucket from logs alone, so preserve any existing
		// projection and fail closed to the archive/raw reader.
		if start < retentionCutoff {
			skippedCoverage = append(skippedCoverage, usageMetricCoverageForWindow(granularity, start, end, timezone, now, watermark, false))
			continue
		}
		if !rebuildDeadline.IsZero() && !time.Now().Before(rebuildDeadline) {
			skippedCoverage = append(skippedCoverage, usageMetricCoverageForWindow(granularity, start, end, timezone, now, watermark, false))
			continue
		}
		countCtx := ctx
		cancelCount := func() {}
		if usageMetricWorkerCountTimeout > 0 {
			timeout := usageMetricWorkerCountTimeout
			if !rebuildDeadline.IsZero() {
				remaining := time.Until(rebuildDeadline)
				if remaining <= 0 {
					skippedCoverage = append(skippedCoverage, usageMetricCoverageForWindow(granularity, start, end, timezone, now, watermark, false))
					continue
				}
				if remaining < timeout {
					timeout = remaining
				}
			}
			countCtx, cancelCount = context.WithTimeout(ctx, timeout)
		}
		rowCount, countErr := countUsageMetricBucketLogs(countCtx, granularity, start, end, watermark)
		cancelCount()
		if countErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// A child timeout is a budget miss, not a fatal worker error. Keep the
			// old snapshot and mark coverage incomplete so source progress can still
			// advance.
			if errors.Is(countErr, context.DeadlineExceeded) {
				skippedCoverage = append(skippedCoverage, usageMetricCoverageForWindow(granularity, start, end, timezone, now, watermark, false))
				continue
			}
			return countErr
		}
		if usageMetricWorkerMaxRowsPerBucket > 0 && rowCount > usageMetricWorkerMaxRowsPerBucket {
			skippedCoverage = append(skippedCoverage, usageMetricCoverageForWindow(granularity, start, end, timezone, now, watermark, false))
			continue
		}
		// A zero-row source window is ambiguous once detailed retention/archive
		// has run (and can also result from a concurrent delete between the count
		// and the read). Never replace a previously valid snapshot with an empty
		// one; leave it intact and make the coverage fail closed.
		if rowCount == 0 {
			skippedCoverage = append(skippedCoverage, usageMetricCoverageForWindow(granularity, start, end, timezone, now, watermark, false))
			continue
		}
		if usageMetricWorkerMaxRowsPerRebuild > 0 && selectedRows > usageMetricWorkerMaxRowsPerRebuild-rowCount {
			skippedCoverage = append(skippedCoverage, usageMetricCoverageForWindow(granularity, start, end, timezone, now, watermark, false))
			continue
		}
		selectedRows += rowCount
		windows = append(windows, usageMetricRebuildWindow{start: start, end: end, rows: rowCount})
	}

	// Mark every planned window incomplete before reading. If the context is
	// cancelled or the replace fails, readers fail closed to the raw path rather
	// than observing a partially refreshed snapshot as complete.
	pendingCoverage := make([]model.UsageMetricCoverage, 0, len(windows)+len(skippedCoverage))
	for _, window := range windows {
		pendingCoverage = append(pendingCoverage, usageMetricCoverageForWindow(granularity, window.start, window.end, timezone, now, watermark, false))
	}
	pendingCoverage = append(pendingCoverage, skippedCoverage...)
	if err := model.UpsertUsageMetricCoverage(ctx, model.LOG_DB, pendingCoverage); err != nil {
		return err
	}
	if len(windows) == 0 {
		return nil
	}

	// Read all selected windows in one SQL result instead of issuing a full
	// source query once per bucket. BuildUsageMetricBuckets still creates exact
	// per-window/per-dimension snapshots, while the single query removes the
	// previous O(number-of-buckets * rows-per-bucket) scan pattern.
	readCtx := ctx
	cancelRead := func() {}
	if !rebuildDeadline.IsZero() {
		remaining := time.Until(rebuildDeadline)
		if remaining <= 0 {
			return nil
		}
		readCtx, cancelRead = context.WithTimeout(ctx, remaining)
	}
	logs, err := readUsageMetricBucketLogs(readCtx, granularity, windows, watermark)
	cancelRead()
	if err != nil {
		if ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
			return nil
		}
		return err
	}
	buckets, err := model.BuildUsageMetricBuckets(logs, granularity, location, usageMetricWorkerSourceVersion, now, watermark)
	if err != nil {
		return err
	}
	windowStarts := make([]int64, 0, len(windows))
	windowEnds := make(map[int64]int64, len(windows))
	for _, window := range windows {
		windowStarts = append(windowStarts, window.start)
		windowEnds[window.start] = window.end
	}
	for index := range buckets {
		end, ok := windowEnds[buckets[index].BucketStart]
		if !ok {
			return fmt.Errorf("usage metric source query returned an unexpected bucket: %d", buckets[index].BucketStart)
		}
		buckets[index].Timezone = timezone
		buckets[index].CoverageStart = buckets[index].BucketStart
		buckets[index].CoverageEnd = end
		buckets[index].IsComplete = end <= now-60
		buckets[index].RefreshDimensionHash()
	}
	if err := model.ReplaceUsageMetricBucketWindow(ctx, model.LOG_DB, granularity, windowStarts, buckets); err != nil {
		return err
	}
	coverage := make([]model.UsageMetricCoverage, 0, len(windows)+len(skippedCoverage))
	for _, window := range windows {
		coverage = append(coverage, usageMetricCoverageForWindow(granularity, window.start, window.end, timezone, now, watermark, window.end <= now-60))
	}
	coverage = append(coverage, skippedCoverage...)
	return model.UpsertUsageMetricCoverage(ctx, model.LOG_DB, coverage)
}

func readUsageMetricBucketLogs(ctx context.Context, granularity model.UsageMetricGranularity, windows []usageMetricRebuildWindow, watermark int64) ([]model.Log, error) {
	if model.LOG_DB == nil {
		return nil, errors.New("log database is unavailable")
	}
	if len(windows) == 0 {
		return nil, nil
	}
	// The granularity argument is intentionally part of the signature so a
	// future source planner can choose a rollup instead of raw logs. Today the
	// worker reads all selected windows in one SQL result.
	_ = granularity
	conditions := make([]string, 0, len(windows))
	args := make([]interface{}, 0, len(windows)*2+3)
	for _, window := range windows {
		conditions = append(conditions, "(created_at >= ? AND created_at < ?)")
		args = append(args, window.start, window.end)
	}
	where := "(" + strings.Join(conditions, " OR ") + ") AND type IN (?, ?)"
	args = append(args, model.LogTypeConsume, model.LogTypeError)
	if watermark > 0 {
		where += " AND id <= ?"
		args = append(args, watermark)
	}
	var logs []model.Log
	err := model.LOG_DB.WithContext(ctx).Model(&model.Log{}).
		Select(usageMetricSourceSelect()).
		Where(where, args...).
		Order("id ASC").Find(&logs).Error
	return logs, err
}

func reconcileUsageMetricWindows(ctx context.Context, location *time.Location, timezone string, watermark, now int64) error {
	for _, item := range []struct {
		granularity model.UsageMetricGranularity
		start       int64
		count       int
	}{
		{model.UsageMetricGranularityHour, now - usageMetricWorkerReconcileHours*60*60, usageMetricWorkerReconcileHours},
		{model.UsageMetricGranularityDay, now - usageMetricWorkerReconcileHours*60*60, 3},
	} {
		first, err := model.UsageMetricBucketStart(item.start, item.granularity, location)
		if err != nil {
			return err
		}
		starts := make([]int64, 0, item.count)
		cursor := first
		for i := 0; i < item.count && i < usageMetricWorkerReconcileMaxEach; i++ {
			starts = append(starts, cursor)
			cursor, err = usageMetricBucketEnd(cursor, item.granularity, location)
			if err != nil {
				return err
			}
		}
		if err := rebuildUsageMetricBuckets(ctx, location, timezone, item.granularity, starts, watermark, now); err != nil {
			return fmt.Errorf("reconcile %s usage metric buckets: %w", item.granularity, err)
		}
	}
	return nil
}
