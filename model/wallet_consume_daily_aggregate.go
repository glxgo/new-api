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
package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// WalletConsumeAggregateSourceVersion identifies the shape and accounting
// rules of the wallet financial-flow projection.  Bumping the version is an
// explicit rebuild decision; readers fail closed when the marker version does
// not match.
const WalletConsumeAggregateSourceVersion = "wallet-consume-v2"

const (
	WalletConsumeAggregateWorkerEnableEnv = "WALLET_CONSUME_AGGREGATE_WORKER_ENABLE"
	WalletConsumeAggregateReadEnableEnv   = "WALLET_CONSUME_AGGREGATE_READ_ENABLE"
)

// ErrWalletConsumeAggregateSourceOverlap means raw logs and archived daily
// rows cannot be proven disjoint for a day.  The projection builder fails
// closed on this condition and invalidates that day's coverage marker rather
// than risking a double-counted financial total.
var ErrWalletConsumeAggregateSourceOverlap = errors.New("wallet consume aggregate source overlap")

// WalletConsumeDailyAggregate is a compact, one-row-per-user-per-local-day
// projection for the authenticated wallet financial-flow endpoint.  It is a
// query acceleration layer only: the logs table remains the audit source of
// truth and is used by the rebuild/fallback paths.
//
// The projection intentionally includes every type=consume row regardless of
// billing_source.  That preserves the existing financial-flow response
// semantics (which predate billing_source and therefore included all consume
// rows).  If a wallet-only view is introduced later, it should use a separate
// versioned projection rather than silently changing this contract.
type WalletConsumeDailyAggregate struct {
	Id           int    `json:"id" gorm:"primaryKey"`
	UserId       int    `json:"user_id" gorm:"not null;uniqueIndex:uk_wallet_consume_daily,priority:1;index:idx_wallet_consume_daily_range,priority:1"`
	DayStart     int64  `json:"day_start" gorm:"type:bigint;not null;uniqueIndex:uk_wallet_consume_daily,priority:2;index:idx_wallet_consume_daily_range,priority:2"`
	RequestCount int64  `json:"request_count" gorm:"type:bigint;not null;default:0"`
	Quota        int64  `json:"quota" gorm:"type:bigint;not null;default:0"`
	LastLogAt    int64  `json:"last_log_at" gorm:"type:bigint;not null;default:0"`
	LastLogId    int64  `json:"last_log_id" gorm:"type:bigint;not null;default:0"`
	BalanceAfter *int64 `json:"balance_after" gorm:"default:null"`

	// Projection metadata makes partial/failed rebuilds fail closed instead of
	// returning a deceptively complete but stale day.
	ComputedAt    int64  `json:"computed_at" gorm:"type:bigint;not null;default:0"`
	Watermark     int64  `json:"watermark" gorm:"type:bigint;not null;default:0"`
	SourceVersion string `json:"source_version" gorm:"type:varchar(128);not null;default:''"`
}

func (WalletConsumeDailyAggregate) TableName() string {
	return "wallet_consume_daily_aggregates"
}

// WalletConsumeDailyAggregateCoverage distinguishes an empty, successfully
// rebuilt day from a day the worker has not reached.  Without this marker a
// missing per-user row could mean either “the user had no usage” or “the
// projection is incomplete”.
type WalletConsumeDailyAggregateCoverage struct {
	Id            int    `json:"id" gorm:"primaryKey"`
	DayStart      int64  `json:"day_start" gorm:"type:bigint;not null;uniqueIndex"`
	DayEnd        int64  `json:"day_end" gorm:"type:bigint;not null;default:0"`
	Timezone      string `json:"timezone" gorm:"type:varchar(64);not null;default:''"`
	ComputedAt    int64  `json:"computed_at" gorm:"type:bigint;not null;default:0"`
	Watermark     int64  `json:"watermark" gorm:"type:bigint;not null;default:0"`
	SourceVersion string `json:"source_version" gorm:"type:varchar(128);not null;default:''"`
	IsComplete    bool   `json:"is_complete" gorm:"not null;default:false;index"`
}

func (WalletConsumeDailyAggregateCoverage) TableName() string {
	return "wallet_consume_daily_aggregate_coverage"
}

// WalletConsumeAggregateCheckpoint stores the next local day to process for
// the optional maintenance worker.  It is deliberately independent from the
// usage-metric log-id checkpoint: wallet rows are day snapshots and can be
// rebuilt safely even when the usage-metric worker is disabled or paused.
type WalletConsumeAggregateCheckpoint struct {
	Id               int    `json:"id" gorm:"primaryKey"`
	Name             string `json:"name" gorm:"type:varchar(128);not null;uniqueIndex"`
	NextDayStart     int64  `json:"next_day_start" gorm:"type:bigint;not null;default:0"`
	LastReconciledAt int64  `json:"last_reconciled_at" gorm:"type:bigint;not null;default:0"`
	UpdatedAt        int64  `json:"updated_at" gorm:"type:bigint;not null;default:0"`
}

func (WalletConsumeAggregateCheckpoint) TableName() string {
	return "wallet_consume_aggregate_checkpoints"
}

// The checkpoint name is versioned with the projection source.  A new source
// version must start from a fresh bounded lookback instead of accidentally
// inheriting a v1 watermark whose coverage rows are no longer readable by the
// v2 reader.
const walletConsumeAggregateCheckpointName = "wallet-consume-daily-v2"

// WalletConsumeAggregateWorkerEnabled and WalletConsumeAggregateReadEnabled
// are opt-in independently.  A schema migration alone therefore cannot add
// background scans or change an existing endpoint's source of data.
func WalletConsumeAggregateWorkerEnabled() bool {
	return common.GetEnvOrDefaultBool(WalletConsumeAggregateWorkerEnableEnv, false)
}

func WalletConsumeAggregateReadEnabled() bool {
	return common.GetEnvOrDefaultBool(WalletConsumeAggregateReadEnableEnv, false)
}

func walletConsumeTimezoneName() string {
	if time.Local == nil {
		return "UTC"
	}
	name := strings.TrimSpace(time.Local.String())
	if name == "" {
		return "Local"
	}
	return name
}

func walletConsumeDayStart(timestamp int64) int64 {
	value := time.Unix(timestamp, 0).In(time.Local)
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.Local).Unix()
}

func walletConsumeDayEnd(dayStart int64) int64 {
	value := time.Unix(dayStart, 0).In(time.Local)
	return time.Date(value.Year(), value.Month(), value.Day()+1, 0, 0, 0, 0, time.Local).Unix()
}

type walletConsumeDayPart struct {
	Start int64
	End   int64
	Full  bool
}

func walletConsumeDayParts(startTimestamp, endTimestamp int64) ([]walletConsumeDayPart, error) {
	if startTimestamp <= 0 || endTimestamp <= startTimestamp {
		return nil, nil
	}
	if time.Local == nil {
		return nil, errors.New("local timezone is unavailable")
	}
	parts := make([]walletConsumeDayPart, 0, 33)
	cursor := walletConsumeDayStart(startTimestamp)
	for cursor < endTimestamp {
		next := walletConsumeDayEnd(cursor)
		if next <= cursor {
			return nil, errors.New("wallet consume day boundary did not advance")
		}
		overlapStart := startTimestamp
		if overlapStart < cursor {
			overlapStart = cursor
		}
		overlapEnd := endTimestamp
		if overlapEnd > next {
			overlapEnd = next
		}
		if overlapStart < overlapEnd {
			parts = append(parts, walletConsumeDayPart{
				Start: overlapStart,
				End:   overlapEnd,
				Full:  overlapStart == cursor && overlapEnd == next,
			})
		}
		cursor = next
		if len(parts) > 10000 {
			return nil, errors.New("wallet consume range is too large")
		}
	}
	return parts, nil
}

type walletConsumeAggregateRow struct {
	UserId       int           `gorm:"column:user_id"`
	FirstLogAt   int64         `gorm:"column:first_log_at"`
	CreatedAt    int64         `gorm:"column:created_at"`
	SourceId     int64         `gorm:"column:source_id"`
	SourceKind   int           `gorm:"column:source_kind"`
	RequestCount int64         `gorm:"column:request_count"`
	Quota        int64         `gorm:"column:quota"`
	BalanceAfter sql.NullInt64 `gorm:"column:balance_after"`
}

// walletConsumeArchiveSourceIDExpression keeps reads compatible with an
// installation whose pre-v2 archive table has not yet gained last_log_id.
// New archives always populate the source ID; old rows conservatively fall
// back to their archive-row ID until they are rebuilt.
var walletConsumeArchiveSchemaCache struct {
	sync.Mutex
	db           *gorm.DB
	checked      bool
	hasLastLogId bool
}

func walletConsumeArchiveSourceIDExpression(db *gorm.DB) string {
	if walletConsumeArchiveHasLastLogId(db) {
		return "COALESCE(NULLIF(last_log_id, 0), id)"
	}
	return "id"
}

func walletConsumeArchiveHasLastLogId(db *gorm.DB) bool {
	if db == nil {
		return false
	}
	walletConsumeArchiveSchemaCache.Lock()
	defer walletConsumeArchiveSchemaCache.Unlock()
	if walletConsumeArchiveSchemaCache.checked && walletConsumeArchiveSchemaCache.db == db {
		return walletConsumeArchiveSchemaCache.hasLastLogId
	}
	walletConsumeArchiveSchemaCache.db = db
	walletConsumeArchiveSchemaCache.checked = true
	walletConsumeArchiveSchemaCache.hasLastLogId = db.Migrator().HasColumn(&UsageLogDailyAggregate{}, "last_log_id")
	return walletConsumeArchiveSchemaCache.hasLastLogId
}

// invalidateWalletConsumeArchiveSchemaCache forgets a previous HasColumn
// result after an explicit schema migration.  In particular, a process may
// have served a legacy archive read (and cached false) before an operator runs
// the opt-in migration; without invalidation, subsequent retention writes
// would keep omitting last_log_id until the process restarted.
func invalidateWalletConsumeArchiveSchemaCache(db *gorm.DB) {
	walletConsumeArchiveSchemaCache.Lock()
	defer walletConsumeArchiveSchemaCache.Unlock()
	if db == nil || walletConsumeArchiveSchemaCache.db == db {
		walletConsumeArchiveSchemaCache.db = nil
		walletConsumeArchiveSchemaCache.checked = false
		walletConsumeArchiveSchemaCache.hasLastLogId = false
	}
}

// walletConsumeSourceIntervalsOverlap reports whether an archived segment can
// overlap the raw rows that are still present for the same user/day. Archive
// rows from older schemas may not retain individual log IDs, so an interval
// intersection is intentionally treated as ambiguous and must fail closed
// (rebuild) or be skipped (legacy reader). Invalid archive bounds are
// ambiguous too.
func walletConsumeSourceIntervalsOverlap(archiveFirst, archiveLast, rawFirst, rawLast int64) bool {
	if archiveFirst <= 0 || archiveLast <= 0 || archiveFirst > archiveLast || rawFirst <= 0 || rawLast <= 0 || rawFirst > rawLast {
		return true
	}
	return archiveFirst <= rawLast && archiveLast >= rawFirst
}

type walletConsumeAggregateAccumulator struct {
	UserId       int
	RequestCount int64
	Quota        int64
	LastLogAt    int64
	LastLogId    int64
	LastKind     int
	BalanceAfter *int64
}

func (acc *walletConsumeAggregateAccumulator) add(row walletConsumeAggregateRow) {
	if acc == nil {
		return
	}
	requestCount := row.RequestCount
	if requestCount <= 0 {
		requestCount = 1
	}
	acc.RequestCount += requestCount
	acc.Quota += row.Quota
	// Raw logs use SourceKind=0 and their real log id.  Archived daily rows
	// use SourceKind=1 and the archive-row id as a deterministic tie breaker;
	// normal operation never has raw/archive overlap because archiving deletes
	// source rows transactionally.
	newer := row.CreatedAt > acc.LastLogAt ||
		(row.CreatedAt == acc.LastLogAt && (row.SourceKind > acc.LastKind ||
			(row.SourceKind == acc.LastKind && row.SourceId > acc.LastLogId)))
	if newer {
		acc.LastLogAt = row.CreatedAt
		acc.LastLogId = row.SourceId
		acc.LastKind = row.SourceKind
		if row.BalanceAfter.Valid {
			value := row.BalanceAfter.Int64
			acc.BalanceAfter = &value
		} else {
			// A valid latest operation with no snapshot must clear an older
			// snapshot.  This keeps NULL distinct from a real zero balance.
			acc.BalanceAfter = nil
		}
	}
}

func (acc *walletConsumeAggregateAccumulator) toModel(dayStart int64, computedAt, watermark int64) WalletConsumeDailyAggregate {
	result := WalletConsumeDailyAggregate{
		UserId:        acc.UserId,
		DayStart:      dayStart,
		RequestCount:  acc.RequestCount,
		Quota:         acc.Quota,
		LastLogAt:     acc.LastLogAt,
		LastLogId:     acc.LastLogId,
		ComputedAt:    computedAt,
		Watermark:     watermark,
		SourceVersion: WalletConsumeAggregateSourceVersion,
	}
	if acc.BalanceAfter != nil {
		value := *acc.BalanceAfter
		result.BalanceAfter = &value
	}
	return result
}

// readWalletConsumeSourceRows streams one day from the raw log table and, when
// present, the compact archive table.  Keeping source reads bounded to one day
// lets the maintenance caller apply a short context deadline and prevents an
// unusually busy user from turning a background pass into an unbounded load.
func readWalletConsumeSourceRows(ctx context.Context, db *gorm.DB, dayStart, dayEnd int64, visit func(walletConsumeAggregateRow) error) (int64, error) {
	if db == nil {
		return 0, errors.New("log database is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var visited int64
	read := func(query *gorm.DB, rowVisit func(walletConsumeAggregateRow) error) error {
		rows, err := query.Rows()
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var row walletConsumeAggregateRow
			if err := db.ScanRows(rows, &row); err != nil {
				return err
			}
			visited++
			if err := rowVisit(row); err != nil {
				return err
			}
		}
		return rows.Err()
	}

	type rawSourceBounds struct {
		First int64
		Last  int64
	}
	rawBoundsByUser := make(map[int]rawSourceBounds)
	if err := read(db.WithContext(ctx).Raw(`
		SELECT id AS source_id, user_id, created_at AS first_log_at, created_at, 1 AS request_count, quota, balance_after,
			0 AS source_kind
		FROM logs
		WHERE type = ? AND created_at >= ? AND created_at < ?
		ORDER BY user_id ASC, created_at ASC, id ASC`, LogTypeConsume, dayStart, dayEnd), func(row walletConsumeAggregateRow) error {
		if row.UserId > 0 {
			bounds := rawBoundsByUser[row.UserId]
			if bounds.First == 0 || row.FirstLogAt < bounds.First {
				bounds.First = row.FirstLogAt
			}
			if row.CreatedAt > bounds.Last {
				bounds.Last = row.CreatedAt
			}
			rawBoundsByUser[row.UserId] = bounds
		}
		return visit(row)
	}); err != nil {
		return visited, err
	}

	// Older deployments may not have run the explicit archive migration yet.
	// The raw source is still sufficient; absence of the archive table must not
	// make the maintenance hook or the read fallback fail.
	if hasTable := db.Migrator().HasTable(&UsageLogDailyAggregate{}); hasTable {
		overlap := false
		archiveSourceID := walletConsumeArchiveSourceIDExpression(db)
		if err := read(db.WithContext(ctx).Raw(fmt.Sprintf(`
			SELECT %s AS source_id, user_id, first_log_at, last_log_at AS created_at, request_count, quota, balance_after,
				1 AS source_kind
			FROM usage_log_daily_aggregates
			WHERE type = ? AND bucket_start = ?
			ORDER BY user_id ASC, last_log_at ASC, id ASC`, archiveSourceID), LogTypeConsume, dayStart), func(row walletConsumeAggregateRow) error {
			// If an archived segment intersects the raw timestamp envelope for
			// the same user, the source boundary is ambiguous. We cannot map
			// archive rows back to individual log IDs, so fail closed instead of
			// guessing and potentially charging the user twice.
			if rawBounds, ok := rawBoundsByUser[row.UserId]; ok && walletConsumeSourceIntervalsOverlap(row.FirstLogAt, row.CreatedAt, rawBounds.First, rawBounds.Last) {
				overlap = true
				return nil
			}
			return visit(row)
		}); err != nil {
			return visited, err
		}
		if overlap {
			return visited, ErrWalletConsumeAggregateSourceOverlap
		}
	}
	return visited, nil
}

// RebuildWalletConsumeDailyAggregates rebuilds complete local days in the
// dedicated projection.  It is an explicit maintenance hook; callers should
// pass a bounded context and a bounded day range.  Re-running the same range is
// snapshot replacement, not additive upsert, so retries are idempotent.
func RebuildWalletConsumeDailyAggregates(ctx context.Context, startTimestamp, endTimestamp int64) (int64, error) {
	if LOG_DB == nil {
		return 0, errors.New("log database is unavailable")
	}
	if startTimestamp <= 0 || endTimestamp <= startTimestamp {
		return 0, errors.New("invalid wallet consume aggregate range")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	startDay := walletConsumeDayStart(startTimestamp)
	endDay := walletConsumeDayStart(endTimestamp - 1)
	if endDay < startDay {
		return 0, errors.New("invalid wallet consume aggregate day range")
	}
	if startTimestamp != startDay || endTimestamp != walletConsumeDayEnd(endDay) {
		return 0, errors.New("wallet consume aggregate rebuild requires complete local days")
	}
	// Never seal the current local day. New consume rows can still arrive until
	// the day closes, and serving a snapshot marked complete for that day would
	// silently omit those rows. The wallet endpoint continues to use its exact
	// raw fallback for the open day.
	if endDay >= walletConsumeDayStart(time.Now().Unix()) {
		return 0, errors.New("wallet consume aggregate range includes the open day")
	}
	// A maintenance call is intentionally bounded.  The worker processes one
	// day at a time; an operator can invoke this hook repeatedly for a larger
	// historical backfill.
	if endDay-startDay > int64(31*24*60*60) {
		return 0, errors.New("wallet consume aggregate range exceeds 32 days")
	}

	dayCount := int64(0)
	for day := startDay; day <= endDay; day = walletConsumeDayEnd(day) {
		if err := ctx.Err(); err != nil {
			return dayCount, err
		}
		if err := rebuildWalletConsumeDailyAggregateDay(ctx, day); err != nil {
			return dayCount, err
		}
		dayCount++
		if day == endDay {
			break
		}
	}
	return dayCount, nil
}

func rebuildWalletConsumeDailyAggregateDay(ctx context.Context, dayStart int64) error {
	dayEnd := walletConsumeDayEnd(dayStart)
	computedAt := time.Now().Unix()
	grouped := make(map[int]*walletConsumeAggregateAccumulator)
	var watermark int64
	_, err := readWalletConsumeSourceRows(ctx, LOG_DB, dayStart, dayEnd, func(row walletConsumeAggregateRow) error {
		if row.CreatedAt <= 0 || row.UserId <= 0 {
			return nil
		}
		acc := grouped[row.UserId]
		if acc == nil {
			acc = &walletConsumeAggregateAccumulator{UserId: row.UserId}
			grouped[row.UserId] = acc
		}
		acc.add(row)
		if row.SourceKind == 0 && row.SourceId > watermark {
			watermark = row.SourceId
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrWalletConsumeAggregateSourceOverlap) {
			// Keep any old snapshot physically available for forensic inspection,
			// but make it unreadable by the projection path until an operator
			// resolves the source boundary and reruns the rebuild.
			invalidateResult := LOG_DB.WithContext(ctx).Model(&WalletConsumeDailyAggregateCoverage{}).
				Where("day_start = ?", dayStart).
				Updates(map[string]interface{}{"is_complete": false, "computed_at": time.Now().Unix()})
			if invalidateResult.Error != nil {
				// Preserve the overlap sentinel for callers that need to route the
				// day to the exact fallback, while surfacing an inability to mark
				// the stale projection unreadable. Silently dropping this error can
				// leave a previously complete snapshot serving duplicate totals.
				return fmt.Errorf("read wallet consume source day %d: %w; invalidate coverage: %v", dayStart, err, invalidateResult.Error)
			}
		}
		return fmt.Errorf("read wallet consume source day %d: %w", dayStart, err)
	}

	rows := make([]WalletConsumeDailyAggregate, 0, len(grouped))
	for _, acc := range grouped {
		rows = append(rows, acc.toModel(dayStart, computedAt, watermark))
	}
	return LOG_DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("day_start = ?", dayStart).Delete(&WalletConsumeDailyAggregate{}).Error; err != nil {
			return fmt.Errorf("delete wallet consume aggregate day %d: %w", dayStart, err)
		}
		if len(rows) > 0 {
			// SQLite cannot emit DEFAULT for a nullable pointer in a multi-row
			// INSERT. Split NULL snapshots out and insert those rows one at a
			// time; non-NULL rows still use bounded batches on all databases.
			withBalance := make([]WalletConsumeDailyAggregate, 0, len(rows))
			withoutBalance := make([]WalletConsumeDailyAggregate, 0)
			for _, row := range rows {
				if row.BalanceAfter == nil {
					withoutBalance = append(withoutBalance, row)
				} else {
					withBalance = append(withBalance, row)
				}
			}
			if len(withBalance) > 0 {
				if err := tx.CreateInBatches(&withBalance, 200).Error; err != nil {
					return fmt.Errorf("write wallet consume aggregate day %d: %w", dayStart, err)
				}
			}
			for index := range withoutBalance {
				if err := tx.Create(&withoutBalance[index]).Error; err != nil {
					return fmt.Errorf("write wallet consume aggregate day %d: %w", dayStart, err)
				}
			}
		}
		coverage := &WalletConsumeDailyAggregateCoverage{
			DayStart:      dayStart,
			DayEnd:        dayEnd,
			Timezone:      walletConsumeTimezoneName(),
			ComputedAt:    computedAt,
			Watermark:     watermark,
			SourceVersion: WalletConsumeAggregateSourceVersion,
			IsComplete:    true,
		}
		if err := tx.Where("day_start = ?", dayStart).Delete(&WalletConsumeDailyAggregateCoverage{}).Error; err != nil {
			return fmt.Errorf("delete wallet consume coverage day %d: %w", dayStart, err)
		}
		if err := tx.Create(coverage).Error; err != nil {
			return fmt.Errorf("write wallet consume coverage day %d: %w", dayStart, err)
		}
		return nil
	})
}

// loadWalletConsumeAggregateCoverageStatuses returns a readiness bit for each
// requested full day.  A missing, stale, or invalid marker makes only that
// day unavailable; it must not force already-covered days back onto the large
// raw-log query.  Database errors remain fatal to this probe so the caller can
// use the all-range legacy fallback and preserve correctness.
func loadWalletConsumeAggregateCoverageStatuses(ctx context.Context, starts []int64) (map[int64]bool, error) {
	statuses := make(map[int64]bool, len(starts))
	for _, start := range starts {
		statuses[start] = false
	}
	if len(starts) == 0 {
		return statuses, nil
	}
	if LOG_DB == nil {
		return nil, errors.New("log database is unavailable")
	}
	var rows []WalletConsumeDailyAggregateCoverage
	if err := LOG_DB.WithContext(ctx).
		Where("day_start IN ? AND is_complete = ? AND source_version = ? AND timezone = ?", starts, true, WalletConsumeAggregateSourceVersion, walletConsumeTimezoneName()).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	cutoff := time.Now().Unix() - 60
	seen := make(map[int64]bool, len(rows))
	ambiguous := make(map[int64]bool)
	for _, row := range rows {
		if _, requested := statuses[row.DayStart]; !requested {
			continue
		}
		if row.DayEnd != walletConsumeDayEnd(row.DayStart) || row.ComputedAt < row.DayEnd || !row.IsComplete || row.DayEnd > cutoff {
			continue
		}
		// The schema has a unique day key, but keep duplicate/ambiguous rows
		// fail-closed if a legacy import violates that invariant.
		if seen[row.DayStart] {
			ambiguous[row.DayStart] = true
			continue
		}
		seen[row.DayStart] = true
		statuses[row.DayStart] = true
	}
	for dayStart := range ambiguous {
		statuses[dayStart] = false
	}
	return statuses, nil
}

// loadWalletConsumeAggregateCoverage verifies that every full day in a read
// range has a complete marker from the current projection version/timezone.
// It is retained as a small compatibility helper for maintenance callers;
// wallet reads use the per-day status map above so a sparse backfill remains
// useful.
func loadWalletConsumeAggregateCoverage(ctx context.Context, starts []int64) (bool, error) {
	statuses, err := loadWalletConsumeAggregateCoverageStatuses(ctx, starts)
	if err != nil {
		return false, err
	}
	for _, start := range starts {
		if !statuses[start] {
			return false, nil
		}
	}
	return len(starts) > 0, nil
}

// mergeWalletConsumeLegacyParts coalesces adjacent partial/uncovered ranges.
// This keeps a sparse projection from issuing one raw-log query per day while
// never crossing a day that will be served by the aggregate table.
func mergeWalletConsumeLegacyParts(parts []walletConsumeDayPart) []walletConsumeDayPart {
	if len(parts) == 0 {
		return nil
	}
	merged := make([]walletConsumeDayPart, 0, len(parts))
	for _, part := range parts {
		if part.Start <= 0 || part.End <= part.Start {
			continue
		}
		if len(merged) > 0 && merged[len(merged)-1].End == part.Start {
			merged[len(merged)-1].End = part.End
			continue
		}
		merged = append(merged, walletConsumeDayPart{Start: part.Start, End: part.End})
	}
	return merged
}

// readWalletConsumeDailyAggregatesWithContext returns (items, ready, error).
// `ready=true` means the result was assembled successfully, possibly from a
// mix of covered projection days and bounded legacy ranges. `ready=false` is
// deliberately non-fatal and tells the caller to use the all-range legacy
// source path (for example when the projection query itself fails).
func readWalletConsumeDailyAggregatesWithContext(ctx context.Context, userId int, startTimestamp, endTimestamp int64) ([]FinancialConsumeDaily, bool, error) {
	if !WalletConsumeAggregateReadEnabled() || LOG_DB == nil || userId <= 0 {
		return nil, false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	parts, err := walletConsumeDayParts(startTimestamp, endTimestamp)
	if err != nil {
		return nil, false, err
	}
	fullStarts := make([]int64, 0, len(parts))
	for _, part := range parts {
		if part.Full {
			fullStarts = append(fullStarts, part.Start)
		}
	}
	if len(fullStarts) == 0 {
		return nil, false, nil
	}
	coverageReady, err := loadWalletConsumeAggregateCoverageStatuses(ctx, fullStarts)
	if err != nil {
		return nil, false, err
	}
	readyStarts := make([]int64, 0, len(fullStarts))
	legacyParts := make([]walletConsumeDayPart, 0, len(parts))
	for _, part := range parts {
		if part.Full && coverageReady[part.Start] {
			readyStarts = append(readyStarts, part.Start)
			continue
		}
		legacyParts = append(legacyParts, walletConsumeDayPart{Start: part.Start, End: part.End})
	}
	// If no full day is covered, preserve the old single-query behavior.  This
	// also avoids doing a legacy query here only for the caller to repeat it.
	if len(readyStarts) == 0 {
		return nil, false, nil
	}
	var rows []WalletConsumeDailyAggregate
	if err := LOG_DB.WithContext(ctx).Where("user_id = ? AND day_start IN ? AND source_version = ?", userId, readyStarts, WalletConsumeAggregateSourceVersion).Find(&rows).Error; err != nil {
		return nil, false, err
	}

	// Full-day rows are already aggregated.  Partial edges and uncovered full
	// days retain the old precise/fallback behavior, with adjacent legacy days
	// coalesced into one bounded query.  No legacy range crosses an aggregate-
	// ready day, so each calendar day has exactly one source.
	legacyRanges := mergeWalletConsumeLegacyParts(legacyParts)
	items := make([]FinancialConsumeDaily, 0, len(rows)+len(legacyRanges)*2)
	for _, row := range rows {
		item := FinancialConsumeDaily{DayStart: row.DayStart, Quota: row.Quota}
		if row.BalanceAfter != nil {
			value := *row.BalanceAfter
			item.BalanceAfter = &value
		}
		items = append(items, item)
	}
	for _, rangePart := range legacyRanges {
		partialItems, partialErr := getUserFinancialConsumeDailyLegacyWithContext(ctx, userId, rangePart.Start, rangePart.End)
		if partialErr != nil {
			return nil, false, partialErr
		}
		items = append(items, partialItems...)
	}
	// Aggregate rows and partial tails are separate query sources, so sort
	// explicitly to preserve the historical newest-day-first response contract.
	sort.Slice(items, func(i, j int) bool { return items[i].DayStart > items[j].DayStart })
	return items, true, nil
}

// LoadWalletConsumeAggregateCheckpoint and SaveWalletConsumeAggregateCheckpoint
// are small helpers used by the optional service worker and are exported so a
// reviewed local maintenance command can drive the same durable state.
func LoadWalletConsumeAggregateCheckpoint(ctx context.Context, db *gorm.DB) (WalletConsumeAggregateCheckpoint, error) {
	var checkpoint WalletConsumeAggregateCheckpoint
	if db == nil {
		return checkpoint, errors.New("log database is unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	err := db.WithContext(ctx).Where("name = ?", walletConsumeAggregateCheckpointName).First(&checkpoint).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return WalletConsumeAggregateCheckpoint{Name: walletConsumeAggregateCheckpointName}, nil
	}
	return checkpoint, err
}

func SaveWalletConsumeAggregateCheckpoint(ctx context.Context, db *gorm.DB, checkpoint WalletConsumeAggregateCheckpoint) error {
	if db == nil {
		return errors.New("log database is unavailable")
	}
	if strings.TrimSpace(checkpoint.Name) == "" {
		checkpoint.Name = walletConsumeAggregateCheckpointName
	}
	if checkpoint.NextDayStart < 0 || checkpoint.LastReconciledAt < 0 || checkpoint.UpdatedAt <= 0 {
		return errors.New("invalid wallet consume aggregate checkpoint")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	checkpoint.Id = 0
	return db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"next_day_start", "last_reconciled_at", "updated_at",
		}),
	}).Create(&checkpoint).Error
}
