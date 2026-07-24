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

func TestUserCapacityLimitsInheritIndependently(t *testing.T) {
	withUserCapacityDefaults(t, 8, 12)
	user := User{ConcurrencyLimit: 99, RPMLimit: 999}
	require.Equal(t, 8, user.EffectiveConcurrencyLimit())
	require.Equal(t, 12, user.EffectiveRPMLimit())

	user.ConcurrencyLimitOverride = true
	require.Equal(t, 99, user.EffectiveConcurrencyLimit())
	require.Equal(t, 12, user.EffectiveRPMLimit())

	common.DefaultUserConcurrencyLimit = 16
	common.DefaultUserRPMLimit = 30
	require.Equal(t, 99, user.EffectiveConcurrencyLimit())
	require.Equal(t, 30, user.EffectiveRPMLimit())

	user.RPMLimitOverride = true
	require.Equal(t, 999, user.EffectiveRPMLimit())
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
	require.Equal(t, 20, stored.EffectiveConcurrencyLimit())
	require.Equal(t, 30, stored.EffectiveRPMLimit())

	falseValue := false
	require.NoError(t, (&User{
		Id: user.Id, Username: user.Username, DisplayName: user.DisplayName, Group: user.Group,
	}).Edit(false, UserCapacityLimitUpdate{ConcurrencyLimitOverride: &falseValue}))
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.False(t, stored.ConcurrencyLimitOverride)
	require.Equal(t, 16, stored.EffectiveConcurrencyLimit())
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
