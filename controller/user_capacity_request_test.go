package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestUpdateUserRequestCapacityFieldsArePresenceAware(t *testing.T) {
	var legacy updateUserRequest
	require.NoError(t, common.Unmarshal([]byte(`{"id":7,"username":"legacy"}`), &legacy))
	require.Nil(t, legacy.ConcurrencyLimit)
	require.Nil(t, legacy.ConcurrencyLimitOverride)
	require.Nil(t, legacy.RPMLimit)
	require.Nil(t, legacy.RPMLimitOverride)

	var explicit updateUserRequest
	require.NoError(t, common.Unmarshal([]byte(`{"id":7,"concurrency_limit":20,"concurrency_limit_override":true,"rpm_limit":50,"rpm_limit_override":false}`), &explicit))
	require.Equal(t, 20, *explicit.ConcurrencyLimit)
	require.True(t, *explicit.ConcurrencyLimitOverride)
	require.Equal(t, 50, *explicit.RPMLimit)
	require.False(t, *explicit.RPMLimitOverride)
}
