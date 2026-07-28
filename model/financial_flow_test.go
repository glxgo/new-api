package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetUserFinancialConsumeDaily(t *testing.T) {
	previousLogDB := LOG_DB
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:financial-flow-%s?mode=memory&cache=shared", t.Name())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	LOG_DB = db
	t.Cleanup(func() { LOG_DB = previousLogDB })

	day1 := time.Date(2026, 7, 28, 0, 0, 0, 0, time.Local).Unix()
	balance80, balance50, balance0, balance1 := int64(80), int64(50), int64(0), int64(1)
	require.NoError(t, db.Create([]Log{
		{UserId: 1, Type: LogTypeConsume, CreatedAt: day1 + 10, Quota: 20, BalanceAfter: &balance80},
		{UserId: 1, Type: LogTypeConsume, CreatedAt: day1 + 20, Quota: 30, BalanceAfter: &balance50},
		{UserId: 1, Type: LogTypeConsume, CreatedAt: day1 + 86400 + 10, Quota: 50, BalanceAfter: &balance0},
		{UserId: 2, Type: LogTypeConsume, CreatedAt: day1 + 30, Quota: 999, BalanceAfter: &balance1},
		{UserId: 1, Type: LogTypeTopup, CreatedAt: day1 + 40, Quota: 999, BalanceAfter: &balance1},
	}).Error)

	items, err := GetUserFinancialConsumeDaily(1, day1, day1+2*86400)
	require.NoError(t, err)
	require.Equal(t, []FinancialConsumeDaily{
		{DayStart: day1 + 86400, Quota: 50, BalanceAfter: &balance0},
		{DayStart: day1, Quota: 50, BalanceAfter: &balance50},
	}, items)
}
