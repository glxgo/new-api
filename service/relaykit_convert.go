package service

import (
	"context"
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/relaykit/relayconvert/convmeta"
	kittypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type relayKitContextKey struct{}

func init() {
	relayconvert.SetMediaResolver(relayconvert.MediaResolver{
		GetBase64Data: func(ctx context.Context, source kittypes.FileSource, reason ...string) (string, string, error) {
			c, ok := ctx.Value(relayKitContextKey{}).(*gin.Context)
			if !ok || c == nil || source == nil {
				return "", "", errors.New("media request context is unavailable")
			}
			mime := ""
			if inline, ok := source.(*kittypes.Base64Source); ok {
				mime = inline.MimeType
			}
			return GetBase64Data(c, types.NewFileSourceFromData(source.GetRawData(), mime), reason...)
		},
		DecodeBase64FileData: DecodeBase64FileData,
	})
}

func RelayKitContext(c *gin.Context) context.Context {
	return context.WithValue(c.Request.Context(), relayKitContextKey{}, c)
}

// NewRelayKitMeta keeps provider conversion state (notably custom-tool identity)
// scoped to one request. Transport, authentication and money remain host-owned.
func NewRelayKitMeta(info *relaycommon.RelayInfo) *convmeta.Values {
	claude := model_setting.GetClaudeSettings()
	gemini := model_setting.GetGeminiSettings()
	return &convmeta.Values{
		OriginModelName: info.OriginModelName, UpstreamModelName: info.UpstreamModelName,
		ChannelMetaAttached: true, ChannelID: info.ChannelId, ChannelType: info.ChannelType,
		IsStream: info.IsStream, EstimatePromptTokens: info.GetEstimatePromptTokens(),
		Options: &convmeta.Options{
			ToolLossPolicy: kittypes.ConversionLossPolicySafe,
			Claude: convmeta.ClaudeOptions{DefaultMaxTokens: claude.GetDefaultMaxTokens,
				ThinkingAdapterEnabled:                claude.ThinkingAdapterEnabled,
				ThinkingAdapterBudgetTokensPercentage: claude.ThinkingAdapterBudgetTokensPercentage},
			Gemini: convmeta.GeminiOptions{ThinkingAdapterEnabled: gemini.ThinkingAdapterEnabled,
				ThinkingAdapterBudgetTokensPercentage: gemini.ThinkingAdapterBudgetTokensPercentage,
				FunctionCallThoughtSignatureEnabled:   gemini.FunctionCallThoughtSignatureEnabled,
				SupportsImagine:                       model_setting.IsGeminiModelSupportImagine, SafetySetting: model_setting.GetGeminiSafetySetting},
		},
	}
}

func ConvertResponsesWithRelayKit(c *gin.Context, info *relaycommon.RelayInfo, req *dto.OpenAIResponsesRequest, target kittypes.RelayFormat) (*relayconvert.RequestResult, *convmeta.Values, error) {
	data, err := common.Marshal(req)
	if err != nil {
		return nil, nil, err
	}
	var request kitdto.OpenAIResponsesRequest
	if err := common.Unmarshal(data, &request); err != nil {
		return nil, nil, err
	}
	meta := NewRelayKitMeta(info)
	result, err := relayconvert.ConvertRequest(RelayKitContext(c), meta, target, &request)
	if err != nil {
		return nil, nil, err
	}
	if claude, ok := result.Value.(*kitdto.ClaudeRequest); ok {
		if err := normalizeClaudeToolMedia(c, claude); err != nil {
			return nil, nil, err
		}
	}
	info.ReasoningEffort = meta.ReasoningEffort
	return result, meta, nil
}

func RelayKitUsage(usage *kitdto.Usage) (*dto.Usage, error) {
	result := &dto.Usage{}
	if usage == nil {
		return result, nil
	}
	data, err := common.Marshal(usage)
	if err != nil {
		return nil, err
	}
	err = common.Unmarshal(data, result)
	return result, err
}
