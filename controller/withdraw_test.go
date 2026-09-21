package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func TestWithdrawPaymentInfoRequirements(t *testing.T) {
	tests := []struct {
		name     string
		wtype    int
		role     int
		expected bool
	}{
		{name: "principal", wtype: model.WithdrawTypePrincipal, role: common.RoleCommonUser, expected: true},
		{name: "agent commission", wtype: model.WithdrawTypeDividend, role: common.RoleAgentUser, expected: true},
		{name: "admin dividend", wtype: model.WithdrawTypeDividend, role: common.RoleAdminUser, expected: false},
		{name: "root dividend", wtype: model.WithdrawTypeDividend, role: common.RoleRootUser, expected: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := withdrawPaymentInfoRequired(test.wtype, test.role); got != test.expected {
				t.Fatalf("withdrawPaymentInfoRequired(%d, %d) = %v, want %v", test.wtype, test.role, got, test.expected)
			}
		})
	}
}

func TestWithdrawNotificationOnlyTargetsTheReviewMailbox(t *testing.T) {
	if got := withdrawNotificationRecipients(); got != withdrawNotifyEmail {
		t.Fatalf("withdraw notification recipients = %q, want %q", got, withdrawNotifyEmail)
	}
}
