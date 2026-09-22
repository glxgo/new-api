package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// Membership, labels, price and poll index are intentionally excluded; actual
// credentials, endpoints, conversions and overrides invalidate current work.
func CapabilityConfigurationHash(ch *model.Channel, profile model.CapabilityProfile, secret []byte) string {
	data, _ := common.Marshal([]any{ch.Id, ch.Type, ch.Key, ch.BaseURL, ch.Other, ch.ModelMapping, ch.ParamOverride, ch.HeaderOverride, ch.Setting, ch.OtherSettings, ch.OpenAIOrganization, profile})
	h := hmac.New(sha256.New, secret)
	_, _ = h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}
