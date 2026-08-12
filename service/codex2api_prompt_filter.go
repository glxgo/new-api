package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/gin-gonic/gin"
)

const (
	codex2APIPromptFilterDefaultTimeoutMS = 2000
	codex2APIPromptFilterMaxBodyBytes     = 8 * 1024 * 1024
	codex2APIPromptFilterMaxResponseBytes = 64 * 1024
	codex2APIPromptFilterCurrentUserBytes = 128 * 1024
	codex2APIPromptFilterBoundedBodyBytes = 256 * 1024
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

type codex2APIPromptFilterPolicyMeta struct {
	Profile          string `json:"profile"`
	Mode             string `json:"mode"`
	Provider         string `json:"provider"`
	Protocol         string `json:"protocol"`
	OriginalEndpoint string `json:"original_endpoint,omitempty"`
	RequestedModel   string `json:"requested_model,omitempty"`
	UpstreamModel    string `json:"upstream_model,omitempty"`
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

func Codex2APIPromptFilterAcceptsBodySize(size int64) bool {
	return size > 0 && size <= codex2APIPromptFilterMaxBodyBytes
}

// BuildBoundedCodex2APIPromptFilterBody creates a small, role-preserving
// preflight request from the request DTO that has already been parsed by the
// relay. It is used only when the original body exceeds the transport bound,
// so long history/tool payloads cannot bypass the sidecar while the newest
// current-user message remains enforceable.
func BuildBoundedCodex2APIPromptFilterBody(request dto.Request) ([]byte, error) {
	var envelope any
	switch typed := request.(type) {
	case *dto.OpenAIResponsesRequest:
		input, err := boundedResponsesInput(typed.Input)
		if err != nil {
			return nil, err
		}
		envelope = map[string]any{"model": typed.Model, "input": input}
	case *dto.OpenAIResponsesCompactionRequest:
		input, err := boundedResponsesInput(typed.Input)
		if err != nil {
			return nil, err
		}
		envelope = map[string]any{"model": typed.Model, "input": input}
	case *dto.GeneralOpenAIRequest:
		message, ok := latestOpenAIUserMessage(typed.Messages)
		if ok {
			envelope = map[string]any{"model": typed.Model, "messages": []any{boundedOpenAIMessage(message)}}
		} else {
			envelope = map[string]any{"model": typed.Model, "prompt": boundedPromptText(promptText(typed.Prompt), codex2APIPromptFilterCurrentUserBytes)}
		}
	case *dto.ClaudeRequest:
		message, ok := latestClaudeUserMessage(typed.Messages)
		if ok {
			envelope = map[string]any{"model": typed.Model, "messages": []any{boundedClaudeMessage(message)}}
		} else {
			envelope = map[string]any{"model": typed.Model, "prompt": boundedPromptText(typed.Prompt, codex2APIPromptFilterCurrentUserBytes)}
		}
	default:
		return nil, fmt.Errorf("unsupported oversized prompt-filter request type %T", request)
	}
	body, err := common.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshal bounded prompt-filter envelope: %w", err)
	}
	if len(body) == 0 || len(body) > codex2APIPromptFilterBoundedBodyBytes {
		return nil, fmt.Errorf("bounded prompt-filter envelope has invalid size %d", len(body))
	}
	return body, nil
}

func boundedResponsesInput(raw json.RawMessage) (any, error) {
	var input any
	if err := common.Unmarshal(raw, &input); err != nil {
		return nil, fmt.Errorf("parse oversized Responses input: %w", err)
	}
	switch value := input.(type) {
	case string:
		return []any{boundedResponsesMessage("user", value)}, nil
	case []any:
		if len(value) == 0 {
			return []any{}, nil
		}
		selected := value[len(value)-1]
		for idx := len(value) - 1; idx >= 0; idx-- {
			item, ok := value[idx].(map[string]any)
			if ok && strings.EqualFold(strings.TrimSpace(promptText(item["role"])), "user") {
				selected = item
				break
			}
		}
		role := "user"
		if item, ok := selected.(map[string]any); ok {
			if candidate := strings.TrimSpace(promptText(item["role"])); candidate != "" {
				role = candidate
			}
		}
		return []any{boundedResponsesMessage(role, extractPromptText(selected))}, nil
	case map[string]any:
		role := strings.TrimSpace(promptText(value["role"]))
		if role == "" {
			role = "user"
		}
		return []any{boundedResponsesMessage(role, extractPromptText(value))}, nil
	default:
		return []any{boundedResponsesMessage("user", extractPromptText(value))}, nil
	}
}

func boundedResponsesMessage(role string, text string) map[string]any {
	return map[string]any{
		"type": "message",
		"role": role,
		"content": []any{map[string]any{
			"type": "input_text",
			"text": boundedPromptText(text, codex2APIPromptFilterCurrentUserBytes),
		}},
	}
}

func latestOpenAIUserMessage(messages []dto.Message) (dto.Message, bool) {
	if len(messages) == 0 {
		return dto.Message{}, false
	}
	for idx := len(messages) - 1; idx >= 0; idx-- {
		if strings.EqualFold(strings.TrimSpace(messages[idx].Role), "user") {
			return messages[idx], true
		}
	}
	return messages[len(messages)-1], true
}

func boundedOpenAIMessage(message dto.Message) map[string]any {
	return map[string]any{
		"role":    message.Role,
		"content": boundedPromptText(extractPromptText(message.Content), codex2APIPromptFilterCurrentUserBytes),
	}
}

func latestClaudeUserMessage(messages []dto.ClaudeMessage) (dto.ClaudeMessage, bool) {
	if len(messages) == 0 {
		return dto.ClaudeMessage{}, false
	}
	for idx := len(messages) - 1; idx >= 0; idx-- {
		if strings.EqualFold(strings.TrimSpace(messages[idx].Role), "user") {
			return messages[idx], true
		}
	}
	return messages[len(messages)-1], true
}

func boundedClaudeMessage(message dto.ClaudeMessage) map[string]any {
	return map[string]any{
		"role":    message.Role,
		"content": boundedPromptText(extractPromptText(message.Content), codex2APIPromptFilterCurrentUserBytes),
	}
}

func extractPromptText(value any) string {
	var parts []string
	collectPromptText(value, &parts)
	return strings.Join(parts, "\n")
}

func collectPromptText(value any, parts *[]string) {
	switch typed := value.(type) {
	case nil:
		return
	case string:
		if strings.TrimSpace(typed) != "" {
			*parts = append(*parts, typed)
		}
	case []any:
		for _, item := range typed {
			collectPromptText(item, parts)
		}
	case map[string]any:
		preferred := []string{"text", "content", "output", "prompt", "input", "arguments"}
		found := false
		for _, key := range preferred {
			if item, ok := typed[key]; ok {
				collectPromptText(item, parts)
				found = true
			}
		}
		if found {
			return
		}
		for key, item := range typed {
			switch strings.ToLower(key) {
			case "role", "type", "id", "call_id", "tool_call_id", "name", "image_url", "file_data", "data", "source", "input_audio":
				continue
			}
			collectPromptText(item, parts)
		}
	default:
		data, err := common.Marshal(typed)
		if err == nil {
			*parts = append(*parts, string(data))
		}
	}
}

func promptText(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return extractPromptText(value)
}

func boundedPromptText(text string, maxBytes int) string {
	if maxBytes <= 0 || len(text) <= maxBytes {
		return text
	}
	const marker = "\n...[bounded preflight omitted middle content]...\n"
	available := maxBytes - len(marker)
	if available <= 0 {
		return marker[:maxBytes]
	}
	headBytes := available * 3 / 4
	tailBytes := available - headBytes
	for headBytes > 0 && !utf8.ValidString(text[:headBytes]) {
		headBytes--
	}
	tailStart := len(text) - tailBytes
	for tailStart < len(text) && !utf8.ValidString(text[tailStart:]) {
		tailStart++
	}
	return text[:headBytes] + marker + text[tailStart:]
}

// CheckCodex2APIPrompt runs the optional defense-in-depth sidecar before any
// billing reservation or upstream request. Transport errors and unverifiable
// responses fail open because the built-in NewAPI filter and upstream policy
// interception remain independent layers.
func CheckCodex2APIPrompt(c *gin.Context, rawBody []byte, model string, endpoint string) Codex2APIPromptFilterResult {
	if c == nil || !Codex2APIPromptFilterEnabled() || !Codex2APIPromptFilterAcceptsBodySize(int64(len(rawBody))) {
		return Codex2APIPromptFilterResult{}
	}

	filterURL := strings.TrimSpace(os.Getenv("CODEX2API_PROMPT_FILTER_URL"))
	secret := strings.TrimSpace(os.Getenv("CODEX2API_PROMPT_FILTER_SECRET"))
	parsedURL, err := url.Parse(filterURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		logger.LogWarn(c, "Codex2API prompt filter URL is invalid; request allowed by fail-open policy")
		return Codex2APIPromptFilterResult{}
	}

	requestID := strings.TrimSpace(c.GetString(common.RequestIdKey))
	if requestID == "" {
		requestID = newCodex2APIFilterRequestID()
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	bodyDigest := sha256.Sum256(rawBody)
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

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, filterURL, bytes.NewReader(rawBody))
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
	request.Header.Set("X-NewAPI-Prompt-Envelope", "raw-v1")
	setCodex2APIPromptFilterPolicyMeta(request.Header, secret, requestID, bodyDigestHex, model, endpoint)

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

func setCodex2APIPromptFilterPolicyMeta(header http.Header, secret string, requestID string, bodyDigest string, model string, endpoint string) {
	protocol, provider := codex2APIPromptFilterProtocol(endpoint)
	meta := codex2APIPromptFilterPolicyMeta{
		Profile:          codex2APIPromptFilterPolicyProfile(),
		Mode:             "enforce",
		Provider:         provider,
		Protocol:         protocol,
		OriginalEndpoint: strings.TrimSpace(endpoint),
		RequestedModel:   strings.TrimSpace(model),
		UpstreamModel:    strings.TrimSpace(model),
	}
	payload, err := common.Marshal(meta)
	if err != nil {
		return
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	canonical := strings.Join([]string{"policy-meta-v1", requestID, bodyDigest, encoded}, "\n")
	signature := hmacSHA256Hex(secret, canonical)
	if encoded == "" || signature == "" {
		return
	}
	header.Set("X-NewAPI-Policy-Meta", encoded)
	header.Set("X-NewAPI-Policy-Meta-Signature", signature)
}

func codex2APIPromptFilterPolicyProfile() string {
	profile := strings.ToLower(strings.TrimSpace(os.Getenv("CODEX2API_PROMPT_FILTER_PROFILE")))
	switch profile {
	case "balanced", "strict", "research":
		return profile
	default:
		return "balanced"
	}
}

func codex2APIPromptFilterProtocol(endpoint string) (string, string) {
	endpoint = strings.ToLower(strings.TrimSpace(endpoint))
	switch {
	case strings.Contains(endpoint, "/responses"):
		return "responses", "openai"
	case strings.Contains(endpoint, "/chat/completions"):
		return "chat", "openai"
	case strings.Contains(endpoint, "/messages"):
		return "messages", "anthropic"
	case strings.Contains(endpoint, "/images"):
		return "images", "openai"
	default:
		return "unknown", "unknown"
	}
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
