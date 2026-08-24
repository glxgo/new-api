package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRechargeIdentityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &UserIdentity{}, &MainlandIPAllowlist{}, &RechargeCredit{}, &Option{}))
	oldDB, oldMaster, oldRedis := DB, common.IsMasterNode, common.RedisEnabled
	DB, common.IsMasterNode, common.RedisEnabled = db, true, false
	t.Cleanup(func() {
		DB, common.IsMasterNode, common.RedisEnabled = oldDB, oldMaster, oldRedis
	})
	return db
}

func TestRechargeAutomaticallyGrantsEnterpriseIdentityAfterMoreThanOneThousand(t *testing.T) {
	db := setupRechargeIdentityTestDB(t)
	user := User{Username: "recharge-enterprise", Password: "hashed", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		created, err := RecordLegacyRechargeCreditTx(tx, user.Id, EnterpriseIdentityRechargeThresholdCents, RechargeSourceWalletTopUp, "threshold", 1)
		require.True(t, created)
		return err
	}))
	var identity UserIdentity
	require.ErrorIs(t, db.Where("user_id = ?", user.Id).First(&identity).Error, gorm.ErrRecordNotFound)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		created, err := RecordLegacyRechargeCreditTx(tx, user.Id, 1, RechargeSourceWalletTopUp, "over-threshold", 2)
		require.True(t, created)
		return err
	}))
	require.NoError(t, db.Where("user_id = ?", user.Id).First(&identity).Error)
	require.Equal(t, IdentityTypeEnterprise, identity.IdentityType)
	require.Zero(t, identity.IdentityVerifiedBy, "automatic grants are system decisions")
}

func TestRechargeAutomaticIdentityPreservesEducationAndBackfillsExistingUsers(t *testing.T) {
	db := setupRechargeIdentityTestDB(t)
	education := User{Username: "education-user", Password: "hashed", AffCode: "education-user", RechargeTotalCents: EnterpriseIdentityRechargeThresholdCents + 1}
	legacy := User{Username: "legacy-enterprise", Password: "hashed", AffCode: "legacy-enterprise", RechargeTotalCents: EnterpriseIdentityRechargeThresholdCents + 1}
	require.NoError(t, db.Create(&education).Error)
	require.NoError(t, db.Create(&legacy).Error)
	require.NoError(t, db.Create(&UserIdentity{UserID: education.Id, IdentityType: IdentityTypeEducation}).Error)

	require.NoError(t, MigrateRechargeEnterpriseIdentitiesV1())
	var educationIdentity, legacyIdentity UserIdentity
	require.NoError(t, db.Where("user_id = ?", education.Id).First(&educationIdentity).Error)
	require.NoError(t, db.Where("user_id = ?", legacy.Id).First(&legacyIdentity).Error)
	require.Equal(t, IdentityTypeEducation, educationIdentity.IdentityType)
	require.Equal(t, IdentityTypeEnterprise, legacyIdentity.IdentityType)

	var marker Option
	require.NoError(t, db.Where("key = ?", rechargeEnterpriseIdentityMigrationKey).First(&marker).Error)
	require.Equal(t, "true", marker.Value)
	require.NoError(t, MigrateRechargeEnterpriseIdentitiesV1())
}
