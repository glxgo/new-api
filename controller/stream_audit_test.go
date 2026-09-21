package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestStreamAuditRetryBoundary(t *testing.T) {
	for _, output := range []string{"none", "heartbeat", "event"} {
		t.Run(output, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			info := &relaycommon.RelayInfo{RelayFormat: types.RelayFormatOpenAIResponses}
			switch output {
			case "heartbeat":
				helper.SetEventStreamHeaders(c)
				require.NoError(t, helper.PingData(c))
			case "event":
				require.NoError(t, helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: "response.created"}, `{"type":"response.created"}`))
			}
			apiErr := types.NewErrorWithStatusCode(errors.New("read failed"), types.ErrorCodeChannelIncompleteStream, http.StatusBadGateway)
			require.Equal(t, output != "event", shouldRetry(c, apiErr, 2))
			require.Equal(t, output != "event", canRetrySameResponsesChannel(c, info, apiErr))
		})
	}
}
