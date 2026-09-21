package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestClaudeReasoningEffortReachesLogFromOutboundRequest(t *testing.T) {
	require.NoError(t, i18n.Init())
	settings := model_setting.GetGlobalSettings()
	oldPass := settings.PassThroughRequestEnabled
	settings.PassThroughRequestEnabled = false
	t.Cleanup(func() { settings.PassThroughRequestEnabled = oldPass })
	for _, tc := range []struct {
		name, model, effort, override, want string
		pass                                bool
	}{
		{name: "native", model: "claude-opus-4-6", effort: "high", want: "high"},
		{name: "native max", model: "claude-opus-4-6", effort: "max", want: "max"},
		{name: "model suffix", model: "claude-opus-4-6-high", want: "high"},
		{name: "max model suffix", model: "claude-opus-4-6-max", want: "max"},
		{name: "channel override wins", model: "claude-opus-4-6", effort: "high", override: "low", want: "low"},
		{name: "passthrough preserves original", model: "claude-opus-4-6", effort: "medium", override: "low", pass: true, want: "medium"},
		{name: "no effort clears stale", model: "claude-opus-4-6"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			incoming := dto.ClaudeRequest{Model: tc.model, MaxTokens: common.GetPointer(uint(1280))}
			if tc.effort != "" {
				incoming.OutputConfig = []byte(`{"effort":"` + tc.effort + `"}`)
				incoming.Thinking = &dto.Thinking{Type: "adaptive"}
			}
			payload, err := common.Marshal(incoming)
			require.NoError(t, err)
			wire := make(chan []byte, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				wire <- body
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(400)
				io.WriteString(w, `{"error":{"type":"invalid_request_error","message":"fixture stop before billing"}}`)
			}))
			defer server.Close()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/messages", strings.NewReader(string(payload)))
			c.Request.Header.Set("Content-Type", "application/json")
			defer common.CleanupBodyStorage(c)
			common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeAnthropic)
			common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, server.URL)
			common.SetContextKey(c, constant.ContextKeyOriginalModel, tc.model)
			common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{PassThroughBodyEnabled: tc.pass})
			if tc.override != "" {
				common.SetContextKey(c, constant.ContextKeyChannelParamOverride, map[string]interface{}{"output_config": map[string]interface{}{"effort": tc.override}})
			}
			info := &relaycommon.RelayInfo{Request: &incoming, OriginModelName: tc.model, RelayFormat: types.RelayFormatClaude, ReasoningEffort: "stale"}
			require.NotNil(t, ClaudeHelper(c, info))
			sent := <-wire
			require.Equal(t, tc.want, gjson.GetBytes(sent, "output_config.effort").String())
			require.Equal(t, tc.want, info.ReasoningEffort)
			if tc.pass {
				require.JSONEq(t, string(payload), string(sent))
			}
			logOther := service.GenerateClaudeOtherInfo(c, info, 1, 1, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1)
			if tc.want == "" {
				require.NotContains(t, logOther, "reasoning_effort")
			} else {
				require.Equal(t, tc.want, logOther["reasoning_effort"])
			}
		})
	}
}
