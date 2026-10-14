package service

import (
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSecurityConcurrentRemovalKeepsOneLogin(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "auth.db")), &gorm.Config{})
	require.NoError(t, err)
	sql, err := db.DB()
	require.NoError(t, err)
	sql.SetMaxOpenConns(4)
	old := model.DB
	model.DB = db
	settings := system_setting.GetPasskeySettings()
	enabled := settings.Enabled
	settings.Enabled = true
	t.Cleanup(func() { model.DB = old; settings.Enabled = enabled; sql.Close(); oauth.Unregister("security-test") })
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.PasskeyCredential{}, &model.UserOAuthBinding{}, &model.DashboardSession{}, &model.SecurityProof{}))
	require.NoError(t, db.Create(&model.User{Id: 7, Username: "user"}).Error)
	require.NoError(t, db.Create(&model.PasskeyCredential{UserID: 7, CredentialID: "credential", PublicKey: "key"}).Error)
	require.NoError(t, db.Create(&model.UserOAuthBinding{UserId: 7, ProviderId: 99, ProviderUserId: "external-user"}).Error)
	oauth.RegisterCustom("security-test", oauth.NewGenericOAuthProvider(&model.CustomOAuthProvider{Id: 99, Slug: "security-test", Enabled: true}))
	var workers sync.WaitGroup
	var success atomic.Int32
	for _, passkey := range []bool{true, false} {
		workers.Add(1)
		go func(passkey bool) {
			defer workers.Done()
			provider := 99
			if passkey {
				provider = 0
			}
			if RemoveLoginMethod(7, passkey, provider) == nil {
				success.Add(1)
			}
		}(passkey)
	}
	workers.Wait()
	require.EqualValues(t, 1, success.Load())
	var keys, bindings int64
	require.NoError(t, db.Model(&model.PasskeyCredential{}).Count(&keys).Error)
	require.NoError(t, db.Model(&model.UserOAuthBinding{}).Count(&bindings).Error)
	require.EqualValues(t, 1, keys+bindings)
}
