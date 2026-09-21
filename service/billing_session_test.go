package service

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

type settleCallFundingStub struct {
	calls  int
	deltas []int
}

func (f *settleCallFundingStub) Source() string       { return BillingSourceVirtualMembership }
func (f *settleCallFundingStub) PreConsume(int) error { return nil }
func (f *settleCallFundingStub) Refund() error        { return nil }
func (f *settleCallFundingStub) Settle(delta int) error {
	f.calls++
	f.deltas = append(f.deltas, delta)
	return nil
}

func TestBillingSessionSettlesVirtualMembershipAtZeroDelta(t *testing.T) {
	funding := &settleCallFundingStub{}
	session := &BillingSession{
		relayInfo:        &relaycommon.RelayInfo{IsPlayground: true},
		funding:          funding,
		preConsumedQuota: 100,
	}

	require.NoError(t, session.Settle(100))
	require.Equal(t, 1, funding.calls)
	require.Equal(t, []int{0}, funding.deltas)
	require.True(t, session.fundingSettled)
	require.True(t, session.settled)

	require.NoError(t, session.Settle(100))
	require.Equal(t, 1, funding.calls, "settlement must remain idempotent")
}
