package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateGroupRatioWithRenamesSynchronizesAPIKeys(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}, &Token{}, &TokenRouteStep{}))

	oldDB, oldRedis := DB, common.RedisEnabled
	DB, common.RedisEnabled = db, false
	t.Cleanup(func() { DB, common.RedisEnabled = oldDB, oldRedis })

	require.NoError(t, db.Create(&Option{Key: "GroupRatio", Value: `{"old":0.5}`}).Error)
	tokens := []Token{
		{Id: 1, UserId: 9, Key: "single", Group: "old", PlannedSubscriptionGroup: "old"},
		{Id: 2, UserId: 9, Key: "custom", Group: "old", RoutingMode: TokenRoutingModeCustom},
	}
	require.NoError(t, db.Create(&tokens).Error)
	require.NoError(t, db.Create(&TokenRouteStep{TokenId: 2, UserId: 9, Position: 1, GroupName: "old", FundingSource: TokenRouteSourceWallet}).Error)

	require.NoError(t, UpdateGroupRatioWithRenames(`{"new":0.5}`, map[string]string{"old": "new"}))

	var saved []Token
	require.NoError(t, db.Order("id").Find(&saved).Error)
	require.Equal(t, "new", saved[0].Group)
	require.Equal(t, "new", saved[0].PlannedSubscriptionGroup)
	require.Equal(t, "new", saved[1].Group)
	var step TokenRouteStep
	require.NoError(t, db.First(&step).Error)
	require.Equal(t, "new", step.GroupName)
}
