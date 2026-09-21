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
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openWalletConsumeAggregateTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:wallet-consume-aggregate-%s?mode=memory&cache=shared", t.Name())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&Log{},
		&UsageLogDailyAggregate{},
		&WalletConsumeDailyAggregate{},
		&WalletConsumeDailyAggregateCoverage{},
		&WalletConsumeAggregateCheckpoint{},
	))
	previous := LOG_DB
	LOG_DB = db
	return db, func() { LOG_DB = previous }
}

func TestWalletConsumeDailyAggregateRebuildMatchesLegacyAndIsIdempotent(t *testing.T) {
	db, restore := openWalletConsumeAggregateTestDB(t)
	defer restore()
	t.Setenv(WalletConsumeAggregateReadEnableEnv, "true")

	day1 := time.Date(2025, 1, 10, 0, 0, 0, 0, time.Local).Unix()
	day2 := walletConsumeDayEnd(day1)
	day3 := walletConsumeDayEnd(day2)
	balance80, balance50, balance0 := int64(80), int64(50), int64(0)
	require.NoError(t, db.Create([]Log{
		{Id: 11, UserId: 7, Type: LogTypeConsume, CreatedAt: day1 + 10, Quota: 20, BalanceAfter: &balance80, BillingSource: "wallet"},
		{Id: 12, UserId: 7, Type: LogTypeConsume, CreatedAt: day1 + 20, Quota: 30, BalanceAfter: &balance50, BillingSource: "subscription"},
		{Id: 13, UserId: 7, Type: LogTypeConsume, CreatedAt: day2 + 10, Quota: 40, BalanceAfter: &balance0, BillingSource: "virtual_membership"},
		{Id: 14, UserId: 8, Type: LogTypeConsume, CreatedAt: day1 + 30, Quota: 999},
		{Id: 15, UserId: 7, Type: LogTypeTopup, CreatedAt: day1 + 40, Quota: 5000},
	}).Error)

	processed, err := RebuildWalletConsumeDailyAggregates(context.Background(), day1, day3)
	require.NoError(t, err)
	require.EqualValues(t, 2, processed)

	aggregated, err := GetUserFinancialConsumeDailyWithContext(context.Background(), 7, day1, day3)
	require.NoError(t, err)
	require.Equal(t, []FinancialConsumeDaily{
		{DayStart: day2, Quota: 40, BalanceAfter: &balance0},
		{DayStart: day1, Quota: 50, BalanceAfter: &balance50},
	}, aggregated)

	legacy, err := getUserFinancialConsumeDailyLegacyWithContext(context.Background(), 7, day1, day3)
	require.NoError(t, err)
	require.Equal(t, legacy, aggregated)

	var rowCount int64
	require.NoError(t, db.Model(&WalletConsumeDailyAggregate{}).Where("user_id = ?", 7).Count(&rowCount).Error)
	require.EqualValues(t, 2, rowCount)
	_, err = RebuildWalletConsumeDailyAggregates(context.Background(), day1, day3)
	require.NoError(t, err)
	var rowCountAfterRetry int64
	require.NoError(t, db.Model(&WalletConsumeDailyAggregate{}).Where("user_id = ?", 7).Count(&rowCountAfterRetry).Error)
	require.EqualValues(t, rowCount, rowCountAfterRetry)

	// A partial-day request has no complete projection bucket and therefore
	// falls back to the exact raw/archive implementation.
	partial, err := GetUserFinancialConsumeDailyWithContext(context.Background(), 7, day1+15, day2+15)
	require.NoError(t, err)
	require.Equal(t, []FinancialConsumeDaily{
		{DayStart: day2, Quota: 40, BalanceAfter: &balance0},
		{DayStart: day1, Quota: 30, BalanceAfter: &balance50},
	}, partial)
}

func TestWalletConsumeDailyAggregatePreservesNullAndZeroBalance(t *testing.T) {
	db, restore := openWalletConsumeAggregateTestDB(t)
	defer restore()
	t.Setenv(WalletConsumeAggregateReadEnableEnv, "true")

	day := time.Date(2025, 2, 3, 0, 0, 0, 0, time.Local).Unix()
	zero := int64(0)
	require.NoError(t, db.Create([]Log{
		{Id: 21, UserId: 9, Type: LogTypeConsume, CreatedAt: day + 10, Quota: 10, BalanceAfter: &zero},
		// The newest operation has no snapshot. It must clear the prior zero
		// pointer rather than accidentally retaining it.
		{Id: 22, UserId: 9, Type: LogTypeConsume, CreatedAt: day + 20, Quota: 5},
	}).Error)
	_, err := RebuildWalletConsumeDailyAggregates(context.Background(), day, walletConsumeDayEnd(day))
	require.NoError(t, err)
	items, err := GetUserFinancialConsumeDailyWithContext(context.Background(), 9, day, walletConsumeDayEnd(day))
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.EqualValues(t, 15, items[0].Quota)
	require.Nil(t, items[0].BalanceAfter)

	// Replacing the newest row with a real zero must preserve a non-nil pointer.
	require.NoError(t, db.Model(&Log{}).Where("id = ?", 22).Update("balance_after", 0).Error)
	_, err = RebuildWalletConsumeDailyAggregates(context.Background(), day, walletConsumeDayEnd(day))
	require.NoError(t, err)
	items, err = GetUserFinancialConsumeDailyWithContext(context.Background(), 9, day, walletConsumeDayEnd(day))
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.NotNil(t, items[0].BalanceAfter)
	require.EqualValues(t, 0, *items[0].BalanceAfter)
}

func TestWalletConsumeDailyAggregateReadFallsBackWhenCoverageMissing(t *testing.T) {
	db, restore := openWalletConsumeAggregateTestDB(t)
	defer restore()
	t.Setenv(WalletConsumeAggregateReadEnableEnv, "true")

	day := time.Date(2025, 3, 4, 0, 0, 0, 0, time.Local).Unix()
	balance := int64(7)
	require.NoError(t, db.Create(&Log{UserId: 10, Type: LogTypeConsume, CreatedAt: day + 10, Quota: 9, BalanceAfter: &balance}).Error)
	// Deliberately write a projection row without its complete coverage marker.
	require.NoError(t, db.Create(&WalletConsumeDailyAggregate{
		UserId: 10, DayStart: day, RequestCount: 1, Quota: 999,
		LastLogAt: day + 10, LastLogId: 1, BalanceAfter: &balance,
		ComputedAt: time.Now().Unix(), Watermark: 1, SourceVersion: WalletConsumeAggregateSourceVersion,
	}).Error)
	items, err := GetUserFinancialConsumeDailyWithContext(context.Background(), 10, day, walletConsumeDayEnd(day))
	require.NoError(t, err)
	require.Equal(t, []FinancialConsumeDaily{{DayStart: day, Quota: 9, BalanceAfter: &balance}}, items)
}

func TestWalletConsumeDailyAggregateReadFallsBackPerDay(t *testing.T) {
	db, restore := openWalletConsumeAggregateTestDB(t)
	defer restore()
	t.Setenv(WalletConsumeAggregateReadEnableEnv, "true")

	day1 := time.Date(2025, 3, 10, 0, 0, 0, 0, time.Local).Unix()
	day2 := walletConsumeDayEnd(day1)
	day3 := walletConsumeDayEnd(day2)
	day4 := walletConsumeDayEnd(day3)
	balance1, balance3 := int64(101), int64(303)
	rawBalance1, rawBalance2, rawBalance3 := int64(11), int64(22), int64(33)
	require.NoError(t, db.Create([]Log{
		{Id: 301, UserId: 17, Type: LogTypeConsume, CreatedAt: day1 + 10, Quota: 1, BalanceAfter: &rawBalance1},
		{Id: 302, UserId: 17, Type: LogTypeConsume, CreatedAt: day2 + 10, Quota: 2, BalanceAfter: &rawBalance2},
		{Id: 303, UserId: 17, Type: LogTypeConsume, CreatedAt: day3 + 10, Quota: 3, BalanceAfter: &rawBalance3},
	}).Error)
	// Deliberately make covered snapshots differ from the raw values. This
	// proves that day1/day3 come from the projection while the uncovered day2
	// alone uses the exact legacy path.
	require.NoError(t, db.Create([]WalletConsumeDailyAggregate{
		{UserId: 17, DayStart: day1, RequestCount: 9, Quota: 101, LastLogAt: day1 + 10, LastLogId: 301, BalanceAfter: &balance1, ComputedAt: day1 + 86410, Watermark: 301, SourceVersion: WalletConsumeAggregateSourceVersion},
		{UserId: 17, DayStart: day3, RequestCount: 9, Quota: 303, LastLogAt: day3 + 10, LastLogId: 303, BalanceAfter: &balance3, ComputedAt: day3 + 86410, Watermark: 303, SourceVersion: WalletConsumeAggregateSourceVersion},
	}).Error)
	computedAt := time.Now().Unix()
	require.NoError(t, db.Create([]WalletConsumeDailyAggregateCoverage{
		{DayStart: day1, DayEnd: day2, Timezone: walletConsumeTimezoneName(), ComputedAt: computedAt, Watermark: 301, SourceVersion: WalletConsumeAggregateSourceVersion, IsComplete: true},
		{DayStart: day3, DayEnd: day4, Timezone: walletConsumeTimezoneName(), ComputedAt: computedAt, Watermark: 303, SourceVersion: WalletConsumeAggregateSourceVersion, IsComplete: true},
	}).Error)

	items, err := GetUserFinancialConsumeDailyWithContext(context.Background(), 17, day1, day4)
	require.NoError(t, err)
	require.Equal(t, []FinancialConsumeDaily{
		{DayStart: day3, Quota: 303, BalanceAfter: &balance3},
		{DayStart: day2, Quota: 2, BalanceAfter: &rawBalance2},
		{DayStart: day1, Quota: 101, BalanceAfter: &balance1},
	}, items)
}

func TestMergeWalletConsumeLegacyPartsCoalescesAdjacentRanges(t *testing.T) {
	day := time.Date(2025, 3, 11, 0, 0, 0, 0, time.Local).Unix()
	day2 := walletConsumeDayEnd(day)
	day3 := walletConsumeDayEnd(day2)
	day4 := walletConsumeDayEnd(day3)
	got := mergeWalletConsumeLegacyParts([]walletConsumeDayPart{
		{Start: day + 10, End: day + 20},
		{Start: day + 20, End: day2},
		{Start: day3, End: day4},
	})
	require.Equal(t, []walletConsumeDayPart{
		{Start: day + 10, End: day2},
		{Start: day3, End: day4},
	}, got)
}

func TestWalletConsumeDailyAggregateReadFallsBackAllRangeOnAggregateQueryError(t *testing.T) {
	db, restore := openWalletConsumeAggregateTestDB(t)
	defer restore()
	t.Setenv(WalletConsumeAggregateReadEnableEnv, "true")

	day := time.Date(2025, 3, 12, 0, 0, 0, 0, time.Local).Unix()
	dayEnd := walletConsumeDayEnd(day)
	balance := int64(44)
	require.NoError(t, db.Create(&Log{
		Id: 401, UserId: 18, Type: LogTypeConsume, CreatedAt: day + 10, Quota: 4, BalanceAfter: &balance,
	}).Error)
	require.NoError(t, db.Create(&WalletConsumeDailyAggregateCoverage{
		DayStart: day, DayEnd: dayEnd, Timezone: walletConsumeTimezoneName(), ComputedAt: time.Now().Unix(),
		Watermark: 401, SourceVersion: WalletConsumeAggregateSourceVersion, IsComplete: true,
	}).Error)
	// Make the projection query fail after coverage succeeds. The public reader
	// must then use the complete legacy range, rather than returning a partial
	// result assembled from whichever rows were already fetched.
	require.NoError(t, db.Migrator().DropTable(&WalletConsumeDailyAggregate{}))

	items, err := GetUserFinancialConsumeDailyWithContext(context.Background(), 18, day, dayEnd)
	require.NoError(t, err)
	require.Equal(t, []FinancialConsumeDaily{{DayStart: day, Quota: 4, BalanceAfter: &balance}}, items)
}

func TestWalletConsumeDailyAggregateReadNormalizesNilContext(t *testing.T) {
	db, restore := openWalletConsumeAggregateTestDB(t)
	defer restore()
	t.Setenv(WalletConsumeAggregateReadEnableEnv, "true")

	day := time.Date(2025, 3, 5, 0, 0, 0, 0, time.Local).Unix()
	balance := int64(12)
	require.NoError(t, db.Create(&Log{
		UserId: 101, Type: LogTypeConsume, CreatedAt: day + 10, Quota: 8, BalanceAfter: &balance,
	}).Error)
	_, err := RebuildWalletConsumeDailyAggregates(context.Background(), day, walletConsumeDayEnd(day))
	require.NoError(t, err)

	items, err := GetUserFinancialConsumeDailyWithContext(nil, 101, day, walletConsumeDayEnd(day))
	require.NoError(t, err)
	require.Equal(t, []FinancialConsumeDaily{{DayStart: day, Quota: 8, BalanceAfter: &balance}}, items)
}

func TestWalletConsumeDailyAggregateFailsClosedOnRawArchiveOverlap(t *testing.T) {
	db, restore := openWalletConsumeAggregateTestDB(t)
	defer restore()
	t.Setenv(WalletConsumeAggregateReadEnableEnv, "true")

	day := time.Date(2025, 4, 5, 0, 0, 0, 0, time.Local).Unix()
	// A raw row and an archive segment whose last source timestamp crosses the
	// raw boundary cannot be proven disjoint (archive rows do not retain source
	// log IDs). The rebuild must refuse to publish a potentially double-counted
	// snapshot.
	require.NoError(t, db.Create(&Log{
		Id: 31, UserId: 11, Type: LogTypeConsume, CreatedAt: day + 20, Quota: 5,
	}).Error)
	require.NoError(t, db.Create(&UsageLogDailyAggregate{
		BucketStart: day, UserId: 11, Type: LogTypeConsume,
		Quota: 7, RequestCount: 1, FirstLogAt: day + 10, LastLogAt: day + 30,
		CreatedAt: day + 31,
	}).Error)

	_, err := RebuildWalletConsumeDailyAggregates(context.Background(), day, walletConsumeDayEnd(day))
	require.ErrorIs(t, err, ErrWalletConsumeAggregateSourceOverlap)
	var coverageCount int64
	require.NoError(t, db.Model(&WalletConsumeDailyAggregateCoverage{}).Where("day_start = ?", day).Count(&coverageCount).Error)
	require.Zero(t, coverageCount)
	items, err := GetUserFinancialConsumeDailyWithContext(context.Background(), 11, day, walletConsumeDayEnd(day))
	require.NoError(t, err)
	// Read switch falls back to the old exact path when coverage is absent; the
	// test only asserts that the new projection did not publish a duplicate.
	require.Len(t, items, 1)
}

func TestWalletConsumeDailyAggregateReportsCoverageInvalidationError(t *testing.T) {
	db, restore := openWalletConsumeAggregateTestDB(t)
	defer restore()

	day := time.Date(2025, 4, 6, 0, 0, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create(&Log{
		Id: 41, UserId: 111, Type: LogTypeConsume, CreatedAt: day + 20, Quota: 5,
	}).Error)
	require.NoError(t, db.Create(&UsageLogDailyAggregate{
		BucketStart: day, UserId: 111, Type: LogTypeConsume,
		Quota: 7, RequestCount: 1, FirstLogAt: day + 10, LastLogAt: day + 30,
		CreatedAt: day + 31,
	}).Error)
	// Force the overlap invalidation UPDATE to fail. The rebuild must still
	// retain the overlap sentinel and expose the failed coverage update instead
	// of silently leaving a potentially readable stale marker.
	require.NoError(t, db.Migrator().DropTable(&WalletConsumeDailyAggregateCoverage{}))

	_, err := RebuildWalletConsumeDailyAggregates(context.Background(), day, walletConsumeDayEnd(day))
	require.ErrorIs(t, err, ErrWalletConsumeAggregateSourceOverlap)
	require.Contains(t, err.Error(), "invalidate coverage")
}

func TestWalletConsumeDailyAggregateLegacyIncludesFullDayBeforeExactEndBoundary(t *testing.T) {
	db, restore := openWalletConsumeAggregateTestDB(t)
	defer restore()
	// Keep the projection reader disabled so this exercises the compatibility
	// path used when coverage is unavailable.
	t.Setenv(WalletConsumeAggregateReadEnableEnv, "false")

	day1 := time.Date(2025, 5, 1, 0, 0, 0, 0, time.Local).Unix()
	day2 := walletConsumeDayEnd(day1)
	day3 := walletConsumeDayEnd(day2)
	require.NoError(t, db.Create(&UsageLogDailyAggregate{
		BucketStart:  day2,
		UserId:       12,
		Type:         LogTypeConsume,
		Quota:        123,
		RequestCount: 4,
		// Deliberately straddle the bucket boundary. A full-day request is
		// still allowed to use the bucket-level aggregate; the old exclusive
		// end calculation incorrectly omitted this row.
		FirstLogAt: day2 - 10,
		LastLogAt:  day2 + 20,
		CreatedAt:  day3 + 1,
	}).Error)

	items, err := GetUserFinancialConsumeDailyWithContext(context.Background(), 12, day1+3600, day3)
	require.NoError(t, err)
	require.Equal(t, []FinancialConsumeDaily{{DayStart: day2, Quota: 123}}, items)
}

func TestWalletConsumeDailyAggregateLegacyExcludesArchiveCrossingPartialBoundary(t *testing.T) {
	db, restore := openWalletConsumeAggregateTestDB(t)
	defer restore()
	t.Setenv(WalletConsumeAggregateReadEnableEnv, "false")

	day := time.Date(2025, 5, 10, 0, 0, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create(&UsageLogDailyAggregate{
		BucketStart:  day,
		UserId:       13,
		Type:         LogTypeConsume,
		Quota:        999,
		RequestCount: 2,
		FirstLogAt:   day + 10,
		LastLogAt:    day + 7200,
		CreatedAt:    day + 7201,
	}).Error)

	// The archive segment crosses the requested start. Since individual source
	// rows are gone, counting the whole segment would overstate this partial
	// window; the compatibility path must return no item.
	items, err := GetUserFinancialConsumeDailyWithContext(context.Background(), 13, day+3600, day+10800)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestWalletConsumeDailyAggregateLegacyDoesNotTreatPartialEndDayAsFull(t *testing.T) {
	db, restore := openWalletConsumeAggregateTestDB(t)
	defer restore()
	t.Setenv(WalletConsumeAggregateReadEnableEnv, "false")

	day := time.Date(2025, 5, 10, 0, 0, 0, 0, time.Local).Unix()
	nextDay := walletConsumeDayEnd(day)
	require.NoError(t, db.Create(&UsageLogDailyAggregate{
		BucketStart:  nextDay,
		UserId:       131,
		Type:         LogTypeConsume,
		Quota:        1000,
		RequestCount: 3,
		FirstLogAt:   nextDay + 10,
		LastLogAt:    nextDay + 7200,
		CreatedAt:    nextDay + 7201,
	}).Error)

	// The requested end is inside nextDay. The segment crosses that end and
	// cannot be sliced, so it must not be selected by a full-day predicate.
	items, err := GetUserFinancialConsumeDailyWithContext(context.Background(), 131, day, nextDay+3600)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestWalletConsumeDailyAggregateLegacyKeepsDisjointArchiveAfterRaw(t *testing.T) {
	db, restore := openWalletConsumeAggregateTestDB(t)
	defer restore()
	t.Setenv(WalletConsumeAggregateReadEnableEnv, "false")

	day := time.Date(2025, 5, 11, 0, 0, 0, 0, time.Local).Unix()
	balanceRaw, balanceArchive := int64(90), int64(70)
	require.NoError(t, db.Create(&Log{
		Id:           131,
		UserId:       14,
		Type:         LogTypeConsume,
		CreatedAt:    day + 10,
		Quota:        10,
		BalanceAfter: &balanceRaw,
	}).Error)
	require.NoError(t, db.Create(&UsageLogDailyAggregate{
		BucketStart:  day,
		UserId:       14,
		Type:         LogTypeConsume,
		Quota:        20,
		RequestCount: 1,
		FirstLogAt:   day + 20,
		LastLogAt:    day + 30,
		BalanceAfter: &balanceArchive,
		CreatedAt:    day + 31,
	}).Error)

	items, err := GetUserFinancialConsumeDailyWithContext(context.Background(), 14, day, walletConsumeDayEnd(day))
	require.NoError(t, err)
	require.Equal(t, []FinancialConsumeDaily{{DayStart: day, Quota: 30, BalanceAfter: &balanceArchive}}, items)
}

func TestWalletConsumeDailyAggregateLegacyUsesIdTieBreakForBalance(t *testing.T) {
	db, restore := openWalletConsumeAggregateTestDB(t)
	defer restore()
	t.Setenv(WalletConsumeAggregateReadEnableEnv, "false")

	day := time.Date(2025, 5, 12, 0, 0, 0, 0, time.Local).Unix()
	balanceFirst, balanceSecond := int64(80), int64(60)
	require.NoError(t, db.Create([]Log{
		{Id: 141, UserId: 15, Type: LogTypeConsume, CreatedAt: day + 100, Quota: 10, BalanceAfter: &balanceFirst},
		{Id: 142, UserId: 15, Type: LogTypeConsume, CreatedAt: day + 100, Quota: 20, BalanceAfter: &balanceSecond},
	}).Error)

	items, err := GetUserFinancialConsumeDailyWithContext(context.Background(), 15, day, walletConsumeDayEnd(day))
	require.NoError(t, err)
	require.Equal(t, []FinancialConsumeDaily{{DayStart: day, Quota: 30, BalanceAfter: &balanceSecond}}, items)
}

func TestWalletConsumeDailyAggregateLegacyUsesSourceIdAcrossArchiveRows(t *testing.T) {
	db, restore := openWalletConsumeAggregateTestDB(t)
	defer restore()
	t.Setenv(WalletConsumeAggregateReadEnableEnv, "false")

	day := time.Date(2025, 5, 13, 0, 0, 0, 0, time.Local).Unix()
	firstBalance, lastBalance := int64(80), int64(60)
	require.NoError(t, db.Create([]UsageLogDailyAggregate{
		{
			Id: 201, BucketStart: day, UserId: 16, Type: LogTypeConsume,
			Quota: 10, RequestCount: 1, FirstLogAt: day + 100, LastLogAt: day + 100,
			LastLogId: 141, BalanceAfter: &firstBalance, CreatedAt: day + 1000,
		},
		{
			Id: 202, BucketStart: day, UserId: 16, Type: LogTypeConsume,
			Quota: 20, RequestCount: 1, FirstLogAt: day + 100, LastLogAt: day + 100,
			LastLogId: 142, BalanceAfter: &lastBalance, CreatedAt: day + 1001,
		},
	}).Error)

	items, err := GetUserFinancialConsumeDailyWithContext(context.Background(), 16, day, walletConsumeDayEnd(day))
	require.NoError(t, err)
	require.Equal(t, []FinancialConsumeDaily{{DayStart: day, Quota: 30, BalanceAfter: &lastBalance}}, items)
}

func TestSaveWalletConsumeAggregateCheckpointIsIdempotent(t *testing.T) {
	db, restore := openWalletConsumeAggregateTestDB(t)
	defer restore()
	now := time.Now().Unix()
	first := WalletConsumeAggregateCheckpoint{
		Name: walletConsumeAggregateCheckpointName, NextDayStart: 100, UpdatedAt: now,
	}
	second := first
	second.NextDayStart = 200
	second.UpdatedAt = now + 1
	require.NoError(t, SaveWalletConsumeAggregateCheckpoint(context.Background(), db, first))
	require.NoError(t, SaveWalletConsumeAggregateCheckpoint(context.Background(), db, second))
	loaded, err := LoadWalletConsumeAggregateCheckpoint(context.Background(), db)
	require.NoError(t, err)
	require.EqualValues(t, 200, loaded.NextDayStart)
	var count int64
	require.NoError(t, db.Model(&WalletConsumeAggregateCheckpoint{}).Where("name = ?", first.Name).Count(&count).Error)
	require.EqualValues(t, 1, count)
}
