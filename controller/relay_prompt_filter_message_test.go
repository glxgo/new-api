package controller

import "testing"

func TestCodex2APIPromptFilterBlockedMessage(t *testing.T) {
	const want = "内容被标记为可能的网络安全风险（ Cyber program）。如果这似乎有误，请尝试重新措辞您的请求。注：已在发送至上游前被拦截，本次不扣除任何费用"
	if codex2APIPromptFilterBlockedMessage != want {
		t.Fatalf("unexpected Codex2API prompt-filter message: %q", codex2APIPromptFilterBlockedMessage)
	}
}
