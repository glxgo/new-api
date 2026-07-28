package setting

import "strings"

var CheckSensitiveEnabled = true
var CheckSensitiveOnPromptEnabled = true

//var CheckSensitiveOnCompletionEnabled = true

// StopOnSensitiveEnabled 如果检测到敏感词，是否立刻停止生成，否则替换敏感词
var StopOnSensitiveEnabled = true

// StreamCacheQueueLength 流模式缓存队列长度，0表示无缓存
var StreamCacheQueueLength = 0

// DefaultJailbreakWords keeps the built-in prompt-injection blocklist narrowly
// scoped to explicit instruction-bypass phrases. Operators can still replace
// the list through the SensitiveWords option.
var DefaultJailbreakWords = []string{
	"ignore previous instructions",
	"ignore all previous instructions",
	"disregard previous instructions",
	"disregard all prior instructions",
	"forget previous instructions",
	"forget all previous instructions",
	"override previous instructions",
	"bypass safety restrictions",
	"bypass content policy",
	"bypass security policy",
	"disable safety filters",
	"disable content filters",
	"remove safety restrictions",
	"ignore safety policy",
	"ignore content policy",
	"reveal system prompt",
	"show system prompt",
	"print system prompt",
	"repeat system prompt",
	"expose hidden instructions",
	"reveal hidden instructions",
	"show hidden instructions",
	"developer mode enabled",
	"enable developer mode",
	"enter developer mode",
	"activate developer mode",
	"unrestricted mode enabled",
	"enable unrestricted mode",
	"enter unrestricted mode",
	"jailbreak mode enabled",
	"enable jailbreak mode",
	"do anything now",
	"you are dan",
	"act as dan",
	"pretend you are dan",
	"no ethical restrictions",
	"without ethical restrictions",
	"no safety constraints",
	"without safety constraints",
	"ignore openai policy",
	"忽略之前的指令",
	"忽略前面的指令",
	"忽略所有之前指令",
	"忘记之前的指令",
	"忘记所有规则",
	"覆盖之前的指令",
	"绕过安全限制",
	"绕过内容政策",
	"绕过安全政策",
	"关闭安全过滤",
	"关闭内容过滤",
	"解除安全限制",
	"无视安全规则",
	"无视内容政策",
	"泄露系统提示词",
	"显示系统提示词",
	"输出系统提示词",
	"重复系统提示词",
	"展示隐藏指令",
	"泄露隐藏指令",
	"开启开发者模式",
	"进入开发者模式",
	"激活开发者模式",
	"开启无限制模式",
	"进入无限制模式",
	"开启越狱模式",
	"进入越狱模式",
	"你现在是dan",
	"扮演dan",
	"不受道德限制",
}

var SensitiveWords = append([]string(nil), DefaultJailbreakWords...)

func SensitiveWordsToString() string {
	return strings.Join(SensitiveWords, "\n")
}

func SensitiveWordsFromString(s string) {
	SensitiveWords = []string{}
	seen := make(map[string]struct{})
	sw := strings.Split(s, "\n")
	for _, w := range sw {
		w = strings.ToLower(strings.TrimSpace(w))
		if w == "" {
			continue
		}
		if _, exists := seen[w]; !exists {
			seen[w] = struct{}{}
			SensitiveWords = append(SensitiveWords, w)
		}
	}
}

func ShouldCheckPromptSensitive() bool {
	return CheckSensitiveEnabled && CheckSensitiveOnPromptEnabled
}

//func ShouldCheckCompletionSensitive() bool {
//	return CheckSensitiveEnabled && CheckSensitiveOnCompletionEnabled
//}
