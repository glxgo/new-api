package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	kittypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"net/http/httptest"
	"testing"
)

func TestRelayKitCustomToolsAndMedia(t *testing.T) {
	for _, target := range []kittypes.RelayFormat{kittypes.RelayFormatOpenAI, kittypes.RelayFormatClaude, kittypes.RelayFormatGemini} {
		t.Run(string(target), func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
			var request dto.OpenAIResponsesRequest
			require.NoError(t, common.UnmarshalJsonStr(`{"model":"test","input":[{"type":"custom_tool_call","call_id":"call-1","name":"patch","input":"hello"},{"type":"custom_tool_call_output","call_id":"call-1","output":[{"type":"input_text","text":"done"},{"type":"input_image","image_url":"data:image/png;base64,aGVsbG8="}]}],"tools":[{"type":"custom","name":"patch","format":{"type":"text"}}]}`, &request))
			info := &relaycommon.RelayInfo{OriginModelName: "test", ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "test"}}
			result, _, err := ConvertResponsesWithRelayKit(c, info, &request, target)
			require.NoError(t, err)
			raw, err := common.Marshal(result.Value)
			require.NoError(t, err)
			require.True(t, gjson.GetBytes(raw, "tools").Exists())
			require.NotContains(t, string(raw), `\"image_url\"`)
			if target == kittypes.RelayFormatGemini {
				require.Contains(t, string(raw), "inlineData")
			}
			if target == kittypes.RelayFormatClaude {
				require.Contains(t, string(raw), `"type":"image"`)
			}
			if target == kittypes.RelayFormatOpenAI {
				require.Contains(t, string(raw), `"type":"image_url"`)
			}
		})
	}
}

func TestClaudeToolMediaIsNotSerializedAsText(t *testing.T) {
	text, media, err := claudeToolResultToChat(nil, []dto.ClaudeMediaMessage{{Type: "image", Source: &dto.ClaudeMessageSource{Type: "base64", MediaType: "image/png", Data: "aGVsbG8="}}, {Type: "document", Source: &dto.ClaudeMessageSource{Type: "base64", MediaType: "application/pdf", Data: "cGRm"}}})
	require.NoError(t, err)
	require.NotContains(t, text, "aGVsbG8=")
	require.Len(t, media, 2)
	require.Equal(t, "image_url", media[0].Type)
	require.Equal(t, "file", media[1].Type)
}

func TestClaudeOpus55DefaultEffort(t *testing.T) {
	for _, test := range []struct{ name, raw, want string }{
		{"default", `{}`, "medium"},
		{"adaptive", `{"thinking":{"type":"adaptive"}}`, "medium"},
		{"explicit", `{"output_config":{"effort":"low"}}`, "low"},
		{"disabled", `{"thinking":{"type":"disabled"}}`, ""},
		{"budget", `{"thinking":{"type":"enabled","budget_tokens":2048}}`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			var request dto.ClaudeRequest
			require.NoError(t, common.UnmarshalJsonStr(test.raw, &request))
			request.Model = "claude-opus-5-5"
			info := &relaycommon.RelayInfo{OriginModelName: request.Model, ChannelMeta: &relaycommon.ChannelMeta{}}
			converted, err := ClaudeToOpenAIRequest(nil, request, info)
			require.NoError(t, err)
			require.Equal(t, test.want, converted.ReasoningEffort)
			require.Equal(t, test.want, info.ReasoningEffort)
		})
	}
}
