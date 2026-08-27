package operation_setting

import "testing"

func TestPaymentAmountWithFee(t *testing.T) {
	original := PayMethods
	t.Cleanup(func() { PayMethods = original })
	PayMethods = []map[string]string{{"type": "alipay", "fee_rate": "2.5"}}

	fee, total := PaymentAmountWithFee(100, "alipay")
	if fee != 2.5 || total != 102.5 {
		t.Fatalf("fee/total = %.2f/%.2f, want 2.50/102.50", fee, total)
	}
	fee, total = PaymentAmountWithFee(19.99, "alipay")
	if fee != 0.5 || total != 20.49 {
		t.Fatalf("rounded fee/total = %.2f/%.2f, want 0.50/20.49", fee, total)
	}
}

func TestPaymentMethodFeeRateInvalidFailsClosed(t *testing.T) {
	original := PayMethods
	t.Cleanup(func() { PayMethods = original })
	PayMethods = []map[string]string{
		{"type": "bad", "fee_rate": "101"},
		{"type": "nan", "fee_rate": "NaN"},
	}
	if got := GetPaymentMethodFeeRate("bad"); got != 0 {
		t.Fatalf("invalid fee rate = %.2f, want 0", got)
	}
	if got := GetPaymentMethodFeeRate("nan"); got != 0 {
		t.Fatalf("NaN fee rate = %.2f, want 0", got)
	}
}
