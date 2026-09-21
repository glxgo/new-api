package relay_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/cloudflare"
	"github.com/QuantumNous/new-api/relay/channel/cohere"
	"github.com/QuantumNous/new-api/relay/channel/coze"
	"github.com/QuantumNous/new-api/relay/channel/dify"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	"github.com/QuantumNous/new-api/relay/channel/ollama"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	"github.com/QuantumNous/new-api/relay/channel/palm"
	"github.com/QuantumNous/new-api/relay/channel/tencent"
	"github.com/QuantumNous/new-api/relay/channel/xai"
	"github.com/QuantumNous/new-api/relay/channel/zhipu"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type failedStreamReader struct{}

func (failedStreamReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

type auditNotifyRecorder struct{ *httptest.ResponseRecorder }

func (auditNotifyRecorder) CloseNotify() <-chan bool { return make(chan bool) }

func TestStreamAuditLegacyAdaptersPropagateTransportFailure(t *testing.T) {
	for name, handle := range map[string]func(*gin.Context, *http.Response, *relaycommon.RelayInfo) (any, *types.NewAPIError){
		"tencent":    (&tencent.Adaptor{}).DoResponse,
		"cloudflare": (&cloudflare.Adaptor{}).DoResponse,
		"cohere":     (&cohere.Adaptor{}).DoResponse,
		"zhipu":      (&zhipu.Adaptor{}).DoResponse,
		"ollama":     (&ollama.Adaptor{}).DoResponse,
		"coze":       (&coze.Adaptor{}).DoResponse,
		"palm":       (&palm.Adaptor{}).DoResponse,
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(auditNotifyRecorder{w})
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			info := &relaycommon.RelayInfo{IsStream: true, RelayMode: relayconstant.RelayModeChatCompletions, RelayFormat: types.RelayFormatOpenAI, ChannelMeta: &relaycommon.ChannelMeta{}}
			_, err := handle(c, &http.Response{StatusCode: 200, Body: io.NopCloser(failedStreamReader{})}, info)
			require.NotNil(t, err)
			require.Equal(t, http.StatusBadGateway, err.StatusCode)
			require.NotContains(t, w.Body.String(), "[DONE]")
		})
	}
}

func TestStreamAuditLegacySuccessfulTerminals(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		handle     func(*gin.Context, *http.Response, *relaycommon.RelayInfo) (any, *types.NewAPIError)
	}{
		{"tencent", "data: {\"Choices\":[{\"Delta\":{\"Content\":\"hello\"},\"FinishReason\":\"stop\"}]}\n\n", (&tencent.Adaptor{}).DoResponse},
		{"cloudflare", "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", (&cloudflare.Adaptor{}).DoResponse},
		{"cohere", "{\"text\":\"hello\",\"is_finished\":false}\n{\"is_finished\":true,\"finish_reason\":\"COMPLETE\",\"response\":{\"meta\":{\"billed_units\":{\"input_tokens\":2,\"output_tokens\":1}}}}\n", (&cohere.Adaptor{}).DoResponse},
		{"zhipu", "data: hello\nmeta: {\"request_id\":\"audit\",\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":1,\"total_tokens\":3}}\n", (&zhipu.Adaptor{}).DoResponse},
		{"ollama", "{\"message\":{\"role\":\"assistant\",\"content\":\"hello\"},\"done\":false}\n{\"done\":true,\"prompt_eval_count\":2,\"eval_count\":1}\n", (&ollama.Adaptor{}).DoResponse},
		{"coze", "event: conversation.message.delta\ndata: {\"content\":\"hello\"}\n\nevent: conversation.chat.completed\ndata: {\"usage\":{\"input_count\":2,\"output_count\":1,\"token_count\":3}}\n\n", (&coze.Adaptor{}).DoResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			info := &relaycommon.RelayInfo{IsStream: true, RelayMode: relayconstant.RelayModeChatCompletions, RelayFormat: types.RelayFormatOpenAI, ChannelMeta: &relaycommon.ChannelMeta{}}
			usage, err := tc.handle(c, &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.body))}, info)
			require.Nil(t, err)
			require.NotNil(t, usage)
			require.Contains(t, w.Body.String(), "hello")
			require.Equal(t, 1, strings.Count(w.Body.String(), "data: [DONE]"))
		})
	}
}

func TestStreamAuditAdaptersPropagateTransportFailure(t *testing.T) {
	for _, tc := range []struct {
		name   string
		handle func(*gin.Context, *relaycommon.RelayInfo, *http.Response) (*dto.Usage, *types.NewAPIError)
	}{
		{"chat", openai.OaiStreamHandler},
		{"claude", func(c *gin.Context, i *relaycommon.RelayInfo, r *http.Response) (*dto.Usage, *types.NewAPIError) {
			return claude.ClaudeStreamHandler(c, r, i)
		}},
		{"gemini", gemini.GeminiChatStreamHandler},
		{"image", openai.OpenaiImageStreamHandler},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			i := &relaycommon.RelayInfo{IsStream: true, RelayFormat: types.RelayFormatOpenAI, ChannelMeta: &relaycommon.ChannelMeta{}}
			r := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(failedStreamReader{})}
			_, err := tc.handle(c, i, r)
			require.NotNil(t, err)
			require.Equal(t, http.StatusBadGateway, err.StatusCode)
			require.NotContains(t, w.Body.String(), "[DONE]")
		})
	}
}

type finalFailureWriter struct{ *httptest.ResponseRecorder }

func (w *finalFailureWriter) Write(p []byte) (int, error) {
	if strings.Contains(string(p), "[DONE]") {
		return 0, io.ErrClosedPipe
	}
	return w.ResponseRecorder.Write(p)
}
func TestStreamAuditFinalWriteFailure(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		handle     func(*gin.Context, *http.Response, *relaycommon.RelayInfo) (any, *types.NewAPIError)
	}{
		{"xai", "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", (&xai.Adaptor{}).DoResponse},
		{"dify", "data: {\"event\":\"message\",\"answer\":\"hello\"}\n\ndata: {\"event\":\"message_end\"}\n\n", (&dify.Adaptor{}).DoResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(&finalFailureWriter{httptest.NewRecorder()})
			c.Request = httptest.NewRequest("POST", "/", nil)
			info := &relaycommon.RelayInfo{IsStream: true, RelayMode: relayconstant.RelayModeChatCompletions, ChannelMeta: &relaycommon.ChannelMeta{}}
			_, err := tc.handle(c, &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.body))}, info)
			require.NotNil(t, err)
			require.Equal(t, 499, err.StatusCode)
		})
	}
}
