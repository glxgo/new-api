package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicErrorResponsesStream(t *testing.T) {
	for _, terminal := range []string{
		`{"type":"error","code":"invalid_request_error","message":"bad input: https://upstream.private.example/v1/responses","param":"https://upstream.private.example/input"}`,
		`{"type":"response.failed","response":{"id":"resp_test","error":{"type":"invalid_request_error","message":"https://upstream.private.example/v1/responses","metadata":{"url":"https://upstream.private.example/debug"}}}}`,
	} {
		for _, codex := range []bool{false, true} {
			c, info, resp, recorder := newResponsesStreamTest(t, "data: "+terminal+"\n\n")
			if codex {
				c.Request.Header.Set("User-Agent", "codex_cli_rs/0.100.0")
			}
			_, apiErr := OaiResponsesStreamHandler(c, info, resp)
			require.NotNil(t, apiErr)
			require.True(t, types.IsSkipRetryError(apiErr))
			require.Contains(t, apiErr.Error(), "upstream.private.example", "internal diagnostic must remain available")
			require.NotEmpty(t, recorder.Body.String())
			require.NotContains(t, recorder.Body.String(), "upstream.private.example")
			require.Contains(t, recorder.Body.String(), "[redacted]")
		}
	}
}

func TestPublicErrorImageStream(t *testing.T) {
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
	body := strings.Join([]string{
		`data: {"type":"image_generation.partial_image","url":"https://images.example/result.png"}`,
		"", `data: {"type":"error","message":"https://upstream.private.example/image","error":{"message":"https://upstream.private.example/image"}}`,
		"", "data: [DONE]", "", "",
	}, "\n")
	c, recorder, resp, info := newImageTestContext(t, body, "text/event-stream", true)
	_, _ = OpenaiImageStreamHandler(c, info, resp)
	require.NotContains(t, recorder.Body.String(), "upstream.private.example")
	require.Contains(t, recorder.Body.String(), "https://images.example/result.png")
	// A failed stream must not be terminated with a successful [DONE] marker.
	require.NotContains(t, recorder.Body.String(), "data: [DONE]")
}

func TestPublicErrorTTSNonStream(t *testing.T) {
	for _, body := range []string{`{"error":{"message":"502, url: https://private.example/audio"}}`, `<html>https://private.example/gateway</html>`, "\x00\x01\xffhttps://audio.example/success"} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest("POST", "/v1/audio/speech", nil)
		info := &relaycommon.RelayInfo{Request: &dto.AudioRequest{ResponseFormat: "pcm"}}
		OpenaiTTSHandler(c, &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"audio/pcm"}}, Body: io.NopCloser(strings.NewReader(body))}, info)
		require.NotContains(t, recorder.Body.String(), "private.example")
		if strings.HasPrefix(body, "{") {
			require.Contains(t, recorder.Body.String(), `"message":"502"`)
		} else if strings.HasPrefix(body, "<") {
			require.Contains(t, recorder.Body.String(), "Upstream request failed")
		} else {
			require.Equal(t, body, recorder.Body.String())
		}
	}
}
