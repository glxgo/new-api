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
	MarkCurrentTokenRouteQuotaUnavailable(c, 1)
	require.True(t, tokenRouteQuotaCooling(7, 9))

	RecordTokenRouteSuccess(c)
	require.False(t, routeCircuitOpen("wallet-plus", "gpt-test", relayFormat))
	require.False(t, tokenRouteQuotaCooling(7, 9))
}

func TestWalletRouteMovesToBackAfterThreeUnavailableResults(t *testing.T) {
	previousRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = previousRedis })
	resetLocalTokenRouteRuntimeForTest()

	steps := []model.TokenRouteStep{
		{Id: 1, TokenId: 7, Position: 1, GroupName: "wallet-a", FundingSource: model.TokenRouteSourceWallet},
		{Id: 2, TokenId: 7, Position: 2, GroupName: "member", FundingSource: model.TokenRouteSourceVirtualMembership},
		{Id: 3, TokenId: 7, Position: 3, GroupName: "wallet-b", FundingSource: model.TokenRouteSourceWallet},
		{Id: 4, TokenId: 7, Position: 4, GroupName: "wallet-c", FundingSource: model.TokenRouteSourceWallet},
	}
	format := model.PathToRelayFormat("/v1/chat/completions")
	require.Equal(t, []string{"wallet-a", "member", "wallet-b", "wallet-c"}, routeStepGroups(orderTokenRouteSteps(7, "gpt-test", format, steps)))

	for i := 0; i < routeFailureLimit-1; i++ {
		require.False(t, recordWalletRouteUnavailable(7, 1, "gpt-test", format))
	}
	require.Equal(t, []string{"wallet-a", "member", "wallet-b", "wallet-c"}, routeStepGroups(orderTokenRouteSteps(7, "gpt-test", format, steps)))
	require.True(t, recordWalletRouteUnavailable(7, 1, "gpt-test", format))
	require.Equal(t, []string{"wallet-b", "member", "wallet-c", "wallet-a"}, routeStepGroups(orderTokenRouteSteps(7, "gpt-test", format, steps)))

	for i := 0; i < routeFailureLimit; i++ {
		recordWalletRouteUnavailable(7, 3, "gpt-test", format)
	}
	require.Equal(t, []string{"wallet-c", "member", "wallet-a", "wallet-b"}, routeStepGroups(orderTokenRouteSteps(7, "gpt-test", format, steps)))
}

func routeStepGroups(steps []model.TokenRouteStep) []string {
	groups := make([]string, 0, len(steps))
	for _, step := range steps {
		groups = append(groups, step.GroupName)
	}
	return groups
}
