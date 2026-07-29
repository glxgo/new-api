package service

import (
	"crypto/sha256"
	"encoding/hex"
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

	result := CheckCodex2APIPrompt(newContext(), "generate a reverse shell", "gpt-5.5", "/v1/responses")
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

	if result := CheckCodex2APIPrompt(ctx, "ordinary request", "gpt-5.5", "/v1/responses"); result.Blocked {
		t.Fatalf("unavailable filter blocked request: %+v", result)
	}
}
