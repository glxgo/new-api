package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
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
