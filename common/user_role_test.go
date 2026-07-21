package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentRoleHasNoAdminPrivilegeAndGetsTwentyPercentDirectRebate(t *testing.T) {
	require.True(t, IsValidateRole(RoleAgentUser))
	require.Less(t, RoleAgentUser, RoleAdminUser)
	require.Equal(t, 0.20, AffiliateDirectRateForRole(RoleAgentUser))
	require.Equal(t, AffiliateDirectRate, AffiliateDirectRateForRole(RoleCommonUser))
	require.Equal(t, 0.20, OrderAffiliateDirectRateForRole(RoleAgentUser))
	require.Equal(t, OrderAffiliateDirectRate, OrderAffiliateDirectRateForRole(RoleCommonUser))
	require.True(t, AffiliateRewardIsWithdrawable(RoleAgentUser))
	require.False(t, AffiliateRewardIsWithdrawable(RoleCommonUser))
	require.True(t, CanWithdrawDividend(RoleAgentUser))
	require.True(t, CanWithdrawDividend(RoleAdminUser))
	require.False(t, CanWithdrawDividend(RoleCommonUser))
}
