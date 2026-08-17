package model

import (
	"testing"
	"time"

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

func TestTokenRouteQuotaAvailabilityFreezesToSubscriptionReset(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	now := common.GetTimestamp()
	resetAt := now + 3600
	require.NoError(t, db.Create(&UserSubscription{
		UserId: 91, PlanId: 1, PlanTitle: "hourly", AmountTotal: 100, AmountUsed: 100,
		StartTime: now - 60, EndTime: now + 86400, Status: "active",
		AllowedGroup: "package-a", NextResetTime: resetAt,
	}).Error)

	availability, err := GetTokenRouteQuotaAvailability(91, "gpt-test", TokenRouteStep{
		GroupName: "package-a", FundingSource: TokenRouteSourceSubscription,
		SelectionMode: TokenRouteSelectionAuto,
	}, 1)
	require.NoError(t, err)
	require.False(t, availability.Usable)
	require.Equal(t, resetAt, availability.ResetAt)
}

func TestVirtualMembershipRouteDetectsEarlyReset(t *testing.T) {
	db := setupSubscriptionBindingTestDB(t)
	now := time.Now().Unix()
	membership := UserVirtualMembership{
		UserId: 92, PlanId: 1, PlanTitle: "membership", WeeklyQuota: 100, WeeklyUsed: 100,
		WeeklyResetAt: now + 7200, StartTime: now - 60, EndTime: now + 86400,
		Status: VirtualMembershipStatusActive, AllowedGroup: "member-a", AllowedModels: "gpt-test",
	}
	require.NoError(t, db.Create(&membership).Error)
	step := TokenRouteStep{
		GroupName: "member-a", FundingSource: TokenRouteSourceVirtualMembership,
		SelectionMode: TokenRouteSelectionInstance, SourceId: membership.Id,
	}

	availability, err := GetTokenRouteQuotaAvailability(92, "gpt-test", step, 1)
	require.NoError(t, err)
	require.False(t, availability.Usable)
	require.Equal(t, membership.WeeklyResetAt, availability.ResetAt)

	require.NoError(t, db.Model(&UserVirtualMembership{}).Where("id = ?", membership.Id).Update("weekly_used", 0).Error)
	availability, err = GetTokenRouteQuotaAvailability(92, "gpt-test", step, 1)
	require.NoError(t, err)
	require.True(t, availability.Usable, "an operator-triggered early membership reset must unfreeze the route")
	require.Zero(t, availability.ResetAt)
}
