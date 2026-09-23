package sora

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicErrorSubmission(t *testing.T) {
	body := `{"id":"upstream-task","status":"failed","error":{"message":"502, url: https://private.example/video, cf-ray: private-trace","code":"https://private.example/code"}}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}
	info.PublicTaskID = "task_public"
	id, original, taskErr := (&TaskAdaptor{}).DoResponse(c, &http.Response{Body: io.NopCloser(strings.NewReader(body))}, info)
	require.Nil(t, taskErr)
	require.Equal(t, "upstream-task", id)
	require.Equal(t, body, string(original))
	require.NotContains(t, recorder.Body.String(), "private.example")
	require.NotContains(t, recorder.Body.String(), "cf-ray")
	require.Contains(t, recorder.Body.String(), `"message":"502"`)
	require.Contains(t, recorder.Body.String(), "task_public")
}
