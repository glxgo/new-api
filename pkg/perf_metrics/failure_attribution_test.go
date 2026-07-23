package perfmetrics

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestShouldRecordRelayFailureExcludesClientAndUserCauses(t *testing.T) {
	require.False(t, ShouldRecordRelayFailure(
		types.NewOpenAIError(errors.New("client disconnected"), types.ErrorCodeDoRequestFailed, http.StatusInternalServerError),
		true,
	))
	require.False(t, ShouldRecordRelayFailure(
		types.NewOpenAIError(errors.New("invalid prompt"), types.ErrorCodeInvalidRequest, http.StatusBadRequest),
		false,
	))
	require.False(t, ShouldRecordRelayFailure(
		types.NewOpenAIError(errors.New("content blocked"), types.ErrorCodePromptBlocked, http.StatusBadRequest),
		false,
	))
	require.False(t, ShouldRecordRelayFailure(
		types.NewOpenAIError(errors.New("payload too large"), types.ErrorCodeBadRequestBody, http.StatusRequestEntityTooLarge),
		false,
	))
}

func TestShouldRecordRelayFailureKeepsServiceAndChannelFailures(t *testing.T) {
	require.True(t, ShouldRecordRelayFailure(
		types.NewOpenAIError(errors.New("upstream unavailable"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway),
		false,
	))
	require.True(t, ShouldRecordRelayFailure(
		types.NewOpenAIError(errors.New("upstream key invalid"), types.ErrorCodeBadResponseStatusCode, http.StatusUnauthorized),
		false,
	))
	require.True(t, ShouldRecordRelayFailure(
		types.NewError(errors.New("incomplete stream"), types.ErrorCodeChannelIncompleteStream),
		false,
	))
}

func TestClassifyRelayFailure(t *testing.T) {
	tests := []struct {
		name           string
		apiErr         *types.NewAPIError
		clientCanceled bool
		want           FailureSource
	}{
		{
			name:           "client cancellation wins",
			apiErr:         types.NewError(errors.New("upstream failed"), types.ErrorCodeChannelIncompleteStream),
			clientCanceled: true,
			want:           FailureSourceClient,
		},
		{
			name:   "upstream server error",
			apiErr: types.WithOpenAIError(types.OpenAIError{Code: "server_error", Type: "server_error", Message: "failed"}, http.StatusBadGateway),
			want:   FailureSourceUpstream,
		},
		{
			name:   "explicit upstream terminal fallback",
			apiErr: types.NewError(errors.New("failed"), types.ErrorCodeUpstreamResponseFailed),
			want:   FailureSourceUpstream,
		},
		{
			name:   "incomplete max tokens belongs to request",
			apiErr: types.NewError(errors.New("incomplete"), types.ErrorCode("upstream:response_incomplete:max_output_tokens")),
			want:   FailureSourceUser,
		},
		{
			name:   "invalid prompt belongs to user",
			apiErr: types.WithOpenAIError(types.OpenAIError{Code: "invalid_prompt", Type: "invalid_request_error", Message: "bad prompt"}, http.StatusBadGateway),
			want:   FailureSourceUser,
		},
		{
			name:   "invalid encrypted content is session state",
			apiErr: types.WithOpenAIError(types.OpenAIError{Code: "invalid_encrypted_content", Type: "invalid_request_error", Message: "cannot decrypt"}, http.StatusBadGateway),
			want:   FailureSourceSession,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ClassifyRelayFailure(tt.apiErr, tt.clientCanceled))
		})
	}
}

func TestInvalidEncryptedContentDoesNotAffectModelHealth(t *testing.T) {
	apiErr := types.WithOpenAIError(types.OpenAIError{
		Code: "invalid_encrypted_content", Type: "invalid_request_error", Message: "cannot decrypt",
	}, http.StatusBadGateway)
	require.False(t, ShouldRecordRelayFailure(apiErr, false))
}
