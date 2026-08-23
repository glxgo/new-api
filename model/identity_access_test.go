package model

import (
	"net"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupIdentityAccessTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &UserIdentity{}, &MainlandIPAllowlist{}))
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })
	return db
}

func TestIdentityAndMainlandAllowlistLifecycle(t *testing.T) {
	db := setupIdentityAccessTestDB(t)
	user := User{Username: "identity-user", Password: "hashed", Role: 1, Status: 1}
	require.NoError(t, db.Create(&user).Error)

	previous, next, err := SetUserIdentity(user.Id, 99, IdentityTypeEnterprise)
	require.NoError(t, err)
	require.Equal(t, IdentityTypeNone, previous)
	require.Equal(t, IdentityTypeEnterprise, next)

	ip := net.ParseIP("203.0.113.10")
	row, err := AddMainlandIPWhitelist(user.Id, user.Id, ip, MainlandIPAllowlistSourceSelf)
	require.NoError(t, err)
	require.Equal(t, "ipv4", row.AddressFamily)
	require.Equal(t, 32, row.PrefixLength)
	require.True(t, IsMainlandIPWhitelisted(ip))

	// Repeated application is idempotent.
	rowAgain, err := AddMainlandIPWhitelist(user.Id, user.Id, ip, MainlandIPAllowlistSourceSelf)
	require.NoError(t, err)
	require.Equal(t, row.ID, rowAgain.ID)
	var count int64
	require.NoError(t, db.Model(&MainlandIPAllowlist{}).Where("user_id = ?", user.Id).Count(&count).Error)
	require.EqualValues(t, 1, count)

	// Identity changes revoke prior exceptions before granting the new state.
	_, next, err = SetUserIdentity(user.Id, 99, IdentityTypeEducation)
	require.NoError(t, err)
	require.Equal(t, IdentityTypeEducation, next)
	require.False(t, IsMainlandIPWhitelisted(ip))

	_, next, err = SetUserIdentity(user.Id, 99, IdentityTypeNone)
	require.NoError(t, err)
	require.Equal(t, IdentityTypeNone, next)
	_, err = AddMainlandIPWhitelist(user.Id, user.Id, ip, MainlandIPAllowlistSourceSelf)
	require.ErrorIs(t, err, ErrIdentityRequired)

	// A stale allowlist row cannot survive an out-of-band identity revocation.
	// The middleware performs a current-identity check in addition to the
	// transactional revoke above.
	require.NoError(t, db.Model(&UserIdentity{}).Where("user_id = ?", user.Id).
		Update("identity_type", IdentityTypeEnterprise).Error)
	row, err = AddMainlandIPWhitelist(user.Id, user.Id, ip, MainlandIPAllowlistSourceSelf)
	require.NoError(t, err)
	require.NoError(t, db.Model(&UserIdentity{}).Where("user_id = ?", user.Id).
		Update("identity_type", IdentityTypeNone).Error)
	require.False(t, IsMainlandIPWhitelisted(ip))
}

func TestMainlandAllowlistSupportsIPv6AndRejectsUnauthorisedIdentity(t *testing.T) {
	db := setupIdentityAccessTestDB(t)
	user := User{Username: "ipv6-user", Password: "hashed", Role: 1, Status: 1}
	require.NoError(t, db.Create(&user).Error)
	_, err := AddMainlandIPWhitelist(user.Id, user.Id, net.ParseIP("2001:db8::1"), MainlandIPAllowlistSourceSelf)
	require.ErrorIs(t, err, ErrIdentityRequired)
	require.NoError(t, db.Create(&UserIdentity{UserID: user.Id, IdentityType: IdentityTypeEducation}).Error)
	ip := net.ParseIP("2001:db8::1")
	row, err := AddMainlandIPWhitelist(user.Id, user.Id, ip, MainlandIPAllowlistSourceSelf)
	require.NoError(t, err)
	require.Equal(t, "ipv6", row.AddressFamily)
	require.Equal(t, 128, row.PrefixLength)
	require.True(t, IsMainlandIPWhitelisted(ip))
}
