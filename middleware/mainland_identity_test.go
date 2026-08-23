package middleware

import (
	"net"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/access_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMainlandWhitelistRunsBeforeGeoIPBlock(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserIdentity{}, &model.MainlandIPAllowlist{}))
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB })
	user := model.User{Username: "allowlist-user", Password: "hashed", Status: 1}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&model.UserIdentity{UserID: user.Id, IdentityType: model.IdentityTypeEnterprise}).Error)
	_, err = model.AddMainlandIPWhitelist(user.Id, user.Id, net.ParseIP("203.0.113.10"), model.MainlandIPAllowlistSourceSelf)
	require.NoError(t, err)

	oldPolicy := *access_setting.GetAccessPolicy()
	t.Cleanup(func() { *access_setting.GetAccessPolicy() = oldPolicy })
	access_setting.GetAccessPolicy().BlockMainlandWebAccess = true
	access_setting.GetAccessPolicy().GeoIPUnknownPolicy = access_setting.GeoIPUnknownPolicyDeny
	r := newMainlandTestRouter(func(ip net.IP) (string, bool) {
		if ip.String() == "203.0.113.10" {
			return "CN", true
		}
		return "US", true
	})
	w := requestMainland(r, "/")
	require.Equal(t, http.StatusOK, w.Code)
}
