package common

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const ClientSnapshotContextKey = "request_client_snapshot"
const ClientCodingContextKey = "request_client_coding_group"
const MaxClientUABytes = 2048

// ClientSnapshot is derived only from the original UA, never model names or
// caller-supplied identity headers. It describes a claim, not authentication.
type ClientSnapshot struct {
	ClientKey   string `json:"client_key"`
	Family      string `json:"family"`
	Variant     string `json:"variant"`
	DisplayName string `json:"display_name"`
	Version     string `json:"version"`
	Confidence  string `json:"confidence"`
	Source      string `json:"source"`
	UserAgent   string `json:"user_agent"`
	Truncated   bool   `json:"truncated"`
}

type clientUARule struct {
	pattern                    *regexp.Regexp
	key, family, variant, name string
}

func uaRule(prefix, key, family, variant, name string) clientUARule {
	return uaRuleSeparator(prefix, `/`, key, family, variant, name)
}

func uaRuleSeparator(prefix, separator, key, family, variant, name string) clientUARule {
	return clientUARule{regexp.MustCompile(`^(?:` + prefix + `)` + separator + `([0-9]{1,8}(?:\.[0-9]{1,8}){0,5}(?:[-+][0-9A-Za-z.-]{1,64})?)(?:[ ;(]|$)`), key, family, variant, name}
}

var clientUARules = []clientUARule{
	uaRule(`Codex Desktop|codex_desktop|codex-desktop|Codex`, "codex:desktop", "codex", "desktop", "Codex Desktop"),
	uaRule(`codex_vscode|codex-vscode`, "codex:vscode", "codex", "vscode", "Codex VS Code"),
	uaRule(`codex_exec|codex-exec`, "codex:exec", "codex", "exec", "Codex exec"),
	uaRule(`codex_sdk_ts|codex_sdk|codex-sdk|@openai/codex-sdk`, "codex:sdk", "codex", "sdk", "Codex SDK"),
	uaRule(`codex_acp|codex-acp`, "codex:acp", "codex", "acp", "Codex ACP"),
	uaRule(`codex_cli_rs|codex-cli|codex_tui|codex-cli-tui`, "codex:cli", "codex", "cli_tui", "Codex CLI / TUI"),
	uaRule(`codex`, "codex:unspecified", "codex", "unspecified", "Codex"),
	uaRule(`claude-cli|claude-code|Claude-Code`, "claude_code", "claude_code", "cli", "Claude Code"),
	uaRule(`pi|pi-coding-agent`, "pi", "pi", "cli", "Pi"),
	uaRule(`opencode|OpenCode`, "opencode", "opencode", "unspecified", "OpenCode"),
	uaRule(`ZCode`, "zcode:versioned", "zcode", "versioned", "ZCode"),
	uaRule(`deepseek-harness`, "deepseek_harness", "deepseek_harness", "harness", "DeepSeek Harness (DSH)"),
	uaRule(`Go-http-client`, "newapi:go_inferred", "newapi", "go_inferred", "NewAPI"),
	uaRule(`OpenClaw|openclaw`, "openclaw", "openclaw", "unspecified", "OpenClaw"),
	uaRule(`CherryStudio|Cherry-Studio|Cherry Studio`, "cherry_studio", "cherry_studio", "desktop", "Cherry Studio"),
	uaRuleSeparator(`OpenAI/Python`, `[ /]`, "openai_sdk:python", "openai_sdk", "python", "OpenAI SDK"),
	uaRuleSeparator(`OpenAI/JS`, `[ /]`, "openai_sdk:node", "openai_sdk", "node", "OpenAI SDK"),
	uaRuleSeparator(`OpenAI/Go`, `[ /]`, "openai_sdk:go", "openai_sdk", "go", "OpenAI SDK"),
	uaRule(`openai-python`, "openai_sdk:python", "openai_sdk", "python", "OpenAI SDK"),
	uaRule(`openai-node`, "openai_sdk:node", "openai_sdk", "node", "OpenAI SDK"),
	uaRule(`node|Node\.js|node-fetch`, "node", "node", "runtime", "Node"),
	uaRule(`Bun|bun`, "bun", "bun", "runtime", "Bun"),
	uaRule(`python-requests|python-httpx|Python-urllib|Python`, "python", "python", "runtime", "Python"),
	uaRule(`curl`, "curl", "curl", "cli", "curl"),
	uaRule(`Mozilla`, "browser", "browser", "browser", "Browser"),
}
var zcodeUnknownUA = regexp.MustCompile(`^ZCode/unknown(?:[ ;(]|$)`)

// SanitizeClientUA bounds retained bytes without splitting UTF-8. Hashing and
// recognition happen on raw input so sanitation cannot manufacture a prefix.
func SanitizeClientUA(raw string) (string, bool) {
	var b strings.Builder
	for _, r := range raw {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == utf8.RuneError {
			continue
		}
		if b.Len()+utf8.RuneLen(r) > MaxClientUABytes {
			return b.String(), true
		}
		b.WriteRune(r)
	}
	return b.String(), len(raw) > MaxClientUABytes
}
func IdentifyClient(raw string) ClientSnapshot {
	ua, truncated := SanitizeClientUA(raw)
	s := ClientSnapshot{ClientKey: fmt.Sprintf("unknown:%x", sha256.Sum256([]byte(raw))), Family: "unknown", Variant: "unknown", DisplayName: "Unknown client", Confidence: "unknown", Source: "unrecognized_ua", UserAgent: ua, Truncated: truncated}
	if zcodeUnknownUA.MatchString(raw) {
		s.ClientKey, s.Family, s.Variant, s.DisplayName, s.Version, s.Confidence, s.Source = "zcode:unknown", "zcode", "unknown_version", "ZCode (unknown)", "unknown", "prefix", "ua_prefix"
		return s
	}
	// Bounded parsing; the full raw string is used only for the unknown hash.
	probe := raw
	if len(probe) > MaxClientUABytes {
		probe = probe[:MaxClientUABytes]
	}
	for _, rule := range clientUARules {
		m := rule.pattern.FindStringSubmatch(probe)
		if m == nil {
			continue
		}
		s.ClientKey, s.Family, s.Variant, s.DisplayName, s.Version, s.Confidence, s.Source = rule.key, rule.family, rule.variant, rule.name, m[1], "prefix", "ua_prefix"
		if rule.family == "newapi" {
			s.Confidence, s.Source = "inferred", "generic_go_ua"
		}
		return s
	}
	if raw == "node" || raw == "undici" {
		s.ClientKey, s.Family, s.Variant, s.DisplayName, s.Confidence, s.Source = "node", "node", "runtime", "Node", "prefix", "ua_prefix"
	}
	return s
}
func CaptureClientSnapshot(c *gin.Context) ClientSnapshot {
	if s, ok := GetClientSnapshot(c); ok {
		return s
	}
	raw := ""
	if c.Request != nil {
		raw = c.Request.UserAgent()
	}
	s := IdentifyClient(raw)
	c.Set(ClientSnapshotContextKey, s)
	return s
}
func GetClientSnapshot(c *gin.Context) (ClientSnapshot, bool) {
	if c == nil {
		return ClientSnapshot{}, false
	}
	value, ok := c.Get(ClientSnapshotContextKey)
	if !ok {
		return ClientSnapshot{}, false
	}
	s, ok := value.(ClientSnapshot)
	return s, ok
}

func ClientSnapshotPointer(c *gin.Context) *ClientSnapshot {
	s, ok := GetClientSnapshot(c)
	if !ok {
		return nil
	}
	return &s
}
