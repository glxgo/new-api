package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func withUserCapacityDefaults(t *testing.T, concurrency, rpm int) {
	t.Helper()
	oldConcurrency := common.DefaultUserConcurrencyLimit
	oldRPM := common.DefaultUserRPMLimit
	oldRechargeCapacity := common.RechargeCapacityEnabled
	common.DefaultUserConcurrencyLimit = concurrency
	common.DefaultUserRPMLimit = rpm
	common.RechargeCapacityEnabled = false
	t.Cleanup(func() {
		common.DefaultUserConcurrencyLimit = oldConcurrency
		common.DefaultUserRPMLimit = oldRPM
		common.RechargeCapacityEnabled = oldRechargeCapacity
	})
}

func TestUserCapacityLimitsPreserveHigherEntitlements(t *testing.T) {
	withUserCapacityDefaults(t, 8, 12)
	for _, override := range []bool{false, true} {
		for _, test := range []struct {
			total                                    int64
			concurrent, rpm, wantConcurrent, wantRPM int
		}{
			{0, 8, 12, 200, 1000}, {99999, 300, 2000, 300, 2000}, {0, 300, 12, 300, 1000},
			{0, 8, 2000, 200, 2000}, {100000, 300, 2000, 0, 0}, {100001, 0, 0, 0, 0},
		} {
			user := User{RechargeTotalCents: test.total, ConcurrencyLimit: test.concurrent, RPMLimit: test.rpm, ConcurrencyLimitOverride: override, RPMLimitOverride: override}
			require.Equal(t, test.wantConcurrent, user.EffectiveConcurrencyLimit())
			require.Equal(t, test.wantRPM, user.EffectiveRPMLimit())
			cached := user.ToBaseUser()
			require.Equal(t, test.wantConcurrent, cached.EffectiveConcurrencyLimit())
			require.Equal(t, test.wantRPM, cached.EffectiveRPMLimit())
		}
	}
	common.DefaultUserConcurrencyLimit = 500
	common.DefaultUserRPMLimit = 3000
	user := User{}
	require.Equal(t, 500, user.EffectiveConcurrencyLimit())
	require.Equal(t, 3000, user.EffectiveRPMLimit())
}

func TestMembershipCannotReduceAccountCapacity(t *testing.T) {
	require.Equal(t, 200, MergeAccountAndMembershipCapacity(200, 10))
	require.Equal(t, 300, MergeAccountAndMembershipCapacity(200, 300))
	require.Zero(t, MergeAccountAndMembershipCapacity(0, 300))
	require.Zero(t, MergeAccountAndMembershipCapacity(200, 0))
}

func TestUserEditKeepsCapacityOverridesIndependent(t *testing.T) {
	withUserCapacityDefaults(t, 8, 12)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}))
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	user := User{Username: "capacity-policy", Password: "hashed-password", ConcurrencyLimit: 8, RPMLimit: 12}
	require.NoError(t, db.Create(&user).Error)
	customConcurrency := 20
	trueValue := true
	require.NoError(t, (&User{
		Id: user.Id, Username: user.Username, DisplayName: user.DisplayName, Group: user.Group,
	}).Edit(false, UserCapacityLimitUpdate{
		ConcurrencyLimit:         &customConcurrency,
		ConcurrencyLimitOverride: &trueValue,
	}))

	var stored User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.True(t, stored.ConcurrencyLimitOverride)
	require.False(t, stored.RPMLimitOverride)
	common.DefaultUserConcurrencyLimit = 16
	common.DefaultUserRPMLimit = 30
	require.Equal(t, 200, stored.EffectiveConcurrencyLimit())
	require.Equal(t, 1000, stored.EffectiveRPMLimit())

	falseValue := false
	require.NoError(t, (&User{
		Id: user.Id, Username: user.Username, DisplayName: user.DisplayName, Group: user.Group,
	}).Edit(false, UserCapacityLimitUpdate{ConcurrencyLimitOverride: &falseValue}))
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.False(t, stored.ConcurrencyLimitOverride)
	require.Equal(t, 200, stored.EffectiveConcurrencyLimit())
}

func TestMigrateUserCapacityOverridesPreservesLegacyCustomConcurrency(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &Option{}))
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	users := []User{
		{Username: "legacy-default", Password: "hashed-password", AffCode: "cap1", ConcurrencyLimit: 8},
		{Username: "legacy-custom", Password: "hashed-password", AffCode: "cap2", ConcurrencyLimit: 24},
	}
	require.NoError(t, db.Create(&users).Error)
	require.NoError(t, migrateUserCapacityOverridesV1())
	require.NoError(t, migrateUserCapacityOverridesV1(), "migration must be idempotent")

	var defaultUser, customUser User
	require.NoError(t, db.Where("username = ?", "legacy-default").First(&defaultUser).Error)
	require.NoError(t, db.Where("username = ?", "legacy-custom").First(&customUser).Error)
	require.False(t, defaultUser.ConcurrencyLimitOverride)
	require.True(t, customUser.ConcurrencyLimitOverride)
	require.False(t, defaultUser.RPMLimitOverride)
	require.False(t, customUser.RPMLimitOverride)
}
