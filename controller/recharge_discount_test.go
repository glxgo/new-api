package controller

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
	"gorm.io/gorm"
)

func TestRechargeDiscountQuoteAndEpayOrderStack(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.TopUpCoupon{}, &model.TopUpCouponUse{}))
	oldDB, oldRedis := model.DB, common.RedisEnabled
	initModelListColumnNames(t)
	oldPrice, oldMin := operation_setting.Price, operation_setting.MinTopUp
	oldDisplay := operation_setting.GetGeneralSetting().QuotaDisplayType
	oldDiscounts := operation_setting.GetPaymentSetting().AmountDiscount
	oldRatio := common.TopupGroupRatio2JSONString()
	oldAddress, oldID, oldKey := operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey
	oldMethods := operation_setting.PayMethods
	t.Cleanup(func() {
		model.DB = oldDB
		common.RedisEnabled = oldRedis
		operation_setting.Price = oldPrice
		operation_setting.MinTopUp = oldMin
		operation_setting.GetGeneralSetting().QuotaDisplayType = oldDisplay
		operation_setting.GetPaymentSetting().AmountDiscount = oldDiscounts
		require.NoError(t, common.UpdateTopupGroupRatioByJSONString(oldRatio))
		operation_setting.PayAddress = oldAddress
		operation_setting.EpayId = oldID
		operation_setting.EpayKey = oldKey
		operation_setting.PayMethods = oldMethods
	})
	model.DB = db
	common.RedisEnabled = false
	operation_setting.Price = 1
	operation_setting.MinTopUp = 1
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	operation_setting.GetPaymentSetting().AmountDiscount = map[int]float64{100: .9}
	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(`{"default":0.8}`))
	operation_setting.PayAddress = "https://payments.invalid"
	operation_setting.EpayId = "test"
	operation_setting.EpayKey = "test"
	operation_setting.PayMethods = []map[string]string{{"type": "alipay", "name": "Alipay", "fee_rate": "2.5"}}
	user := model.User{Username: "loyal", AffCode: "loyal", Group: "default", RechargeTotalCents: 50000}
	require.NoError(t, db.Create(&user).Error)
	coupon := model.TopUpCoupon{Code: "STACK", Title: "Stack", Discount: .95, UserLimit: 10, Enabled: true}
	require.NoError(t, model.SaveTopUpCoupon(&coupon))
	request := func(handler gin.HandlerFunc, body string) map[string]any {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("id", user.Id)
		c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		handler(c)
		var response map[string]any
		require.NoError(t, common.Unmarshal(w.Body.Bytes(), &response))
		return response
	}
	quote := request(RequestAmount, `{"amount":100,"coupon_code":"STACK"}`)
	require.Equal(t, "success", quote["message"])
	// 100 * group .8 * amount .9 * cumulative .97 * coupon .95 = 66.348
	require.Equal(t, "66.35", quote["data"])
	preview := request(PreviewTopUpCoupon, `{"amount":100,"coupon_code":"STACK"}`)
	require.Equal(t, true, preview["success"])
	order := request(RequestEpay, `{"amount":100,"payment_method":"alipay","coupon_code":"STACK"}`)
	require.Equal(t, "success", order["message"])
	params := order["data"].(map[string]any)
	require.Equal(t, "68.01", params["money"])
	var stored model.TopUp
	require.NoError(t, db.First(&stored).Error)
	require.EqualValues(t, 100, stored.Amount, "discount must not reduce purchased credits")
	require.EqualValues(t, 6801, stored.ExpectedPaymentAmountMinor)
	require.Equal(t, "CNY", stored.ExpectedPaymentCurrency)
	// Reaching the next tier affects subsequent quotes, not the existing order.
	require.NoError(t, db.Model(&user).Update("recharge_total_cents", 100000).Error)
	updated := request(RequestAmount, `{"amount":100,"coupon_code":"STACK"}`)
	require.Equal(t, "65.66", updated["data"])
	require.NoError(t, db.First(&stored).Error)
	require.EqualValues(t, 6801, stored.ExpectedPaymentAmountMinor)
}

type rechargeDiscountTransport func(*http.Request) (*http.Response, error)

func (f rechargeDiscountTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCreemRechargeDiscountIsAppliedAtProvider(t *testing.T) {
	oldTransport, oldKey := http.DefaultTransport, setting.CreemApiKey
	t.Cleanup(func() { http.DefaultTransport = oldTransport; setting.CreemApiKey = oldKey })
	setting.CreemApiKey = "test-local"
	var discountCode string
	http.DefaultTransport = rechargeDiscountTransport(func(r *http.Request) (*http.Response, error) {
		var payload map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &payload))
		body := `{}`
		switch r.URL.Path {
		case "/v1/discounts":
			require.Equal(t, float64(3), payload["percentage"])
			require.Equal(t, float64(1), payload["max_redemptions"])
			discountCode = payload["code"].(string)
			require.NotEmpty(t, discountCode)
		case "/v1/checkouts":
			require.NotEmpty(t, discountCode)
			require.Equal(t, discountCode, payload["discount_code"])
			require.Equal(t, "prod_test", payload["product_id"])
			body = `{"id":"checkout_test","checkout_url":"https://checkout.invalid/test"}`
		default:
			t.Fatalf("unexpected outbound request %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	url, err := genCreemLink(context.Background(), "ref_test", &CreemProduct{ProductId: "prod_test", Price: 100, Quota: 100}, "test@example.invalid", "test", .97)
	require.NoError(t, err)
	require.Equal(t, "https://checkout.invalid/test", url)
}

func TestStripeChargesDiscountedQuoteAndRetainsProviderPromotions(t *testing.T) {
	oldBackend, oldKey := stripe.GetBackend(stripe.APIBackend), stripe.Key
	oldSecret, oldPrice, oldPromos := setting.StripeApiSecret, setting.StripePriceId, setting.StripePromotionCodesEnabled
	t.Cleanup(func() {
		stripe.SetBackend(stripe.APIBackend, oldBackend)
		stripe.Key = oldKey
		setting.StripeApiSecret = oldSecret
		setting.StripePriceId = oldPrice
		setting.StripePromotionCodesEnabled = oldPromos
	})
	setting.StripeApiSecret = "sk_test_local"
	setting.StripePriceId = "price_test"
	setting.StripePromotionCodesEnabled = true
	checked := false
	transport := rechargeDiscountTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"id":"price_test","currency":"usd","product":"prod_test"}`
		if r.URL.Path == "/v1/checkout/sessions" {
			require.NoError(t, r.ParseForm())
			require.Equal(t, "9700", r.Form.Get("line_items[0][price_data][unit_amount]"))
			require.Equal(t, "1", r.Form.Get("line_items[0][quantity]"))
			require.Equal(t, "prod_test", r.Form.Get("line_items[0][price_data][product]"))
			require.Equal(t, "true", r.Form.Get("allow_promotion_codes"))
			body = `{"id":"cs_test","url":"https://checkout.invalid/stripe"}`
			checked = true
		} else {
			require.Equal(t, "/v1/prices/price_test", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	stripe.SetBackend(stripe.APIBackend, stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{HTTPClient: &http.Client{Transport: transport}}))
	url, err := genStripeLink("ref_test", "", "test@example.invalid", 97, "https://return.invalid/success", "https://return.invalid/cancel")
	require.NoError(t, err)
	require.Equal(t, "https://checkout.invalid/stripe", url)
	require.True(t, checked)
}
