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
	require.True(t, DB.Migrator().HasColumn(&SubscriptionPlan{}, "number_pool"))
}
