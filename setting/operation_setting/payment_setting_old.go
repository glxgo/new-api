/**
此文件为旧版支付设置文件，如需增加新的参数、变量等，请在 payment_setting.go 中添加
This file is the old version of the payment settings file. If you need to add new parameters, variables, etc., please add them in payment_setting.go
*/

package operation_setting

import (
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

var PayAddress = ""
var CustomCallbackAddress = ""
var EpayId = ""
var EpayKey = ""
var Price = 7.3
var MinTopUp = 1
var USDExchangeRate = 7.3

var PayMethods = []map[string]string{
	{
		"name":  "支付宝",
		"color": "rgba(var(--semi-blue-5), 1)",
		"type":  "alipay",
	},
	{
		"name":  "微信",
		"color": "rgba(var(--semi-green-5), 1)",
		"type":  "wxpay",
	},
	{
		"name":      "自定义1",
		"color":     "black",
		"type":      "custom1",
		"min_topup": "50",
	},
}

func UpdatePayMethodsByJsonString(jsonString string) error {
	PayMethods = make([]map[string]string, 0)
	return common.Unmarshal([]byte(jsonString), &PayMethods)
}

func PayMethods2JsonString() string {
	jsonBytes, err := common.Marshal(PayMethods)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}

func ContainsPayMethod(method string) bool {
	for _, payMethod := range PayMethods {
		if payMethod["type"] == method {
			return true
		}
	}
	return false
}

// GetPaymentMethodFeeRate returns the configured percentage surcharge for an
// Epay payment method. The value is read from the method's `fee_rate` field
// (percent, e.g. 2.5). `fee_percent` and the legacy-friendly `fee` aliases
// are accepted as well. Invalid, negative, or over-100 values fail closed to
// zero so a malformed admin setting can never overcharge a customer.
func GetPaymentMethodFeeRate(method string) float64 {
	method = strings.TrimSpace(method)
	if method == "" {
		return 0
	}
	for _, payMethod := range PayMethods {
		if strings.TrimSpace(payMethod["type"]) != method {
			continue
		}
		for _, key := range []string{"fee_rate", "fee_percent", "fee"} {
			raw := strings.TrimSpace(payMethod[key])
			if raw == "" {
				continue
			}
			rate, err := strconv.ParseFloat(raw, 64)
			if err != nil || math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 || rate > 100 {
				return 0
			}
			return rate
		}
		return 0
	}
	return 0
}

// PaymentAmountWithFee rounds both the surcharge and final amount to cents,
// matching the amount sent to Epay. The first return value is the fee, and
// the second is the total amount charged. Callers should retain their base
// product amount separately for balance/entitlement settlement.
func PaymentAmountWithFee(baseAmount float64, method string) (float64, float64) {
	if baseAmount <= 0 {
		return 0, baseAmount
	}
	rate := GetPaymentMethodFeeRate(method)
	fee := math.Round(baseAmount*rate) / 100
	fee = math.Round(fee*100) / 100
	total := math.Round((baseAmount+fee)*100) / 100
	return fee, total
}
