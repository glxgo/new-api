package common

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/types"
)

func (info *RelayInfo) ObserveKiroPayload(data []byte, streaming bool) []byte {
	if info == nil || info.Kiro == nil {
		return data
	}
	return info.Kiro.Observe(data, streaming)
}

func (info *RelayInfo) KiroCompletionError() *types.NewAPIError {
	if info == nil || !info.Kiro.Charge() {
		return nil
	}
	r := info.Kiro.Result()
	if !r.Trusted || !r.Complete || r.Error != "" || r.Receipt == nil || r.Receipt.Status != "complete" {
		return types.NewErrorWithStatusCode(errors.New("upstream Kiro metering unavailable or invalid"), "kiro_metering_invalid", http.StatusBadGateway, types.ErrOptionWithSkipRetry())
	}
	return nil
}
