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
}
