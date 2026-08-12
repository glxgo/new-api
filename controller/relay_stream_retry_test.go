package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

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
