package common

import (
	"crypto/sha256"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClientIdentityPrefixes(t *testing.T) {
	cases := map[string]string{
		"codex_desktop/26.9.1 (Macintosh)": "codex:desktop",
		"codex_cli_rs/0.110.0":             "codex:cli", "codex_vscode/0.4.1": "codex:vscode",
		"codex_exec/0.110.0": "codex:exec", "codex_sdk_ts/0.110.0": "codex:sdk",
		"codex_acp/0.3.0": "codex:acp", "claude-cli/2.1.1 (external, cli)": "claude_code",
		"pi/0.30.1": "pi", "opencode/1.2.0": "opencode", "ZCode/1.3.1": "zcode:versioned",
		"ZCode/unknown": "zcode:unknown", "deepseek-harness/0.1.0": "deepseek_harness",
		"Go-http-client/1.1": "newapi:go_inferred", "OpenClaw/1.0.0": "openclaw",
		"CherryStudio/1.0.0": "cherry_studio", "OpenAI/Python/1.50.0": "openai_sdk:python",
		"OpenAI/JS/4.1.0": "openai_sdk:node", "OpenAI/Go/1.1.0": "openai_sdk:go",
		"OpenAI/Python 1.109.0": "openai_sdk:python", "OpenAI/JS 5.0.0": "openai_sdk:node", "OpenAI/Go 3.0.0": "openai_sdk:go",
		"node": "node", "Bun/1.2.0": "bun", "python-httpx/0.28.0": "python",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X)": "browser", "curl/8.1.0": "curl",
	}
	for ua, key := range cases {
		t.Run(ua, func(t *testing.T) { require.Equal(t, key, IdentifyClient(ua).ClientKey) })
	}
	for _, ua := range []string{"proxy codex_cli_rs/1.0", " codex_cli_rs/1.0", "evil-codex/1.0", "codex", "gpt-5-codex", "ZCode/unknownish", "ZCode/1evil", "codex_cli_rs/1.0:forged", "\x00codex_cli_rs/1.0"} {
		require.Equal(t, "unknown", IdentifyClient(ua).Family, ua)
	}
	require.Equal(t, IdentifyClient("codex_cli_rs/0.1.0").ClientKey, IdentifyClient("codex_cli_rs/9.0.0-beta.2").ClientKey)
	require.Equal(t, "generic_go_ua", IdentifyClient("Go-http-client/1.1").Source)
	require.Equal(t, "inferred", IdentifyClient("Go-http-client/1.1").Confidence)
}

func TestClientIdentityRawHashAndSanitization(t *testing.T) {
	raw := "other\x00\n\t\u202e" + strings.Repeat("界", 1000)
	s := IdentifyClient(raw)
	require.Equal(t, fmt.Sprintf("unknown:%x", sha256.Sum256([]byte(raw))), s.ClientKey)
	require.True(t, s.Truncated)
	require.LessOrEqual(t, len(s.UserAgent), MaxClientUABytes)
	require.True(t, utf8.ValidString(s.UserAgent))
	require.NotContains(t, s.UserAgent, "\x00")
	require.NotContains(t, s.UserAgent, "\u202e")
	require.NotEqual(t, s.ClientKey, IdentifyClient(raw+"x").ClientKey)
	require.NotEqual(t, IdentifyClient("custom\n").ClientKey, IdentifyClient("custom").ClientKey)
}

func TestClientSnapshotSurvivesHeaderRewrite(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "codex_exec/1.0")
	c.Request.Header.Set("X-Client-Name", "Claude Code")
	c.Request.Header.Set("Authorization", "Bearer never-record-this")
	first := CaptureClientSnapshot(c)
	c.Request.Header.Set("User-Agent", "Go-http-client/1.1")
	require.Equal(t, first, CaptureClientSnapshot(c))
	encoded, err := Marshal(first)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "never-record-this")
	require.Equal(t, "codex:exec", first.ClientKey)
}
