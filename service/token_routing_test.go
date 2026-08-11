package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTokenRouteCircuitSkipsFailingGroupAndClearsOnSuccess(t *testing.T) {
	previousRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedis })
	localRouteCircuit.Lock()
	localRouteCircuit.failures = map[string][]time.Time{}
	localRouteCircuit.open = map[string]time.Time{}
	localRouteCircuit.quota = map[string]time.Time{}
	localRouteCircuit.Unlock()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("token_route_configured", true)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "wallet-plus")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-test")
	common.SetContextKey(c, constant.ContextKeyTokenId, 7)
	common.SetContextKey(c, constant.ContextKeyTokenRouteStepId, 9)
	apiErr := types.NewErrorWithStatusCode(
		errors.New("temporary upstream failure"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadGateway,
	)
	for range routeFailureLimit {
		RecordTokenRouteFailure(c, apiErr)
	}
	relayFormat := model.PathToRelayFormat(c.Request.URL.Path)
	require.True(t, routeCircuitOpen("wallet-plus", "gpt-test", relayFormat))
	MarkCurrentTokenRouteQuotaUnavailable(c)
	require.True(t, tokenRouteQuotaCooling(7, 9))

	RecordTokenRouteSuccess(c)
	require.False(t, routeCircuitOpen("wallet-plus", "gpt-test", relayFormat))
	require.False(t, tokenRouteQuotaCooling(7, 9))
}
