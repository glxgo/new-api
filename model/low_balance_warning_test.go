package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestLowBalanceWarningFiresOnceAndRechargeRearms(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &RechargeCredit{}))
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	threshold := int(common.QuotaPerUnit)
	user := User{
		Username: "low-balance-user", Password: "hashed", AffCode: "lbw1",
		Quota: threshold - 1, LowBalanceWarningArmed: true,
	}
	require.NoError(t, db.Create(&user).Error)

	remaining, claimed, err := ClaimLowBalanceWarning(user.Id, threshold)
	require.NoError(t, err)
	require.True(t, claimed)
	require.Equal(t, threshold-1, remaining)

	_, claimed, err = ClaimLowBalanceWarning(user.Id, threshold)
	require.NoError(t, err)
	require.False(t, claimed, "同一充值周期内不得重复领取提醒资格")

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		created, createErr := RecordRechargeCreditTx(tx, user.Id, 2_000, "topup", "rearm-order", 100)
		require.True(t, created)
		return createErr
	}))
	_, claimed, err = ClaimLowBalanceWarning(user.Id, threshold)
	require.NoError(t, err)
	require.True(t, claimed, "充值后应重新允许一次低余额提醒")
}

func TestLowBalanceWarningDoesNotClaimAboveThreshold(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}))
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	threshold := int(common.QuotaPerUnit)
	user := User{
		Username: "healthy-balance-user", Password: "hashed", AffCode: "lbw2",
		Quota: threshold, LowBalanceWarningArmed: true,
	}
	require.NoError(t, db.Create(&user).Error)
	_, claimed, err := ClaimLowBalanceWarning(user.Id, threshold)
	require.NoError(t, err)
	require.False(t, claimed)
}
