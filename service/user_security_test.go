package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestIsCyberPolicyError(t *testing.T) {
	require.True(t, IsCyberPolicyError(types.WithOpenAIError(types.OpenAIError{
		Code:    "cyber_policy",
		Message: "blocked",
	}, http.StatusBadGateway)))
	require.True(t, IsCyberPolicyError(types.NewOpenAIError(
		errors.New("This content was flagged for possible cybersecurity risk."),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadGateway,
	)))
	require.False(t, IsCyberPolicyError(types.NewOpenAIError(
		errors.New("ordinary upstream failure"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadGateway,
	)))
}
