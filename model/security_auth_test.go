package model

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func securityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	sql, err := db.DB()
	require.NoError(t, err)
	sql.SetMaxOpenConns(1)
	old, redis := DB, common.RedisEnabled
	DB = db
	common.RedisEnabled = false
	t.Cleanup(func() { DB = old; common.RedisEnabled = redis; sql.Close() })
	require.NoError(t, db.AutoMigrate(&User{}, &Option{}, &PasskeyCredential{}, &UserOAuthBinding{}, &DashboardSession{}, &SecurityProof{}, &SecurityCeremony{}, &TwoFA{}, &TwoFABackupCode{}))
	return db
}

func TestSecurityPendingFactorCannotOverwriteEnabled(t *testing.T) {
	db := securityTestDB(t)
	require.NoError(t, db.Create(&User{Id: 7, Username: "factor"}).Error)
	factor := TwoFA{UserId: 7, Secret: "original", IsEnabled: true}
	require.NoError(t, db.Create(&factor).Error)
	require.Error(t, StorePendingTwoFA(7, "replacement", []string{"12345678"}))
	require.NoError(t, db.First(&factor, factor.Id).Error)
	require.Equal(t, "original", factor.Secret)
}

func TestSecurityBindingUniquenessOnlyChecksChangedIdentities(t *testing.T) {
	db := securityTestDB(t)
	first := User{Id: 7, Username: "one", AffCode: "one", Email: "historical@example.com"}
	second := User{Id: 8, Username: "two", AffCode: "two", Email: first.Email}
	require.NoError(t, db.Create(&first).Error)
	require.NoError(t, db.Create(&second).Error)
	first.DisplayName = "profile edit"
	require.NoError(t, first.Update(false))
	second.Email = "new@example.com"
	require.NoError(t, second.Update(false))
	first.Email = second.Email
	require.Error(t, first.Update(false))
	var stored User
	require.NoError(t, db.First(&stored, first.Id).Error)
	require.Equal(t, "historical@example.com", stored.Email)
}

func TestSecurityProofBindingsReplayAndExpiry(t *testing.T) {
	db := securityTestDB(t)
	token, err := NewSecurityProof(7, "session", "user.delete", "target-9")
	require.NoError(t, err)
	require.Error(t, ConsumeSecurityProof(token, 7, "session", "user.delete", "target-9"))
	require.NoError(t, VerifySecurityProof(token, 7, "session", "password"))
	for _, args := range [][4]string{{"wrong-session", "user.delete", "target-9", ""}, {"session", "user.role", "target-9", ""}, {"session", "user.delete", "target-10", ""}} {
		require.Error(t, ConsumeSecurityProof(token, 7, args[0], args[1], args[2]))
	}
	require.Error(t, ConsumeSecurityProof(token, 8, "session", "user.delete", "target-9"))
	var won atomic.Int32
	var workers sync.WaitGroup
	for i := 0; i < 12; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if ConsumeSecurityProof(token, 7, "session", "user.delete", "target-9") == nil {
				won.Add(1)
			}
		}()
	}
	workers.Wait()
	require.EqualValues(t, 1, won.Load())
	token, err = NewSecurityProof(7, "session", "a", "b")
	require.NoError(t, err)
	require.NoError(t, db.Model(&SecurityProof{}).Where("id = ?", SecurityDigest(token)).Update("expires_at", time.Now().Unix()-1).Error)
	_, err = GetSecurityProof(token, 7, "session")
	require.Error(t, err)
}

func TestSecuritySessionRevocationAndStalePassword(t *testing.T) {
	db := securityTestDB(t)
	user := User{Id: 7, Username: "security", Password: "original-hash", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: 100, GiftQuota: 40}
	require.NoError(t, db.Create(&user).Error)
	token, err := SecurityToken()
	require.NoError(t, err)
	require.NoError(t, CreateDashboardSession(&user, token))
	_, err = ValidateDashboardSession(7, token)
	require.NoError(t, err)
	stale := user
	require.NoError(t, db.Model(&user).Updates(map[string]any{"password": "newer-hash", "quota": 70, "gift_quota": 25}).Error)
	stale.DisplayName = "updated"
	require.NoError(t, stale.Update(false))
	var stored User
	require.NoError(t, db.First(&stored, 7).Error)
	require.Equal(t, "newer-hash", stored.Password)
	require.Equal(t, 70, stored.Quota)
	require.Equal(t, 25, stored.GiftQuota)
	require.Error(t, CreateDashboardSession(&stale, token))
	_, err = ValidateDashboardSession(7, token)
	require.Error(t, err)
	token, err = SecurityToken()
	require.NoError(t, err)
	require.NoError(t, CreateDashboardSession(&stored, token))
	require.NoError(t, RevokeUserAuthentication(db, 7))
	_, err = ValidateDashboardSession(7, token)
	require.Error(t, err)
	require.Error(t, CreateDashboardSession(&stored, token))
}

func TestSecurityAccountDeletionRequiresCurrentSession(t *testing.T) {
	db := securityTestDB(t)
	user := User{Id: 7, Username: "delete", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)
	token, err := SecurityToken()
	require.NoError(t, err)
	require.NoError(t, CreateDashboardSession(&user, token))
	require.NoError(t, RevokeDashboardSession(token))
	require.Error(t, DeleteAccountWithSession(user.Id, token))
	require.NoError(t, db.First(&user, user.Id).Error)
	token, err = SecurityToken()
	require.NoError(t, err)
	require.NoError(t, CreateDashboardSession(&user, token))
	require.NoError(t, DeleteAccountWithSession(user.Id, token))
	require.ErrorIs(t, db.First(&User{}, user.Id).Error, gorm.ErrRecordNotFound)
	_, err = ValidateDashboardSession(user.Id, token)
	require.Error(t, err)
}

func TestSecurityCeremonyOnlyOneConsumer(t *testing.T) {
	securityTestDB(t)
	token, err := SaveSecurityCeremony("private-state")
	require.NoError(t, err)
	var won atomic.Int32
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			payload, err := PopSecurityCeremony(token)
			if err == nil && payload == "private-state" {
				won.Add(1)
			}
		}()
	}
	workers.Wait()
	require.EqualValues(t, 1, won.Load())
}

func TestSecurityTOTPReplaySurvivesStaleFactorUpdate(t *testing.T) {
	db := securityTestDB(t)
	factor := TwoFA{UserId: 7, Secret: "JBSWY3DPEHPK3PXP", IsEnabled: true}
	require.NoError(t, db.Create(&factor).Error)
	stale := factor
	code, err := totp.GenerateCode(factor.Secret, time.Now())
	require.NoError(t, err)
	ok, err := factor.ValidateTOTPAndUpdateUsage(code)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, stale.IncrementFailedAttempts())
	require.NoError(t, stale.ResetFailedAttempts())
	ok, err = stale.ValidateTOTPAndUpdateUsage(code)
	require.NoError(t, err)
	require.False(t, ok)
}
