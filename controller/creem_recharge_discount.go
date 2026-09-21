package controller

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/shopspring/decimal"
	"github.com/thanhpk/randstr"
)

// Provider-supported single-use discounts keep the fixed product's quota intact.
// https://docs.creem.io/features/discounts
func createCreemRechargeDiscount(ctx context.Context, reference, product string, rate float64) (string, error) {
	code := "LOYAL" + randstr.String(24)
	payload, err := common.Marshal(map[string]any{
		"name": "Recharge " + reference, "code": code, "type": "percentage",
		"percentage": int(math.Round((1 - rate) * 100)), "duration": "once",
		"applies_to_products": []string{product}, "max_redemptions": 1,
		"expiry_date": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
	})
	if err != nil {
		return "", err
	}
	endpoint := "https://api.creem.io/v1/discounts"
	if setting.CreemTestMode {
		endpoint = "https://test-api.creem.io/v1/discounts"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", setting.CreemApiKey)
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("Creem discount status %d", resp.StatusCode)
	}
	return code, nil
}

func discountedCreemProducts(totalCents int64) []CreemProduct {
	var products []CreemProduct
	if err := common.UnmarshalJsonStr(setting.CreemProducts, &products); err != nil {
		return []CreemProduct{}
	}
	for i := range products {
		products[i].Price = decimal.NewFromFloat(model.ApplyRechargeDiscount(products[i].Price, totalCents)).Round(2).InexactFloat64()
	}
	return products
}
