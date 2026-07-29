package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCyberPolicyEscalationAndBurstCollapse(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &UserSecurityIncident{}))
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	user := User{Username: "security-user", Password: "hashed-password", AffCode: "sec1"}
	require.NoError(t, db.Create(&user).Error)
	now := int64(1_700_000_000)

	first, err := ApplyCyberPolicyViolation(user.Id, 1, "request-1", "gpt", "cyber_policy", now)
	require.NoError(t, err)
	require.True(t, first.Counted)
	require.Equal(t, 1, first.StrikeNumber)
	require.EqualValues(t, now+600, first.SuspendedUntil)

	burst, err := ApplyCyberPolicyViolation(user.Id, 1, "request-2", "gpt", "cyber_policy", now+1)
	require.NoError(t, err)
	require.False(t, burst.Counted)
	require.Equal(t, 1, burst.StrikeNumber)
	require.Equal(t, UserSecurityActionObserved, burst.Action)

	second, err := ApplyCyberPolicyViolation(user.Id, 1, "request-3", "gpt", "cyber_policy", first.SuspendedUntil+1)
	require.NoError(t, err)
	require.True(t, second.Counted)
	require.Equal(t, 2, second.StrikeNumber)
	require.EqualValues(t, first.SuspendedUntil+1+7200, second.SuspendedUntil)

	third, err := ApplyCyberPolicyViolation(user.Id, 1, "request-4", "gpt", "cyber_policy", second.SuspendedUntil+1)
	require.NoError(t, err)
	require.Equal(t, 3, third.StrikeNumber)
	require.EqualValues(t, second.SuspendedUntil+1+86400, third.SuspendedUntil)

	fourth, err := ApplyCyberPolicyViolation(user.Id, 1, "request-5", "gpt", "cyber_policy", third.SuspendedUntil+1)
	require.NoError(t, err)
	require.True(t, fourth.Permanent)
	require.Equal(t, 4, fourth.StrikeNumber)

	var stored User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.True(t, stored.SecurityPermanentBan)
	require.Equal(t, 4, stored.SecurityStrikeCount)

	var incidents int64
	require.NoError(t, db.Model(&UserSecurityIncident{}).Count(&incidents).Error)
	require.EqualValues(t, 5, incidents)
}

func TestCyberPolicyRequestIdIsIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &UserSecurityIncident{}))
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	user := User{Username: "security-idempotent", Password: "hashed-password", AffCode: "sec2"}
	require.NoError(t, db.Create(&user).Error)
	first, err := ApplyCyberPolicyViolation(user.Id, 2, "same-request", "gpt", "cyber_policy", 1_700_000_000)
	require.NoError(t, err)
	require.True(t, first.Counted)
	duplicate, err := ApplyCyberPolicyViolation(user.Id, 2, "same-request", "gpt", "cyber_policy", 1_700_000_001)
	require.NoError(t, err)
	require.False(t, duplicate.Counted)

	var stored User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.Equal(t, 1, stored.SecurityStrikeCount)
}

func TestCyberPolicySkipsWhitelistedUser(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &UserSecurityIncident{}))
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	user := User{
		Username:            "security-whitelisted",
		Password:            "hashed-password",
		AffCode:             "sec3",
		SecurityWhitelisted: true,
	}
	require.NoError(t, db.Create(&user).Error)

	result, err := ApplyCyberPolicyViolation(user.Id, 3, "whitelisted-request", "gpt", "cyber_policy", 1_700_000_000)
	require.NoError(t, err)
	require.False(t, result.Counted)
	require.Zero(t, result.StrikeNumber)

	var incidents int64
	require.NoError(t, db.Model(&UserSecurityIncident{}).Count(&incidents).Error)
	require.Zero(t, incidents)
}

func TestCyberPolicySkipsWhenEnforcementDisabled(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &UserSecurityIncident{}))
	oldDB := DB
	DB = db
	oldEnabled := common.CyberPolicyEnforcementEnabled
	common.CyberPolicyEnforcementEnabled = false
	t.Cleanup(func() {
		DB = oldDB
		common.CyberPolicyEnforcementEnabled = oldEnabled
	})

	user := User{Username: "security-disabled", Password: "hashed-password", AffCode: "sec4"}
	require.NoError(t, db.Create(&user).Error)

	result, err := ApplyCyberPolicyViolation(user.Id, 4, "disabled-request", "gpt", "cyber_policy", 1_700_000_000)
	require.NoError(t, err)
	require.False(t, result.Counted)
	require.Zero(t, result.StrikeNumber)

	var incidents int64
	require.NoError(t, db.Model(&UserSecurityIncident{}).Count(&incidents).Error)
	require.Zero(t, incidents)
}

func TestSetUserSecurityWhitelistClearsRestriction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}))
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	user := User{
		Username:               "security-whitelist-reset",
		Password:               "hashed-password",
		AffCode:                "sec5",
		SecurityStrikeCount:    3,
		SecuritySuspendedUntil: 1_800_000_000,
		SecurityPermanentBan:   true,
	}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, SetUserSecurityWhitelist(user.Id, true))

	var stored User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.True(t, stored.SecurityWhitelisted)
	require.Zero(t, stored.SecurityStrikeCount)
	require.Zero(t, stored.SecuritySuspendedUntil)
	require.False(t, stored.SecurityPermanentBan)
}
