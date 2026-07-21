package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestAgentCommissionRatesAreConfigurableOptions(t *testing.T) {
	originalConsumptionRate := common.AgentAffiliateDirectRate
	originalOrderRate := common.AgentOrderAffiliateDirectRate
	common.OptionMapRWMutex.RLock()
	originalOptionMap := common.OptionMap
	common.OptionMapRWMutex.RUnlock()
	t.Cleanup(func() {
		common.AgentAffiliateDirectRate = originalConsumptionRate
		common.AgentOrderAffiliateDirectRate = originalOrderRate
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	common.AgentAffiliateDirectRate = 0.20
	common.AgentOrderAffiliateDirectRate = 0.20
	common.OptionMapRWMutex.Lock()
	common.OptionMap = make(map[string]string)
	initAgentCommissionOptionMap()
	common.OptionMapRWMutex.Unlock()
	if got := common.OptionMap["AgentAffiliateDirectRate"]; got != "0.2" {
		t.Fatalf("AgentAffiliateDirectRate option = %q, want %q", got, "0.2")
	}
	if got := common.OptionMap["AgentOrderAffiliateDirectRate"]; got != "0.2" {
		t.Fatalf("AgentOrderAffiliateDirectRate option = %q, want %q", got, "0.2")
	}

	updateOptionMap("AgentAffiliateDirectRate", "0.25")
	updateOptionMap("AgentOrderAffiliateDirectRate", "0.18")
	if common.AgentAffiliateDirectRate != 0.25 {
		t.Fatalf("AgentAffiliateDirectRate = %v, want 0.25", common.AgentAffiliateDirectRate)
	}
	if common.AgentOrderAffiliateDirectRate != 0.18 {
		t.Fatalf("AgentOrderAffiliateDirectRate = %v, want 0.18", common.AgentOrderAffiliateDirectRate)
	}
}

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
