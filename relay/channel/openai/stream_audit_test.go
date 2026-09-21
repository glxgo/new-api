package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestStreamAuditRealHTTPTruncationEmitsFailedTerminal(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_wire\"}}\n\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n"))
		w.(http.Flusher).Flush()
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}))
	defer upstream.Close()
	_, info, _, _ := newResponsesStreamTest(t, "")
	info.RelayFormat = types.RelayFormatOpenAIResponses
	result := make(chan *types.NewAPIError, 1)
	router := gin.New()
	router.POST("/v1/responses", func(c *gin.Context) {
		resp, err := http.Get(upstream.URL)
		if err != nil {
			c.Status(500)
			return
		}
		_, apiErr := OaiResponsesStreamHandler(c, info, resp)
		result <- apiErr
	})
	downstream := httptest.NewServer(router)
	defer downstream.Close()
	resp, err := http.Post(downstream.URL+"/v1/responses", "application/json", strings.NewReader("{}"))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.Contains(t, string(body), "hello")
	require.Equal(t, 1, strings.Count(string(body), "event: response.failed\n"))
	require.Contains(t, string(body), `"id":"resp_wire"`)
	require.NotContains(t, string(body), "[DONE]")
	apiErr := <-result
	require.NotNil(t, apiErr)
	require.True(t, types.IsSkipRetryError(apiErr))
}

type auditReadError struct{ err error }

func (r auditReadError) Read([]byte) (int, error) { return 0, r.err }

func TestStreamAuditResponsesFailureMatrix(t *testing.T) {
	for _, tc := range []struct {
		name, tail string
		readErr    error
		success    bool
	}{
		{"clean_complete", `data: {"type":"response.completed","response":{"status":"completed"}}` + "\n\n", nil, true},
		{"premature_eof", "", nil, false},
		{"read_error", "", io.ErrUnexpectedEOF, false},
		{"invalid_json", "data: {broken}\n\ndata: {\"type\":\"response.completed\"}\n\n", nil, false},
		{"done_failed_status", "data: {\"type\":\"response.done\",\"response\":{\"status\":\"failed\",\"error\":{\"code\":\"stream_read_error\",\"message\":\"read failed\"}}}\n\n", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_matrix\"}}\n\n" + tc.tail
			c, info, resp, w := newResponsesStreamTest(t, body)
			info.RelayFormat = types.RelayFormatOpenAIResponses
			if tc.readErr != nil {
				resp.Body = io.NopCloser(io.MultiReader(strings.NewReader(body), auditReadError{tc.readErr}))
			}
			_, apiErr := OaiResponsesStreamHandler(c, info, resp)
			if tc.success {
				require.Nil(t, apiErr)
				return
			}
			require.NotNil(t, apiErr)
			require.True(t, types.IsSkipRetryError(apiErr))
			require.Contains(t, w.Body.String(), "event: response.failed\n")
			require.NotContains(t, w.Body.String(), "event: response.completed\n")
			require.True(t, common.GetContextKeyBool(c, constant.ContextKeyRelayErrorAlreadyStreamed))
		})
	}
}

func TestStreamAuditResponsesMultilineEvent(t *testing.T) {
	c, info, resp, w := newResponsesStreamTest(t, "\ufeffevent: response.completed\r\ndata: {\r\ndata: \"type\":\"response.completed\",\r\ndata: \"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":2,\"output_tokens\":1,\"total_tokens\":3}}}\r\n\r\n")
	info.RelayFormat = types.RelayFormatOpenAIResponses
	usage, apiErr := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.Equal(t, 3, usage.TotalTokens)
	require.Contains(t, w.Body.String(), "event: response.completed\n")
}

func TestStreamAuditChatTransportFailuresDoNotFinishSuccessfully(t *testing.T) {
	for _, body := range []string{"", "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"}}]}\n\n"} {
		t.Run(body, func(t *testing.T) {
			c, info, resp, w := newResponsesStreamTest(t, body)
			info.RelayFormat = types.RelayFormatOpenAI
			info.RelayMode = relayconstant.RelayModeChatCompletions
			resp.Body = io.NopCloser(io.MultiReader(strings.NewReader(body), auditReadError{io.ErrUnexpectedEOF}))
			_, apiErr := OaiStreamHandler(c, info, resp)
			require.NotNil(t, apiErr)
			require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
			require.NotContains(t, w.Body.String(), "[DONE]")
		})
	}
}

func TestStreamAuditChatDoesNotHideErrorOrTruncation(t *testing.T) {
	for _, body := range []string{
		"data: {\"error\":{\"code\":\"stream_read_error\",\"message\":\"read failed\"}}\n\ndata: [DONE]\n\n",
		"data: {broken}\n\ndata: [DONE]\n\n",
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"}}]}\n\n",
	} {
		t.Run(body, func(t *testing.T) {
			c, info, resp, w := newResponsesStreamTest(t, body)
			info.RelayMode = relayconstant.RelayModeChatCompletions
			info.RelayFormat = types.RelayFormatOpenAI
			_, err := OaiStreamHandler(c, info, resp)
			require.NotNil(t, err)
			require.NotContains(t, w.Body.String(), "[DONE]")
		})
	}
}

func TestStreamAuditResponsesToChatPreservesTerminalError(t *testing.T) {
	for _, prefix := range []string{"", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n"} {
		t.Run(prefix, func(t *testing.T) {
			c, info, resp, w := newResponsesStreamTest(t, prefix+"data: {\"type\":\"error\",\"code\":\"stream_read_error\",\"message\":\"upstream read failed\"}\n\n")
			info.RelayFormat = types.RelayFormatOpenAI
			_, apiErr := OaiResponsesToChatStreamHandler(c, info, resp)
			require.NotNil(t, apiErr)
			require.Equal(t, types.ErrorCode("stream_read_error"), apiErr.GetErrorCode())
			if prefix != "" {
				require.Contains(t, w.Body.String(), `"code":"stream_read_error"`)
				require.True(t, types.IsSkipRetryError(apiErr))
				require.True(t, common.GetContextKeyBool(c, constant.ContextKeyRelayErrorAlreadyStreamed))
			} else {
				require.Empty(t, w.Body.String())
			}
			require.NotContains(t, w.Body.String(), "[DONE]")
		})
	}
}

func TestStreamAuditCodexNativeFailedTerminalIsForwarded(t *testing.T) {
	c, info, resp, recorder := newResponsesStreamTest(t,
		"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_audit\"}}\n\n"+
			"data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_audit\",\"status\":\"failed\",\"error\":{\"code\":\"stream_read_error\",\"message\":\"upstream read failed\"}}}\n\n")
	info.RelayFormat = types.RelayFormatOpenAIResponses
	c.Request.Header.Set("User-Agent", "codex-tui/1.0")
	_, apiErr := OaiResponsesStreamHandler(c, info, resp)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCode("stream_read_error"), apiErr.GetErrorCode())
	require.Equal(t, 1, strings.Count(recorder.Body.String(), "event: response.failed\n"))
	require.True(t, types.IsSkipRetryError(apiErr))
	require.True(t, common.GetContextKeyBool(c, constant.ContextKeyRelayErrorAlreadyStreamed))
}

func TestStreamAuditImageRejectsMalformedOrUnfinishedOutput(t *testing.T) {
	for _, tc := range []struct {
		body    string
		success bool
	}{
		{"data: {broken}\n\ndata: [DONE]\n\n", false},
		{"data: {\"type\":\"image_generation.partial_image\",\"b64_json\":\"part\"}\n\n", false},
		{"data: {\"type\":\"image_generation.completed\",\"b64_json\":\"full\"}\n\n", true},
	} {
		c, w, resp, info := newImageTestContext(t, tc.body, "text/event-stream", true)
		_, err := OpenaiImageStreamHandler(c, info, resp)
		if tc.success {
			require.Nil(t, err)
		} else {
			require.NotNil(t, err)
			require.NotContains(t, w.Body.String(), "[DONE]")
		}
	}
}

func TestStreamAuditSSEEventNameCannotHideFailedStatus(t *testing.T) {
	for _, convert := range []bool{false, true} {
		c, info, resp, _ := newResponsesStreamTest(t, "event: response.done\ndata: {\"response\":{\"id\":\"resp_header\",\"status\":\"failed\",\"error\":{\"code\":\"stream_read_error\",\"message\":\"read failed\"}}}\n\n")
		var err *types.NewAPIError
		if convert {
			_, err = OaiResponsesToChatStreamHandler(c, info, resp)
		} else {
			_, err = OaiResponsesStreamHandler(c, info, resp)
		}
		require.NotNil(t, err)
		require.Equal(t, types.ErrorCode("stream_read_error"), err.GetErrorCode())
	}
}
