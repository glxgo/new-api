package model

import "testing"

func TestPaymentProductNames(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "topup integer", got: PaymentProductNameForTopUp(50), want: "用户余额充值50"},
		{name: "topup decimal", got: PaymentProductNameForTopUp(50.5), want: "用户余额充值50.5"},
		{name: "subscription", got: PaymentProductNameForSubscription("专业版"), want: "订阅套餐专业版"},
		{name: "virtual gpt plus", got: PaymentProductNameForVirtualMembership("GPT Plus"), want: "虚拟会员plus"},
		{name: "virtual pro tier", got: PaymentProductNameForVirtualMembership("GPT Pro 5x"), want: "虚拟会员pro5x"},
		{name: "standalone only", got: PaymentProductNameForVirtualMembership("GPTPro"), want: "虚拟会员gptpro"},
		{name: "reset", got: PaymentProductNameForVirtualMembershipReset("GPT Plus"), want: "虚拟会员plus主动重置次数"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("got %q, want %q", test.got, test.want)
			}
		})
	}
}

func TestPaymentProductNameSanitizesAndBounds(t *testing.T) {
	name := PaymentProductNameForSubscription("  专业版\n\t订单\r\n")
	if name != "订阅套餐专业版 订单" {
		t.Fatalf("unexpected sanitized name %q", name)
	}
	longTitle := make([]rune, PaymentProductNameMaxLength+32)
	for i := range longTitle {
		longTitle[i] = 'x'
	}
	name = PaymentProductNameForSubscription(string(longTitle))
	if len([]rune(name)) > PaymentProductNameMaxLength {
		t.Fatalf("product name has %d runes, max %d", len([]rune(name)), PaymentProductNameMaxLength)
	}
}

func TestFormatPaymentGatewayAmountUsesDecimalRounding(t *testing.T) {
	if got := FormatPaymentGatewayAmount(1.005); got != "1.01" {
		t.Fatalf("gateway amount = %s, want 1.01", got)
	}
	if got := NormalizePaymentAmount(1.005); got != 1.01 {
		t.Fatalf("normalized amount = %v, want 1.01", got)
	}
}
