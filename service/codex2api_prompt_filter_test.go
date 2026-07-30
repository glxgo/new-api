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
