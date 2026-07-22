package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type trackedRequestBody struct {
	reader *bytes.Reader
}

func (b *trackedRequestBody) Read(p []byte) (int, error) {
	return b.reader.Read(p)
}

func (b *trackedRequestBody) Close() error {
	return nil
}

func TestAbortWithOpenAIMessageDrainsUnreadRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	payload := bytes.Repeat([]byte("x"), 2<<20)
	body := &trackedRequestBody{reader: bytes.NewReader(payload)}
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	context.Request.Body = body
	context.Request.ContentLength = int64(len(payload))

	abortWithOpenAiMessage(
		context,
		http.StatusTooManyRequests,
		"rate limited",
		types.ErrorCode("user_rpm_exceeded"),
	)

	if remaining := body.reader.Len(); remaining != 0 {
		t.Fatalf("request body was not drained before the early response: %d bytes remain", remaining)
	}
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusTooManyRequests)
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("user_rpm_exceeded")) {
		t.Fatalf("response body does not contain the error code: %s", recorder.Body.String())
	}
	if _, err := io.Copy(io.Discard, context.Request.Body); err != nil {
		t.Fatalf("drained body should remain readable at EOF: %v", err)
	}
}
