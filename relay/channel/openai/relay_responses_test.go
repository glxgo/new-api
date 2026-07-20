package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newResponsesStreamTest(t *testing.T, body string) (*gin.Context, *relaycommon.RelayInfo, *http.Response) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	setting := operation_setting.GetGeneralSetting()
	oldPingEnabled := setting.PingIntervalEnabled
	oldStreamingTimeout := appconstant.StreamingTimeout
	setting.PingIntervalEnabled = false
	appconstant.StreamingTimeout = 30
	t.Cleanup(func() {
		setting.PingIntervalEnabled = oldPingEnabled
		appconstant.StreamingTimeout = oldStreamingTimeout
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	info := &relaycommon.RelayInfo{
		IsStream:    true,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	return c, info, resp
}

func TestOaiResponsesStreamHandler_CompletedEventMarksNormalEnd(t *testing.T) {
	c, info, resp := newResponsesStreamTest(t,
		"data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":3,\"output_tokens\":2,\"total_tokens\":5}}}\n\n")

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 3, usage.PromptTokens)
	require.Equal(t, 2, usage.CompletionTokens)
	require.NotNil(t, info.StreamStatus)
	require.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
}

func TestOaiResponsesStreamHandler_PrematureEOFIsIncomplete(t *testing.T) {
	c, info, resp := newResponsesStreamTest(t,
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n")

	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)

	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCode("channel:incomplete_stream"), apiErr.GetErrorCode())
	require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	require.NotNil(t, info.StreamStatus)
	require.Equal(t, relaycommon.StreamEndReasonEOF, info.StreamStatus.EndReason)
}
