package common

import (
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/types"
)

// IsChannelPreferredForFormat 判断渠道类型是否匹配请求协议（用于协议优先路由，避免跨协议转换引发 tools 转换 bug，见 Issue #4755）
func IsChannelPreferredForFormat(channelType int, relayFormat types.RelayFormat) bool {
	if relayFormat == "" {
		return false
	}
	apiType, _ := ChannelType2APIType(channelType)
	switch relayFormat {
	case types.RelayFormatClaude:
		return apiType == constant.APITypeAnthropic
	case types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses, types.RelayFormatOpenAIResponsesCompaction:
		return apiType == constant.APITypeOpenAI
	}
	return false
}

// PathToRelayFormat 从请求 URL path 推断 RelayFormat（distributor 中间件阶段 RelayInfo 尚未生成，用 path 判断）
func PathToRelayFormat(path string) types.RelayFormat {
	switch {
	case strings.Contains(path, "/messages"):
		return types.RelayFormatClaude
	case strings.Contains(path, "/chat/completions"), strings.Contains(path, "/completions"), strings.Contains(path, "/responses"):
		return types.RelayFormatOpenAI
	}
	return ""
}
