package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestTokenCustomRoutePersistsExplicitGroupsAndRevision(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	require.NoError(t, db.AutoMigrate(&TokenRouteStep{}))
	previousRatio := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"wallet-plus":1.2,"wallet-pro":2}`))
	t.Cleanup(func() { _ = ratio_setting.UpdateGroupRatioByJSONString(previousRatio) })

	token := Token{
		UserId: 83, Name: "route-key", Key: "route-key-secret",
		Status: common.TokenStatusEnabled, UnlimitedQuota: true,
		CreatedTime: common.GetTimestamp(), AccessedTime: common.GetTimestamp(), ExpiredTime: -1,
	}
	require.NoError(t, InsertTokenWithRoute(&token, []TokenRouteStep{
		{GroupName: "wallet-plus", SelectionMode: TokenRouteSelectionAuto},
		{GroupName: "wallet-pro", SelectionMode: TokenRouteSelectionAuto},
	}))
	require.Equal(t, TokenRoutingModeCustom, token.RoutingMode)
	require.EqualValues(t, 1, token.RoutingRevision)
	require.Equal(t, "wallet-plus", token.Group)

	steps, err := GetTokenRouteSteps(token.UserId, token.Id)
	require.NoError(t, err)
	require.Len(t, steps, 2)
	require.Equal(t, "wallet-plus", steps[0].GroupName)
	require.Equal(t, TokenRouteSourceWallet, steps[0].FundingSource)
	require.Equal(t, "wallet-pro", steps[1].GroupName)

	desired := token
	updated, err := UpdateTokenWithRoute(token.UserId, &desired, []TokenRouteStep{
		{GroupName: "wallet-pro", SelectionMode: TokenRouteSelectionAuto},
		{GroupName: "wallet-plus", SelectionMode: TokenRouteSelectionAuto},
	}, 1)
	require.NoError(t, err)
	require.EqualValues(t, 2, updated.RoutingRevision)
	require.Equal(t, "wallet-pro", updated.Group)

	_, err = UpdateTokenWithRoute(token.UserId, &desired, []TokenRouteStep{
		{GroupName: "wallet-plus", SelectionMode: TokenRouteSelectionAuto},
	}, 1)
	require.EqualError(t, err, "API Key 消耗路由策略已被其他窗口修改，请刷新后重试")
}
