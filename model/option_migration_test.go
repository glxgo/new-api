package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestOptionKeyWhereClauseQuotesReservedColumn(t *testing.T) {
	originalPostgreSQL := common.UsingPostgreSQL
	t.Cleanup(func() {
		common.UsingPostgreSQL = originalPostgreSQL
		initCol()
	})

	common.UsingPostgreSQL = false
	initCol()
	if got := optionKeyWhereClause(); got != "`key` = ?" {
		t.Fatalf("MySQL/SQLite option key clause = %q, want %q", got, "`key` = ?")
	}

	common.UsingPostgreSQL = true
	initCol()
	if got := optionKeyWhereClause(); got != `"key" = ?` {
		t.Fatalf("PostgreSQL option key clause = %q, want %q", got, `"key" = ?`)
	}
}

func TestDefaultUserCapacityOptionsAreIndependent(t *testing.T) {
	oldConcurrency := common.DefaultUserConcurrencyLimit
	oldRPM := common.DefaultUserRPMLimit
	common.OptionMapRWMutex.RLock()
	oldOptionMap := common.OptionMap
	common.OptionMapRWMutex.RUnlock()
	t.Cleanup(func() {
		common.DefaultUserConcurrencyLimit = oldConcurrency
		common.DefaultUserRPMLimit = oldRPM
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	common.OptionMapRWMutex.Lock()
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	require.NoError(t, updateOptionMap("DefaultUserConcurrencyLimit", "16"))
	require.NoError(t, updateOptionMap("DefaultUserRPMLimit", "45"))
	require.Equal(t, 16, common.DefaultUserConcurrencyLimit)
	require.Equal(t, 45, common.DefaultUserRPMLimit)
}

func TestCyberPolicyInterceptionAndEnforcementOptionsAreIndependent(t *testing.T) {
	oldInterception := common.CyberPolicyInterceptionEnabled
	oldEnforcement := common.CyberPolicyEnforcementEnabled
	common.OptionMapRWMutex.RLock()
	oldOptionMap := common.OptionMap
	common.OptionMapRWMutex.RUnlock()
	t.Cleanup(func() {
		common.CyberPolicyInterceptionEnabled = oldInterception
		common.CyberPolicyEnforcementEnabled = oldEnforcement
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	common.OptionMapRWMutex.Lock()
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	require.NoError(t, updateOptionMap("CyberPolicyInterceptionEnabled", "true"))
	require.NoError(t, updateOptionMap("CyberPolicyEnforcementEnabled", "false"))
	require.True(t, common.CyberPolicyInterceptionEnabled)
	require.False(t, common.CyberPolicyEnforcementEnabled)
}
