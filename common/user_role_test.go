package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentRoleHasNoAdminPrivilegeAndCanWithdrawCommission(t *testing.T) {
	require.True(t, IsValidateRole(RoleAgentUser))
	require.Less(t, RoleAgentUser, RoleAdminUser)
	require.True(t, AffiliateRewardIsWithdrawable(RoleAgentUser))
	require.False(t, AffiliateRewardIsWithdrawable(RoleCommonUser))
	require.True(t, CanWithdrawDividend(RoleAgentUser))
	require.True(t, CanWithdrawDividend(RoleAdminUser))
	require.False(t, CanWithdrawDividend(RoleCommonUser))
}
