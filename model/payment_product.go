package model

import (
	"math"
	"strings"
	"unicode"

	"github.com/shopspring/decimal"
)

// PaymentProductNameMaxLength keeps product names accepted by Epay and other
// gateways bounded while still leaving enough room for a readable plan title.
const PaymentProductNameMaxLength = 128

// sanitizePaymentProductPart removes control characters and collapses
// whitespace. Product names are display/audit metadata; they must never be
// allowed to contain a newline or other gateway-form-breaking characters.
func sanitizePaymentProductPart(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
	return strings.Join(strings.Fields(value), " ")
}

func truncatePaymentProductName(value string) string {
	value = sanitizePaymentProductPart(value)
	runes := []rune(value)
	if len(runes) <= PaymentProductNameMaxLength {
		return value
	}
	return string(runes[:PaymentProductNameMaxLength])
}

// PaymentProductNameForTopUp returns the stable, human-readable name sent to
// Epay for a wallet top-up. The amount is the same server-calculated amount
// used by the Epay Money field; no quota/token/USD unit is exposed.
func PaymentProductNameForTopUp(payMoney float64) string {
	if math.IsNaN(payMoney) || math.IsInf(payMoney, 0) || payMoney < 0 {
		payMoney = 0
	}
	amount := FormatPaymentProductAmount(payMoney)
	return truncatePaymentProductName("用户余额充值" + amount)
}

// PaymentProductNameForSubscription returns the stable name for a normal
// subscription purchase. It deliberately avoids internal order prefixes such
// as SUB: and uses the title captured at order creation time.
func PaymentProductNameForSubscription(title string) string {
	title = truncatePaymentProductName(title)
	if title == "" {
		title = "套餐"
	}
	return truncatePaymentProductName("订阅套餐" + title)
}

// PaymentProductNameForVirtualMembership normalizes a virtual-membership
// title for gateway display. Only a standalone GPT/gpt token is removed;
// strings such as "GPTPro" are preserved. Separators are removed and the
// remaining tier is lower-cased so "GPT Plus" becomes "plus" and the final
// product name becomes "虚拟会员plus".
func PaymentProductNameForVirtualMembership(title string) string {
	title = sanitizePaymentProductPart(title)
	parts := strings.FieldsFunc(title, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
	})
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.EqualFold(part, "gpt") {
			continue
		}
		kept = append(kept, part)
	}
	core := strings.ToLower(strings.Join(kept, ""))
	if core == "" {
		core = "方案"
	}
	return truncatePaymentProductName("虚拟会员" + core)
}

// PaymentProductNameForVirtualMembershipReset is kept alongside the regular
// virtual-membership naming so future reset-credit orders cannot accidentally
// reintroduce the old VM: technical prefix.
func PaymentProductNameForVirtualMembershipReset(title string) string {
	base := PaymentProductNameForVirtualMembership(title)
	return truncatePaymentProductName(base + "主动重置次数")
}

// FormatPaymentProductAmount is useful to tests and controllers that need to
// use exactly the same two-decimal representation as the product helper.
func FormatPaymentProductAmount(payMoney float64) string {
	if math.IsNaN(payMoney) || math.IsInf(payMoney, 0) || payMoney < 0 {
		payMoney = 0
	}
	value := decimal.NewFromFloat(payMoney).Round(2).StringFixed(2)
	value = strings.TrimRight(strings.TrimRight(value, "0"), ".")
	if value == "" {
		return "0"
	}
	return value
}

// FormatPaymentGatewayAmount is the canonical two-decimal representation
// used both in provider requests and in payment snapshots. Keeping this in
// one helper avoids Go float formatting (for example 1.005 -> 1.00) drifting
// from decimal rounding (1.005 -> 1.01).
func FormatPaymentGatewayAmount(amount float64) string {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 {
		amount = 0
	}
	return decimal.NewFromFloat(amount).Round(2).StringFixed(2)
}

func NormalizePaymentAmount(amount float64) float64 {
	value, err := decimal.NewFromString(FormatPaymentGatewayAmount(amount))
	if err != nil {
		return 0
	}
	return value.InexactFloat64()
}
