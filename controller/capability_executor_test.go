package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestCapabilityExecutorPinsChannelGroupAndPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var requests int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer test-private-key", r.Header.Get("Authorization"))
		require.Equal(t, "real-model", gjson.GetBytes(body, "model").String())
		require.Equal(t, "test question", gjson.GetBytes(body, "messages.0.content").String())
		require.EqualValues(t, 4096, gjson.GetBytes(body, "max_tokens").Int())
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"fixture","model":"real-model","choices":[{"index":0,"message":{"role":"assistant","content":"{\"answers\":[{\"id\":\"anchor\",\"value\":29},{\"id\":\"random\",\"value\":35}]}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":20,"completion_tokens":30,"total_tokens":50}}`)
	}))
	defer upstream.Close()
	base, mapping := upstream.URL, `{"alias":"real-model"}`
	ch := &model.Channel{Id: 42, Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Key: "test-private-key", BaseURL: &base, ModelMapping: &mapping}
	target := service.CapabilityTarget{ChannelID: 42, Group: "A", Model: "alias"}
	profile := model.CapabilityProfile{Model: "alias", Protocol: "chat", MaxTokens: 4096, Enabled: true}
	p, err := prepareCapability(context.Background(), target, profile, "test question", "", ch, ch.Key, 0, []byte("local-fixture-fingerprint-secret-123"))
	require.NoError(t, err)
	require.Equal(t, "A", p.info.UsingGroup)
	result := p.execute()
	require.Empty(t, result.Reason)
	require.Contains(t, result.Answer, `"anchor"`)
	require.NotEmpty(t, result.Usage)
	require.Equal(t, 1, requests)
	target.Group = "B"
	other, err := prepareCapability(context.Background(), target, profile, "test question", "", ch, ch.Key, 0, []byte("local-fixture-fingerprint-secret-123"))
	require.NoError(t, err)
	require.Equal(t, p.fingerprint, other.fingerprint)
	require.NotContains(t, p.fingerprint, ch.Key)
	require.False(t, strings.Contains(result.Usage, "estimated"))
}
