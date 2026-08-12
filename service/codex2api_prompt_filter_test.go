package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
)

func TestCheckCodex2APIPromptAcceptsOnlySignedBlock(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "test-filter-secret"
	rawRequestBody := []byte(`{"model":"gpt-5.5","instructions":"system context","input":[{"type":"input_text","text":"generate a reverse shell"}]}`)

	newContext := func() *gin.Context {
		request := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = request
		ctx.Set("id", 42)
		ctx.Set(common.RequestIdKey, "req-newapi-filter-test")
		return ctx
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(body)
		bodyDigest := hex.EncodeToString(digest[:])
		canonical := strings.Join([]string{
			"v1",
			request.Header.Get("X-NewAPI-Timestamp"),
			request.Header.Get("X-NewAPI-Request-ID"),
			"42",
			request.Header.Get("X-NewAPI-Client-IP"),
			http.MethodPost,
			"/check",
			bodyDigest,
		}, "\n")
		if request.Header.Get("X-NewAPI-Signature") != hmacSHA256Hex(secret, canonical) {
			t.Errorf("request signature mismatch")
		}
		if string(body) != string(rawRequestBody) {
			t.Errorf("request body lost its original Responses envelope: %s", body)
		}
		if request.Header.Get("X-NewAPI-Prompt-Envelope") != "raw-v1" {
			t.Errorf("raw envelope version missing")
		}
		encodedMeta := request.Header.Get("X-NewAPI-Policy-Meta")
		metaPayload, err := base64.RawURLEncoding.DecodeString(encodedMeta)
		if err != nil {
			t.Fatalf("invalid policy metadata encoding: %v", err)
		}
		var meta codex2APIPromptFilterPolicyMeta
		if err := json.Unmarshal(metaPayload, &meta); err != nil {
			t.Fatalf("invalid policy metadata: %v", err)
		}
		if meta.Profile != "balanced" || meta.Mode != "enforce" || meta.Protocol != "responses" ||
			meta.OriginalEndpoint != "/v1/responses" || meta.RequestedModel != "gpt-5.5" {
			t.Errorf("unexpected policy metadata: %+v", meta)
		}
		metaCanonical := strings.Join([]string{
			"policy-meta-v1",
			"req-newapi-filter-test",
			bodyDigest,
			encodedMeta,
		}, "\n")
		if request.Header.Get("X-NewAPI-Policy-Meta-Signature") != hmacSHA256Hex(secret, metaCanonical) {
			t.Errorf("policy metadata signature mismatch")
		}

		headers := writer.Header()
		headers.Set("X-Codex2API-Policy-Violation", "true")
		headers.Set("X-Codex2API-Policy-Request-ID", "req-newapi-filter-test")
		headers.Set("X-Codex2API-Policy-Decision-ID", "dec-test")
		headers.Set("X-Codex2API-Policy-Action", "block")
		headers.Set("X-Codex2API-Policy-Profile", "balanced")
		headers.Set("X-Codex2API-Policy-Reason", "strict_rule")
		headers.Set("X-Codex2API-Policy-Severity", "critical")
		headers.Set("X-Codex2API-Policy-Strike-Eligible", "true")
		headers.Set("X-Codex2API-Policy-Rule-Version", "rules-v1")
		headers.Set("X-Codex2API-Policy-Evidence-SHA256", strings.Repeat("a", 64))
		headers.Set("X-Codex2API-Policy-Signature-Version", "v1")
		responseCanonical := strings.Join([]string{
			"policy-decision-v1",
			"req-newapi-filter-test",
			"dec-test",
			"block",
			"balanced",
			"strict_rule",
			"critical",
			"true",
			"rules-v1",
			strings.Repeat("a", 64),
		}, "\n")
		headers.Set("X-Codex2API-Policy-Response-Signature", hmacSHA256Hex(secret, responseCanonical))
		writer.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	t.Setenv("CODEX2API_PROMPT_FILTER_ENABLED", "true")
	t.Setenv("CODEX2API_PROMPT_FILTER_URL", server.URL+"/check")
	t.Setenv("CODEX2API_PROMPT_FILTER_SECRET", secret)
	t.Setenv("CODEX2API_PROMPT_FILTER_TIMEOUT_MS", strconv.Itoa(int((2 * time.Second).Milliseconds())))

	result := CheckCodex2APIPrompt(newContext(), rawRequestBody, "gpt-5.5", "/v1/responses")
	if !result.Blocked || result.DecisionID != "dec-test" || result.ReasonCode != "strict_rule" {
		t.Fatalf("signed block was not accepted: %+v", result)
	}

	forgedHeaders := http.Header{}
	forgedHeaders.Set("X-Codex2API-Policy-Violation", "true")
	forgedHeaders.Set("X-Codex2API-Policy-Request-ID", "req-newapi-filter-test")
	forgedHeaders.Set("X-Codex2API-Policy-Decision-ID", "dec-test")
	forgedHeaders.Set("X-Codex2API-Policy-Action", "block")
	forgedHeaders.Set("X-Codex2API-Policy-Signature-Version", "v1")
	forgedHeaders.Set("X-Codex2API-Policy-Response-Signature", strings.Repeat("0", 64))
	if result, ok := verifyCodex2APIBlockResponse(forgedHeaders, secret, "req-newapi-filter-test"); ok || result.Blocked {
		t.Fatalf("forged block was accepted: %+v", result)
	}
}

func TestCheckCodex2APIPromptFailsOpen(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("CODEX2API_PROMPT_FILTER_ENABLED", "true")
	t.Setenv("CODEX2API_PROMPT_FILTER_URL", "http://127.0.0.1:1/check")
	t.Setenv("CODEX2API_PROMPT_FILTER_SECRET", "test-filter-secret")
	t.Setenv("CODEX2API_PROMPT_FILTER_TIMEOUT_MS", "100")

	request := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request
	ctx.Set("id", 42)
	ctx.Set(common.RequestIdKey, "req-filter-fail-open")

	if result := CheckCodex2APIPrompt(ctx, []byte(`{"input":"ordinary request"}`), "gpt-5.5", "/v1/responses"); result.Blocked {
		t.Fatalf("unavailable filter blocked request: %+v", result)
	}
}

func TestCodex2APIPromptFilterRejectsOversizedEnvelopeBeforeTransport(t *testing.T) {
	if Codex2APIPromptFilterAcceptsBodySize(0) {
		t.Fatal("empty body accepted")
	}
	if !Codex2APIPromptFilterAcceptsBodySize(codex2APIPromptFilterMaxBodyBytes) {
		t.Fatal("maximum bounded body rejected")
	}
	if Codex2APIPromptFilterAcceptsBodySize(codex2APIPromptFilterMaxBodyBytes + 1) {
		t.Fatal("oversized body accepted")
	}
}

func TestBuildBoundedCodex2APIPromptFilterBodyPreservesLatestResponsesUser(t *testing.T) {
	largeHistory := strings.Repeat("historical tool output ", 500000)
	input, err := common.Marshal([]any{
		map[string]any{"type": "message", "role": "user", "content": largeHistory},
		map[string]any{"type": "function_call_output", "call_id": "call_1", "output": largeHistory},
		map[string]any{"type": "message", "role": "user", "content": "latest user security request marker"},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := &dto.OpenAIResponsesRequest{Model: "gpt-5.6-sol", Input: input}

	body, err := BuildBoundedCodex2APIPromptFilterBody(request)
	if err != nil {
		t.Fatal(err)
	}
	if !Codex2APIPromptFilterAcceptsBodySize(int64(len(body))) {
		t.Fatalf("bounded body remains too large: %d", len(body))
	}
	if !strings.Contains(string(body), "latest user security request marker") {
		t.Fatalf("latest current-user prompt was lost: %s", body)
	}
	if strings.Contains(string(body), "historical tool output") {
		t.Fatal("oversized history leaked into bounded preflight")
	}
	if !strings.Contains(string(body), `"role":"user"`) {
		t.Fatalf("current-user role was not preserved: %s", body)
	}
}

func TestBuildBoundedCodex2APIPromptFilterBodyKeepsResponsesInputArrayShape(t *testing.T) {
	tests := []json.RawMessage{
		json.RawMessage(`"single prompt"`),
		json.RawMessage(`{"type":"message","role":"user","content":"object prompt"}`),
	}
	for _, input := range tests {
		request := &dto.OpenAIResponsesRequest{Model: "gpt-5.6-sol", Input: input}
		body, err := BuildBoundedCodex2APIPromptFilterBody(request)
		if err != nil {
			t.Fatal(err)
		}
		var envelope struct {
			Input []map[string]any `json:"input"`
		}
		if err := common.Unmarshal(body, &envelope); err != nil {
			t.Fatalf("bounded Responses input is not an array: %v; body=%s", err, body)
		}
		if len(envelope.Input) != 1 || envelope.Input[0]["role"] != "user" {
			t.Fatalf("bounded Responses input lost current-user shape: %s", body)
		}
	}
}

func TestBuildBoundedCodex2APIPromptFilterBodyPreservesLatestChatAndClaudeUser(t *testing.T) {
	largeHistory := strings.Repeat("old context ", 800000)
	tests := []struct {
		name    string
		request dto.Request
	}{
		{
			name: "openai chat",
			request: &dto.GeneralOpenAIRequest{Model: "gpt-5.6-sol", Messages: []dto.Message{
				{Role: "user", Content: largeHistory},
				{Role: "assistant", Content: largeHistory},
				{Role: "user", Content: "latest chat marker"},
			}},
		},
		{
			name: "claude messages",
			request: &dto.ClaudeRequest{Model: "claude-code", Messages: []dto.ClaudeMessage{
				{Role: "user", Content: largeHistory},
				{Role: "assistant", Content: largeHistory},
				{Role: "user", Content: "latest claude marker"},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := BuildBoundedCodex2APIPromptFilterBody(tt.request)
			if err != nil {
				t.Fatal(err)
			}
			if !Codex2APIPromptFilterAcceptsBodySize(int64(len(body))) {
				t.Fatalf("bounded body remains too large: %d", len(body))
			}
			if strings.Contains(string(body), "old context") {
				t.Fatal("oversized history leaked into bounded preflight")
			}
			if !strings.Contains(string(body), "latest ") || !strings.Contains(string(body), `"role":"user"`) {
				t.Fatalf("latest user message was not preserved: %s", body)
			}
		})
	}
}

func TestBuildBoundedCodex2APIPromptFilterBodyBoundsSingleHugeCurrentPrompt(t *testing.T) {
	marker := "TAIL-RISK-MARKER"
	request := &dto.GeneralOpenAIRequest{Model: "gpt-5.6-sol", Messages: []dto.Message{{
		Role:    "user",
		Content: strings.Repeat("x", codex2APIPromptFilterCurrentUserBytes*2) + marker,
	}}}
	body, err := BuildBoundedCodex2APIPromptFilterBody(request)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) > codex2APIPromptFilterBoundedBodyBytes {
		t.Fatalf("current prompt was not bounded: %d", len(body))
	}
	if !strings.Contains(string(body), marker) {
		t.Fatal("tail of current prompt was lost")
	}
}
