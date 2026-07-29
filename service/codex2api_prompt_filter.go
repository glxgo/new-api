package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/gin-gonic/gin"
)

const (
	codex2APIPromptFilterDefaultTimeoutMS = 800
	codex2APIPromptFilterMaxTextBytes     = 80 * 1024
	codex2APIPromptFilterMaxResponseBytes = 64 * 1024
)

var codex2APIPromptFilterHTTPClient = &http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   50,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 2 * time.Second,
	},
}

type codex2APIPromptFilterRequest struct {
	Text     string `json:"text"`
	Model    string `json:"model"`
	Endpoint string `json:"endpoint"`
}

type Codex2APIPromptFilterResult struct {
	Blocked    bool
	DecisionID string
	ReasonCode string
}

func Codex2APIPromptFilterEnabled() bool {
	return common.GetEnvOrDefaultBool("CODEX2API_PROMPT_FILTER_ENABLED", false) &&
		strings.TrimSpace(os.Getenv("CODEX2API_PROMPT_FILTER_URL")) != "" &&
		strings.TrimSpace(os.Getenv("CODEX2API_PROMPT_FILTER_SECRET")) != ""
}

// CheckCodex2APIPrompt runs the optional defense-in-depth sidecar before any
// billing reservation or upstream request. Transport errors and unverifiable
// responses fail open because the built-in NewAPI filter and upstream policy
// interception remain independent layers.
func CheckCodex2APIPrompt(c *gin.Context, text string, model string, endpoint string) Codex2APIPromptFilterResult {
	if c == nil || !Codex2APIPromptFilterEnabled() || strings.TrimSpace(text) == "" {
		return Codex2APIPromptFilterResult{}
	}

	filterURL := strings.TrimSpace(os.Getenv("CODEX2API_PROMPT_FILTER_URL"))
	secret := strings.TrimSpace(os.Getenv("CODEX2API_PROMPT_FILTER_SECRET"))
	parsedURL, err := url.Parse(filterURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		logger.LogWarn(c, "Codex2API prompt filter URL is invalid; request allowed by fail-open policy")
		return Codex2APIPromptFilterResult{}
	}

	payload := codex2APIPromptFilterRequest{
		Text:     boundedPromptFilterText(text),
		Model:    strings.TrimSpace(model),
		Endpoint: strings.TrimSpace(endpoint),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		logger.LogWarn(c, "Codex2API prompt filter request encoding failed; request allowed by fail-open policy")
		return Codex2APIPromptFilterResult{}
	}

	requestID := strings.TrimSpace(c.GetString(common.RequestIdKey))
	if requestID == "" {
		requestID = newCodex2APIFilterRequestID()
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	bodyDigest := sha256.Sum256(body)
	bodyDigestHex := hex.EncodeToString(bodyDigest[:])
	path := parsedURL.EscapedPath()
	if path == "" {
		path = "/"
	}
	userID := strconv.Itoa(c.GetInt("id"))
	if userID == "0" {
		userID = strconv.Itoa(c.GetInt("user_id"))
	}
	clientIP := strings.TrimSpace(c.ClientIP())
	if clientIP == "" {
		clientIP = "unknown"
	}
	canonical := strings.Join([]string{
		"v1", timestamp, requestID, userID, clientIP, http.MethodPost, path, bodyDigestHex,
	}, "\n")
	signature := hmacSHA256Hex(secret, canonical)

	timeoutMS := common.GetEnvOrDefault("CODEX2API_PROMPT_FILTER_TIMEOUT_MS", codex2APIPromptFilterDefaultTimeoutMS)
	if timeoutMS < 100 {
		timeoutMS = 100
	}
	if timeoutMS > 5000 {
		timeoutMS = 5000
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, filterURL, bytes.NewReader(body))
	if err != nil {
		logger.LogWarn(c, "Codex2API prompt filter request creation failed; request allowed by fail-open policy")
		return Codex2APIPromptFilterResult{}
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-NewAPI-User-ID", userID)
	request.Header.Set("X-NewAPI-Client-IP", clientIP)
	request.Header.Set("X-NewAPI-Request-ID", requestID)
	request.Header.Set("X-NewAPI-Timestamp", timestamp)
	request.Header.Set("X-NewAPI-Method", http.MethodPost)
	request.Header.Set("X-NewAPI-Path", path)
	request.Header.Set("X-NewAPI-Body-SHA256", bodyDigestHex)
	request.Header.Set("X-NewAPI-Signature-Version", "1")
	request.Header.Set("X-NewAPI-Signature", signature)

	response, err := codex2APIPromptFilterHTTPClient.Do(request)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			logger.LogWarn(c, "Codex2API prompt filter unavailable; request allowed by fail-open policy")
		}
		return Codex2APIPromptFilterResult{}
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, codex2APIPromptFilterMaxResponseBytes))

	if response.StatusCode == http.StatusOK {
		return Codex2APIPromptFilterResult{}
	}
	if response.StatusCode != http.StatusBadRequest {
		logger.LogWarn(c, fmt.Sprintf("Codex2API prompt filter returned status %d; request allowed by fail-open policy", response.StatusCode))
		return Codex2APIPromptFilterResult{}
	}
	result, ok := verifyCodex2APIBlockResponse(response.Header, secret, requestID)
	if !ok {
		logger.LogWarn(c, "Codex2API prompt filter returned an unverifiable block; request allowed by fail-open policy")
		return Codex2APIPromptFilterResult{}
	}
	return result
}

func boundedPromptFilterText(text string) string {
	if len(text) <= codex2APIPromptFilterMaxTextBytes {
		return text
	}
	headBytes := codex2APIPromptFilterMaxTextBytes * 3 / 4
	tailBytes := codex2APIPromptFilterMaxTextBytes - headBytes
	return text[:headBytes] + "\n[...truncated...]\n" + text[len(text)-tailBytes:]
}

func verifyCodex2APIBlockResponse(header http.Header, secret string, requestID string) (Codex2APIPromptFilterResult, bool) {
	if !strings.EqualFold(strings.TrimSpace(header.Get("X-Codex2API-Policy-Violation")), "true") ||
		strings.TrimSpace(header.Get("X-Codex2API-Policy-Request-ID")) != requestID ||
		!strings.EqualFold(strings.TrimSpace(header.Get("X-Codex2API-Policy-Action")), "block") ||
		strings.TrimSpace(header.Get("X-Codex2API-Policy-Signature-Version")) != "v1" {
		return Codex2APIPromptFilterResult{}, false
	}

	strikeEligible := strings.EqualFold(strings.TrimSpace(header.Get("X-Codex2API-Policy-Strike-Eligible")), "true")
	canonical := strings.Join([]string{
		"policy-decision-v1",
		requestID,
		strings.TrimSpace(header.Get("X-Codex2API-Policy-Decision-ID")),
		"block",
		strings.TrimSpace(header.Get("X-Codex2API-Policy-Profile")),
		strings.TrimSpace(header.Get("X-Codex2API-Policy-Reason")),
		strings.TrimSpace(header.Get("X-Codex2API-Policy-Severity")),
		strconv.FormatBool(strikeEligible),
		strings.TrimSpace(header.Get("X-Codex2API-Policy-Rule-Version")),
		strings.TrimSpace(header.Get("X-Codex2API-Policy-Evidence-SHA256")),
	}, "\n")
	expected := hmacSHA256Hex(secret, canonical)
	actual := strings.ToLower(strings.TrimSpace(header.Get("X-Codex2API-Policy-Response-Signature")))
	if expected == "" || actual == "" || !hmac.Equal([]byte(expected), []byte(actual)) {
		return Codex2APIPromptFilterResult{}, false
	}
	return Codex2APIPromptFilterResult{
		Blocked:    true,
		DecisionID: strings.TrimSpace(header.Get("X-Codex2API-Policy-Decision-ID")),
		ReasonCode: strings.TrimSpace(header.Get("X-Codex2API-Policy-Reason")),
	}, true
}

func hmacSHA256Hex(secret string, value string) string {
	if strings.TrimSpace(secret) == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func newCodex2APIFilterRequestID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "filter-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return "filter-" + hex.EncodeToString(value[:])
}
