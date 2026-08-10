package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitPaymentUsesGiftBeforePrincipal(t *testing.T) {
	gift, principal := SplitPayment(1_000, 400, 600)
	require.Equal(t, 400, gift)
	require.Equal(t, 600, principal)
}

func TestChannelCostObservationIsOptional(t *testing.T) {
	require.Zero(t, CalcCostFromChannelRatio(1_000, nil))
	ratio := int64(850_000)
	require.Equal(t, 850, CalcCostFromChannelRatio(1_000, &ratio))
}
