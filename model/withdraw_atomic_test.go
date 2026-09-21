package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupWithdrawAtomicTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:withdraw-atomic-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &Withdraw{}))
	oldDB := DB
	DB = db
	t.Cleanup(func() {
		DB = oldDB
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestFreezeUserBalanceRequiresAvailableFundsAtomically(t *testing.T) {
	db := setupWithdrawAtomicTestDB(t)
	user := User{
		Username: "withdraw-freeze-guard",
		Status:   common.UserStatusEnabled,
		Quota:    100,
	}
	require.NoError(t, db.Create(&user).Error)

	require.NoError(t, FreezeUserBalance(user.Id, WithdrawTypePrincipal, 100))
	require.ErrorContains(t, FreezeUserBalance(user.Id, WithdrawTypePrincipal, 1), "可用余额不足")

	var refreshed User
	require.NoError(t, db.First(&refreshed, user.Id).Error)
	require.Zero(t, refreshed.Quota)
	require.Equal(t, 100, refreshed.FrozenQuota)
}

func TestFinishWithdrawOnlyReportsPendingTransition(t *testing.T) {
	db := setupWithdrawAtomicTestDB(t)
	withdraw := Withdraw{UserId: 10, Type: WithdrawTypePrincipal, Amount: 100, Status: WithdrawStatusPending}
	require.NoError(t, db.Create(&withdraw).Error)

	updated, err := FinishWithdraw(withdraw.Id, WithdrawStatusApproved, 99, "root", "paid")
	require.NoError(t, err)
	require.True(t, updated)

	updated, err = FinishWithdraw(withdraw.Id, WithdrawStatusRejected, 100, "other-root", "duplicate")
	require.NoError(t, err)
	require.False(t, updated)

	var refreshed Withdraw
	require.NoError(t, db.First(&refreshed, withdraw.Id).Error)
	require.Equal(t, WithdrawStatusApproved, refreshed.Status)
	require.Equal(t, 99, refreshed.HandlerId)
}

func TestWithdrawReleaseRequiresFrozenBalance(t *testing.T) {
	db := setupWithdrawAtomicTestDB(t)
	user := User{
		Username:       "withdraw-release-guard",
		Status:         common.UserStatusEnabled,
		Quota:          50,
		FrozenQuota:    50,
		FrozenDividend: 20,
	}
	require.NoError(t, db.Create(&user).Error)

	require.Error(t, ApproveUserWithdraw(user.Id, WithdrawTypePrincipal, 51))
	require.NoError(t, RejectUserWithdraw(user.Id, WithdrawTypePrincipal, 50))

	var refreshed User
	require.NoError(t, db.First(&refreshed, user.Id).Error)
	require.Equal(t, 100, refreshed.Quota)
	require.Zero(t, refreshed.FrozenQuota)
	require.Equal(t, 20, refreshed.FrozenDividend)
}
