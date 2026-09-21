package model

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestVirtualMembershipResetCalendarUsesOperatorCountsAndShanghaiBounds(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:virtual-membership-reset-calendar-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&VirtualMembershipResetCalendarEntry{}))
	previousDB := DB
	DB = db
	t.Cleanup(func() { DB = previousDB })

	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	start := time.Date(2026, time.September, 1, 0, 0, 0, 0, location)
	nextMonth := start.AddDate(0, 1, 0)
	entries := []*VirtualMembershipResetCalendarEntry{
		{ResetAt: start.Add(30 * time.Minute).Unix(), Count: 3, Reason: "周期开启"},
		{ResetAt: start.AddDate(0, 0, 10).Add(12 * time.Hour).Unix(), Count: 2, Reason: "官方调整"},
		{ResetAt: nextMonth.Unix(), Count: 99, Reason: "下月记录"},
	}
	for _, entry := range entries {
		require.NoError(t, SaveVirtualMembershipResetCalendarEntry(entry))
	}

	monthEntries, total, err := ListVirtualMembershipResetCalendarEntries(start.Unix(), nextMonth.Unix())
	require.NoError(t, err)
	require.Len(t, monthEntries, 2)
	require.Equal(t, 5, total)
	require.Equal(t, "周期开启", monthEntries[0].Reason)

	require.NoError(t, SaveVirtualMembershipResetCalendarEntry(&VirtualMembershipResetCalendarEntry{
		Id: monthEntries[0].Id, ResetAt: monthEntries[0].ResetAt, Count: 7, Reason: "修正后的运营记录",
		CreatedAt: monthEntries[0].CreatedAt,
	}))
	updated, err := GetVirtualMembershipResetCalendarEntry(monthEntries[0].Id)
	require.NoError(t, err)
	require.Equal(t, 7, updated.Count)
	require.Equal(t, "修正后的运营记录", updated.Reason)

	require.NoError(t, DeleteVirtualMembershipResetCalendarEntry(updated.Id))
	monthEntries, total, err = ListVirtualMembershipResetCalendarEntries(start.Unix(), nextMonth.Unix())
	require.NoError(t, err)
	require.Len(t, monthEntries, 1)
	require.Equal(t, 2, total)
}
