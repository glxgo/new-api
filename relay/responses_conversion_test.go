package relay

import (
	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"
	kittypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestConvertedResponsesSSETerminalAndEOF(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		t.Run(map[bool]string{false: "truncated", true: "complete"}[terminal], func(t *testing.T) {
			body := "data: {\"id\":\"chat-1\",\"model\":\"test\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hello\"}}]}\n\n"
			if terminal {
				body += "data: {\"id\":\"chat-1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n\ndata: [DONE]\n\n"
			}
			writer := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(writer)
			c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
			info := &relaycommon.RelayInfo{OriginModelName: "test", IsStream: true, StartTime: time.Now(), ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "test"}}
			response := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}
			usage, err := convertedResponsesResult(c, info, &convmeta.Values{}, kittypes.RelayFormatOpenAI, response)
			for _, line := range strings.Split(writer.Body.String(), "\n") {
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				var event map[string]any
				require.NoError(t, common.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event))
				require.NotEmpty(t, event["type"], "SSE data must be the Responses payload, not an internal wrapper")
				require.NotContains(t, event, "Payload")
			}
			if terminal {
				require.Nil(t, err)
				require.Equal(t, 5, usage.TotalTokens)
				require.Equal(t, 1, strings.Count(writer.Body.String(), "event: response.completed"))
			} else {
				require.NotNil(t, err)
				require.NotContains(t, writer.Body.String(), "response.completed")
				require.Contains(t, writer.Body.String(), "response.failed")
			}
		})
	}
}
