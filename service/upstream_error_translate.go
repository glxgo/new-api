package service

import (
	"strings"

	"github.com/QuantumNous/new-api/types"
)

// upstreamPattern 描述一条「上游错误关键词 → 中文文案」的映射。
type upstreamPattern struct {
	keywords []string // 小写关键词，命中任意一个即采用本条文案
	zh       string   // 中文文案
}

// upstreamPatterns 按优先级排列：更具体的关键词放前面，避免被宽泛词提前命中。
//
// 仅收录上游模型服务商真实高频返回的错误特征词；过于宽泛、容易误伤正常报文的词
// （如单独的 safety / capacity / quota）不单独成条，必要时收紧为更长的短语。
var upstreamPatterns = []upstreamPattern{
	{keywords: []string{"rate_limit", "rate limit", "too many requests"}, zh: "请求过于频繁，上游已限流，请稍后重试"},
	{keywords: []string{"insufficient_quota", "exceeded your current quota", "quota exceeded", "billing limit", "monthly cap"}, zh: "上游账号额度已用尽，请联系站长"},
	{keywords: []string{"invalid api key", "incorrect api key", "invalid_api_key"}, zh: "上游接口密钥无效（渠道配置异常），请联系站长"},
	{keywords: []string{"model not found", "model_not_found", "does not exist"}, zh: "上游不支持该模型，请更换模型"},
	{keywords: []string{"context length", "maximum context", "context_window", "reduce the length"}, zh: "输入超出模型上下文长度限制，请精简后重试"},
	{keywords: []string{"service is overloaded", "overloaded"}, zh: "上游服务暂时过载，请稍后重试"},
	{keywords: []string{"has been deprecated", "deprecated", "shut down"}, zh: "该模型已下线，请更换模型"},
	{keywords: []string{"content filter", "content_filter", "content policy"}, zh: "内容被上游安全策略拦截，请修改后重试"},
	{keywords: []string{"timed out", "timeout"}, zh: "上游响应超时，请稍后重试"},
	{keywords: []string{"internal server error", "bad gateway", "service unavailable", "gateway timeout"}, zh: "上游服务异常，请稍后重试"},
	{keywords: []string{"permission denied", "access denied"}, zh: "上游权限不足，请联系站长"},
	{keywords: []string{"unauthorized", "authentication"}, zh: "上游鉴权失败，请联系站长"},
}

// 未命中映射表时的中文说明（保留原文便于排查）。
const (
	upstreamUnknownPrefix = "上游模型服务报错："
	upstreamUnknownSuffix = "（该错误来自上游服务商，通常并非中转站故障，可稍后重试或更换模型）"
)

// TranslateUpstreamMessage 把上游返回的错误 message 翻译成中文：
//   - 命中映射表 → 返回对应中文文案；
//   - 未命中 → 中文前缀 + 原文 + 中文后缀，既让用户看懂来源、又保留原文便于排查；
//   - 空消息原样返回（交给调用方兜底）。
func TranslateUpstreamMessage(message string) string {
	if message == "" {
		return message
	}
	lower := strings.ToLower(message)
	for _, p := range upstreamPatterns {
		for _, kw := range p.keywords {
			if strings.Contains(lower, kw) {
				return p.zh
			}
		}
	}
	return upstreamUnknownPrefix + message + upstreamUnknownSuffix
}

func init() {
	// 将上游错误中文化函数注入 types 包，供 ToOpenAIError / ToClaudeError 在序列化前
	// 改写「上游来源」错误的 message。放在 service 的 init() 里，可避免
	// i18n/types/service 三者之间的循环依赖（依赖方向始终保持 service → types 单向）。
	// service 包被 main.go 引用，init() 必然在服务启动前执行。
	types.TranslateUpstreamError = TranslateUpstreamMessage
}
