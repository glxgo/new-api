package perfmetrics

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/types"
)

type FailureSource string

const (
	FailureSourceNone     FailureSource = ""
	FailureSourceClient   FailureSource = "client"
	FailureSourceUser     FailureSource = "user"
	FailureSourceSession  FailureSource = "session"
	FailureSourceChannel  FailureSource = "channel"
	FailureSourceUpstream FailureSource = "upstream"
	FailureSourceService  FailureSource = "service"
)

// ClassifyRelayFailure separates the party that triggered or owns a failed
// request. The classification is intentionally based on stable codes/statuses;
// upstream messages may contain user content and are never used as classifiers.
func ClassifyRelayFailure(apiError *types.NewAPIError, clientCanceled bool) FailureSource {
	if clientCanceled {
		return FailureSourceClient
	}
	if apiError == nil {
		return FailureSourceNone
	}

	code := strings.ToLower(strings.TrimSpace(string(apiError.GetErrorCode())))
	if strings.HasPrefix(code, "channel:") {
		return FailureSourceChannel
	}
	if strings.HasPrefix(code, string(types.ErrorCodeUpstreamResponseIncomplete)+":") {
		reason := strings.TrimPrefix(code, string(types.ErrorCodeUpstreamResponseIncomplete)+":")
		switch reason {
		case "max_output_tokens", "content_filter":
			return FailureSourceUser
		default:
			return FailureSourceUpstream
		}
	}
	switch types.ErrorCode(code) {
	case types.ErrorCodeInvalidRequest,
		types.ErrorCodeSensitiveWordsDetected,
		types.ErrorCodeReadRequestBodyFailed,
		types.ErrorCodeConvertRequestFailed,
		types.ErrorCodeAccessDenied,
		types.ErrorCodeBadRequestBody,
		types.ErrorCodePromptBlocked,
		types.ErrorCodeInsufficientUserQuota,
		types.ErrorCodePreConsumeTokenQuotaFailed:
		return FailureSourceUser
	}
	switch code {
	case "invalid_prompt", "invalid_request_error", "context_length_exceeded",
		"input_too_long", "content_policy_violation", "content_filter",
		"max_output_tokens":
		return FailureSourceUser
	case "invalid_encrypted_content":
		// Encrypted Responses state can become unusable after a stale client
		// replay or an affinity-breaking move between upstream accounts. It is
		// a conversation-state incompatibility, not a model availability signal.
		return FailureSourceSession
	}
	if strings.HasPrefix(code, "upstream:") {
		return FailureSourceUpstream
	}
	switch apiError.GetErrorType() {
	case types.ErrorTypeOpenAIError,
		types.ErrorTypeClaudeError,
		types.ErrorTypeGeminiError,
		types.ErrorTypeUpstreamError:
		return FailureSourceUpstream
	}
	switch apiError.StatusCode {
	case http.StatusBadRequest,
		http.StatusRequestEntityTooLarge,
		http.StatusUnprocessableEntity,
		499:
		return FailureSourceUser
	default:
		return FailureSourceService
	}
}

// ShouldRecordRelayFailure reports whether a final relay failure reflects the
// availability of this service or one of its channels. Client disconnects,
// malformed requests and policy-blocked prompts are not model-health signals.
func ShouldRecordRelayFailure(apiError *types.NewAPIError, clientCanceled bool) bool {
	source := ClassifyRelayFailure(apiError, clientCanceled)
	return source == FailureSourceChannel ||
		source == FailureSourceUpstream ||
		source == FailureSourceService
}
