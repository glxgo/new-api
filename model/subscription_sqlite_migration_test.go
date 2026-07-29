package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestEnsureSubscriptionPlanTableSQLiteAddsDisplayFields(t *testing.T) {
	previousDB := DB
	previousUsingSQLite := common.UsingSQLite
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:subscription-plan-migration-%s?mode=memory&cache=shared", t.Name())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	DB = db
	common.UsingSQLite = true
	t.Cleanup(func() {
		DB = previousDB
		common.UsingSQLite = previousUsingSQLite
	})

	require.NoError(t, ensureSubscriptionPlanTableSQLite())
	require.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, "suitable_for"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, "description"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, "allowed_group"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, "renewal_plan_id"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, "number_pool"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, "model_limit"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, "plan_version"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, "recommended"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, "min_ratio"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, "amount_cap"))
	require.True(t, DB.Migrator().HasIndex(&SubscriptionPlan{}, "idx_subscription_plans_renewal_plan_id"))
}

func TestPrepareUserSubscriptionRenewalSQLiteMigration(t *testing.T) {
	previousDB := DB
	previousUsingSQLite := common.UsingSQLite
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:user-subscription-renewal-migration-%s?mode=memory&cache=shared", t.Name())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	DB = db
	common.UsingSQLite = true
	t.Cleanup(func() {
		DB = previousDB
		common.UsingSQLite = previousUsingSQLite
	})

	require.NoError(t, DB.Exec("CREATE TABLE user_subscriptions (`id` integer PRIMARY KEY)").Error)
	require.NoError(t, prepareUserSubscriptionRenewalSQLiteMigration())
	require.NoError(t, DB.AutoMigrate(&UserSubscription{}))
	require.True(t, DB.Migrator().HasColumn(&UserSubscription{}, "renewed_from_id"))
	require.True(t, DB.Migrator().HasIndex(&UserSubscription{}, "idx_user_subscriptions_renewed_from_id"))

	sourceID := 99
	first := UserSubscription{UserId: 1, Status: "active", RenewedFromId: &sourceID}
	second := UserSubscription{UserId: 1, Status: "active", RenewedFromId: &sourceID}
	require.NoError(t, DB.Create(&first).Error)
	require.Error(t, DB.Create(&second).Error)
}
