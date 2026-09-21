package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestLoadSameChannelRetryCandidateReloadsCompleteChannel(t *testing.T) {
	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalRedisEnabled := common.RedisEnabled
	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.RedisEnabled = originalRedisEnabled
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}))
	model.DB = db
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false

	baseURL := "http://127.0.0.1:18097"
	require.NoError(t, db.Create(&model.Channel{
		Id:      55,
		Type:    1,
		Name:    "cpa2",
		Key:     "channel-secret",
		BaseURL: &baseURL,
	}).Error)

	simplified := &model.Channel{Id: 55, Type: 1, Name: "cpa2", ConcurrencyLimit: 8, RPMLimit: 12}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAIResponses, OriginModelName: "gpt-test"}
	complete, lease, newAPIError := getSameChannelRetryWithCapacity(ctx, info, simplified)
	if lease != nil {
		defer lease.Release()
	}
	require.Nil(t, newAPIError)
	require.NotNil(t, complete)
	require.Equal(t, "channel-secret", complete.Key)
	require.Equal(t, baseURL, complete.GetBaseURL())
	require.Equal(t, baseURL, common.GetContextKeyString(ctx, constant.ContextKeyChannelBaseUrl))
}

func TestShouldRetry_SkipRetryOverridesChannelError(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	apiErr := types.NewErrorWithStatusCode(
		errors.New("partial responses stream"),
		types.ErrorCodeChannelIncompleteStream,
		http.StatusBadGateway,
		types.ErrOptionWithSkipRetry(),
	)

	require.False(t, shouldRetry(c, apiErr, 3))
}

func TestShouldRetry_ClientCancellationStopsRetry(t *testing.T) {
	requestContext, cancel := context.WithCancel(context.Background())
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(requestContext)
	cancel()
	apiErr := types.NewErrorWithStatusCode(
		errors.New("upstream request canceled"),
		types.ErrorCodeChannelIncompleteStream,
		http.StatusBadGateway,
	)

	require.False(t, shouldRetry(c, apiErr, 3))
}

func TestShouldRetry_InvalidResponsesEncryptedContentStopsRetry(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	apiErr := types.NewErrorWithStatusCode(
		errors.New(`Request failed with status 400: code=invalid_encrypted_content`),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadRequest,
	)

	require.False(t, shouldRetry(c, apiErr, 3))
}

func TestShouldRetry_ResponsesTransientBeforeOutput(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	apiErr := types.WithOpenAIError(types.OpenAIError{
		Type: "server_error", Code: "server_is_overloaded", Message: "overloaded",
	}, http.StatusBadGateway)

	require.True(t, shouldRetry(c, apiErr, 3))
}

func TestShouldRetry_ResponsesAfterOutputDoesNotReplay(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	apiErr := types.NewErrorWithStatusCode(
		errors.New("responses stream failed after output"),
		types.ErrorCodeChannelIncompleteStream,
		http.StatusBadGateway,
		types.ErrOptionWithSkipRetry(),
	)

	require.False(t, shouldRetry(c, apiErr, 3))
}

func TestCanRetrySameResponsesChannel_TransientBeforeOutput(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		RelayFormat:                  types.RelayFormatOpenAIResponses,
		ForwardedResponsesEventCount: 0,
	}
	apiErr := types.WithOpenAIError(types.OpenAIError{
		Type: "server_error", Code: "server_is_overloaded", Message: "overloaded",
	}, http.StatusBadGateway)

	require.True(t, canRetrySameResponsesChannel(c, info, apiErr))
}

func TestShouldUseSameChannelRetryAllowsOnlyOneAttempt(t *testing.T) {
	channel := &model.Channel{Id: 55}
	require.True(t, shouldUseSameChannelRetry(channel, 0))
	require.False(t, shouldUseSameChannelRetry(channel, 1))
	require.False(t, shouldUseSameChannelRetry(nil, 0))
}

func TestCanRetrySameResponsesChannel_RejectsUnsafeReplay(t *testing.T) {
	tests := []struct {
		name string
		info *relaycommon.RelayInfo
		err  *types.NewAPIError
	}{
		{
			name: "typed event already forwarded",
			info: &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAIResponses, ForwardedResponsesEventCount: 1},
			err:  types.NewErrorWithStatusCode(errors.New("overloaded"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway),
		},
		{
			name: "deterministic request error",
			info: &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAIResponses},
			err:  types.NewErrorWithStatusCode(errors.New("bad request"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest),
		},
		{
			name: "skip retry override",
			info: &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAIResponses},
			err: types.NewErrorWithStatusCode(
				errors.New("partial stream"),
				types.ErrorCodeChannelIncompleteStream,
				http.StatusBadGateway,
				types.ErrOptionWithSkipRetry(),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			require.False(t, canRetrySameResponsesChannel(c, tt.info, tt.err))
		})
	}
}
