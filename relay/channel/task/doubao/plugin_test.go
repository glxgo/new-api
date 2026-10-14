package doubao

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestDoubaoMoejsPayloadAndTerminalStatuses(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/video/generations", strings.NewReader(`{"model":"alias","prompt":"test","metadata":{"model":"forged","resolution":"720p","watermark":false,"content":[{"type":"video_url","video_url":{"url":"https://example.com/ref.mp4"}}]}}`))
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{OriginModelName: "alias", TaskRelayInfo: &relaycommon.TaskRelayInfo{}, ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "doubao-seedance-2-0-fast-260128"}}
	a := &TaskAdaptor{}
	require.Nil(t, a.ValidateRequestAndSetAction(c, info))
	require.Nil(t, a.ValidateEffectiveRequest(c, info))
	raw, err := a.BuildRequestBody(c, info)
	require.NoError(t, err)
	data, err := io.ReadAll(raw)
	require.NoError(t, err)
	require.Equal(t, info.UpstreamModelName, gjson.GetBytes(data, "model").String())
	require.True(t, gjson.GetBytes(data, "watermark").Exists())
	require.False(t, gjson.GetBytes(data, "watermark").Bool())
	require.InDelta(t, 22.0/37, a.EstimateBilling(c, info)["video_input_ratio"], 0.00001)
	for _, status := range []string{"expired", "cancelled", "failed"} {
		body, _ := common.Marshal(map[string]any{"status": status})
		result, err := a.ParseTaskResult(body)
		require.NoError(t, err)
		require.Equal(t, "FAILURE", result.Status)
	}
	info.UpstreamModelName = "doubao-seedance-2-0-fast-260128"
	// A different mapped model's unsupported resolution is rejected before reserve.
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Request = httptest.NewRequest("POST", "/v1/video/generations", strings.NewReader(`{"model":"alias","prompt":"test","metadata":{"resolution":"4k"}}`))
	c2.Request.Header.Set("Content-Type", "application/json")
	require.Nil(t, a.ValidateRequestAndSetAction(c2, info))
	require.NotNil(t, a.ValidateEffectiveRequest(c2, info))
}
