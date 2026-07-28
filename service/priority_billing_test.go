package service

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

type priorityFundingStub struct {
	settled int
}

func (f *priorityFundingStub) Source() string       { return BillingSourceWallet }
func (f *priorityFundingStub) PreConsume(int) error { return nil }
func (f *priorityFundingStub) Refund() error        { return nil }
func (f *priorityFundingStub) Settle(delta int) error {
	f.settled += delta
	return nil
}

func TestReserveRequiredPreConsumesTrustedPriorityQuota(t *testing.T) {
	funding := &priorityFundingStub{}
	info := &relaycommon.RelayInfo{IsPlayground: true}
	session := &BillingSession{
		relayInfo: info,
		funding:   funding,
		trusted:   true,
	}

	require.NoError(t, session.ReserveRequired(200))
	require.False(t, session.trusted)
	require.Equal(t, 200, session.preConsumedQuota)
	require.Equal(t, 200, funding.settled)
}
