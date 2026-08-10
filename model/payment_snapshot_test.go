package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestVerifiedPaymentSnapshotProviderMatrix(t *testing.T) {
	oldPrice, oldQuota := operation_setting.Price, common.QuotaPerUnit
	operation_setting.Price, common.QuotaPerUnit = 7.3, 500_000
	t.Cleanup(func() { operation_setting.Price, common.QuotaPerUnit = oldPrice, oldQuota })

	strictProviders := []string{PaymentProviderEpay, PaymentProviderWaffo, PaymentProviderWaffoPancake}
	for _, provider := range strictProviders {
		t.Run(provider+" strict expectation", func(t *testing.T) {
			order := TopUp{PaymentProvider: provider}
			expected, err := NewPaymentSnapshotFromMinor(1_000, "USD")
			require.NoError(t, err)
			require.NoError(t, SetTopUpPaymentExpectation(&order, expected))
			require.ErrorIs(t, applyVerifiedTopUpPayment(&order, PaymentSnapshot{AmountMinor: 999, Currency: "USD"}), ErrPaymentSnapshotMismatch)
			require.ErrorIs(t, applyVerifiedTopUpPayment(&order, PaymentSnapshot{AmountMinor: 1_000, Currency: "CNY"}), ErrPaymentSnapshotMismatch)
			require.NoError(t, applyVerifiedTopUpPayment(&order, expected))
		})
	}

	firstWebhookProviders := []string{PaymentProviderStripe, PaymentProviderCreem}
	for _, provider := range firstWebhookProviders {
		t.Run(provider+" verified webhook freezes first actual", func(t *testing.T) {
			order := TopUp{PaymentProvider: provider}
			actual := PaymentSnapshot{AmountMinor: 875, Currency: "USD"}
			require.NoError(t, applyVerifiedTopUpPayment(&order, actual))
			require.EqualValues(t, 875, order.ExpectedPaymentAmountMinor)
			require.Equal(t, "USD", order.ExpectedPaymentCurrency)
			require.EqualValues(t, 875, order.ActualPaymentAmountMinor)
			require.ErrorIs(t, applyVerifiedTopUpPayment(&order, PaymentSnapshot{AmountMinor: 874, Currency: "USD"}), ErrPaymentSnapshotMismatch)
		})
	}

	t.Run("unsupported currency is rejected", func(t *testing.T) {
		_, err := NewPaymentSnapshotFromMinor(100, "EUR")
		require.ErrorIs(t, err, ErrUnsupportedPaymentCurrency)
	})
}

func TestVerifiedSubscriptionPaymentSnapshotProviderMatrix(t *testing.T) {
	oldPrice, oldQuota := operation_setting.Price, common.QuotaPerUnit
	operation_setting.Price, common.QuotaPerUnit = 7.3, 500_000
	t.Cleanup(func() { operation_setting.Price, common.QuotaPerUnit = oldPrice, oldQuota })

	for _, provider := range []string{PaymentProviderEpay, PaymentProviderWaffoPancake} {
		t.Run(provider+" subscription strict expectation", func(t *testing.T) {
			order := SubscriptionOrder{PaymentProvider: provider}
			expected := PaymentSnapshot{AmountMinor: 2_000, Currency: "USD"}
			require.NoError(t, SetSubscriptionOrderPaymentExpectation(&order, expected))
			require.ErrorIs(t, applyVerifiedSubscriptionPayment(&order, PaymentSnapshot{AmountMinor: 1_999, Currency: "USD"}), ErrPaymentSnapshotMismatch)
			require.NoError(t, applyVerifiedSubscriptionPayment(&order, expected))
		})
	}

	for _, provider := range []string{PaymentProviderStripe, PaymentProviderCreem} {
		t.Run(provider+" subscription freezes verified webhook", func(t *testing.T) {
			order := SubscriptionOrder{PaymentProvider: provider}
			actual := PaymentSnapshot{AmountMinor: 1_500, Currency: "CNY"}
			require.NoError(t, applyVerifiedSubscriptionPayment(&order, actual))
			require.EqualValues(t, 1_500, order.ExpectedPaymentAmountMinor)
			require.Equal(t, "CNY", order.ExpectedPaymentCurrency)
			require.ErrorIs(t, applyVerifiedSubscriptionPayment(&order, PaymentSnapshot{AmountMinor: 1_500, Currency: "USD"}), ErrPaymentSnapshotMismatch)
		})
	}
}

func TestEpayCommissionBaseDoesNotDriftAfterOrderCreation(t *testing.T) {
	oldPrice, oldQuota := operation_setting.Price, common.QuotaPerUnit
	operation_setting.Price, common.QuotaPerUnit = 7.3, 500_000
	t.Cleanup(func() { operation_setting.Price, common.QuotaPerUnit = oldPrice, oldQuota })

	order := TopUp{PaymentProvider: PaymentProviderEpay}
	expected, err := NewPaymentSnapshotFromMinor(7_300, "CNY")
	require.NoError(t, err)
	require.NoError(t, SetTopUpPaymentExpectation(&order, expected))
	require.EqualValues(t, 5_000_000, order.CommissionBaseQuota)

	operation_setting.Price = 10
	require.NoError(t, applyVerifiedTopUpPayment(&order, expected))
	require.EqualValues(t, 5_000_000, order.CommissionBaseQuota, "callback must consume the immutable order-time base")
}

func TestPaymentSnapshotRequiresExactMinorUnits(t *testing.T) {
	_, err := NewPaymentSnapshotFromDisplayAmount("10.001", "USD")
	require.Error(t, err)
	_, err = NewPaymentSnapshotFromDisplayAmount("10.00", "EUR")
	require.True(t, errors.Is(err, ErrUnsupportedPaymentCurrency))
}

func TestHistoricalSubscriptionVerifiedPayloadMatrix(t *testing.T) {
	tests := []struct {
		name, provider, payload, currency string
		minor                             int64
	}{
		{"stripe", PaymentProviderStripe, `{"amount_total":"875","currency":"usd"}`, "USD", 875},
		{"creem", PaymentProviderCreem, `{"object":{"order":{"amount_paid":7300,"currency":"CNY"}}}`, "CNY", 7_300},
		{"pancake", PaymentProviderWaffoPancake, `{"data":{"amount":"19.99","currency":"USD"}}`, "USD", 1_999},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot, err := historicalSubscriptionPaymentSnapshot(&SubscriptionOrder{PaymentProvider: tt.provider, ProviderPayload: tt.payload})
			require.NoError(t, err)
			require.Equal(t, tt.currency, snapshot.Currency)
			require.Equal(t, tt.minor, snapshot.AmountMinor)
		})
	}
	_, err := historicalSubscriptionPaymentSnapshot(&SubscriptionOrder{PaymentProvider: PaymentProviderStripe, ProviderPayload: `{}`})
	require.ErrorContains(t, err, "amount_total")
}
