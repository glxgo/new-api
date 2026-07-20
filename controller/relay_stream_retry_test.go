package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

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
