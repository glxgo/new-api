package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setCyberPolicyModesForTest(t *testing.T, interceptionEnabled bool, enforcementEnabled bool) {
	t.Helper()
	oldInterception := common.CyberPolicyInterceptionEnabled
	oldEnforcement := common.CyberPolicyEnforcementEnabled
	common.CyberPolicyInterceptionEnabled = interceptionEnabled
	common.CyberPolicyEnforcementEnabled = enforcementEnabled
	t.Cleanup(func() {
		common.CyberPolicyInterceptionEnabled = oldInterception
		common.CyberPolicyEnforcementEnabled = oldEnforcement
	})
}

func TestCyberPolicyEscalationAndBurstCollapse(t *testing.T) {
	setCyberPolicyModesForTest(t, false, true)
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
	setCyberPolicyModesForTest(t, false, true)
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
	setCyberPolicyModesForTest(t, false, true)
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
	setCyberPolicyModesForTest(t, false, false)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &UserSecurityIncident{}))
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

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

func TestCyberPolicyInterceptionTakesPriorityWithoutPunishment(t *testing.T) {
	setCyberPolicyModesForTest(t, true, true)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &UserSecurityIncident{}))
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	user := User{Username: "security-intercepted", Password: "hashed-password", AffCode: "sec6"}
	require.NoError(t, db.Create(&user).Error)
	now := int64(1_700_000_000)

	first, err := RecordCyberPolicyInterception(user.Id, 6, "intercept-1", "gpt", "cyber_policy", now)
	require.NoError(t, err)
	require.True(t, first.Recorded)
	require.True(t, first.ShouldNotify)

	duplicate, err := RecordCyberPolicyInterception(user.Id, 6, "intercept-1", "gpt", "cyber_policy", now+1)
	require.NoError(t, err)
	require.False(t, duplicate.Recorded)
	require.False(t, duplicate.ShouldNotify)

	second, err := RecordCyberPolicyInterception(user.Id, 6, "intercept-2", "gpt", "cyber_policy", now+60)
	require.NoError(t, err)
	require.True(t, second.Recorded)
	require.False(t, second.ShouldNotify)

	afterCooldown, err := RecordCyberPolicyInterception(user.Id, 6, "intercept-3", "gpt", "cyber_policy", now+60+int64((24*time.Hour).Seconds())+1)
	require.NoError(t, err)
	require.True(t, afterCooldown.Recorded)
	require.True(t, afterCooldown.ShouldNotify)

	enforcement, err := ApplyCyberPolicyViolation(user.Id, 6, "must-not-enforce", "gpt", "cyber_policy", now)
	require.NoError(t, err)
	require.False(t, enforcement.Counted)
	require.Zero(t, enforcement.StrikeNumber)

	var stored User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.Zero(t, stored.SecurityStrikeCount)
	require.Zero(t, stored.SecuritySuspendedUntil)
	require.False(t, stored.SecurityPermanentBan)

	var interceptions []UserSecurityIncident
	require.NoError(t, db.Order("id").Find(&interceptions).Error)
	require.Len(t, interceptions, 3)
	for _, incident := range interceptions {
		require.Equal(t, UserSecurityActionIntercepted, incident.Action)
		require.False(t, incident.Counted)
		require.Zero(t, incident.StrikeNumber)
		require.Zero(t, incident.SuspendedUntil)
	}
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
