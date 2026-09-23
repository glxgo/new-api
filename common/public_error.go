package common

import (
	"bytes"
	"encoding/json"
	"html"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

var (
	publicErrorURL           = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|//)[^\s<>"'\\]+`)
	publicErrorIP            = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}(?::[0-9]+)?\b`)
	publicErrorDomain        = regexp.MustCompile(`[\p{L}\p{N}][\p{L}\p{N}-]*(?:\.[\p{L}\p{N}][\p{L}\p{N}-]*)+(?::[0-9]+)?`)
	publicErrorLocalhost     = regexp.MustCompile(`(?i)\blocalhost(?::[0-9]+)?\b`)
	publicErrorIPv6          = regexp.MustCompile(`\[?[0-9a-fA-F]*:[0-9a-fA-F:.]+(?:%[a-zA-Z0-9_-]+)?\]?(?::[0-9]+)?`)
	publicErrorLookup        = regexp.MustCompile(`(?i)\b(lookup|dial tcp|dial udp)\s+[^\s:]+(?::[0-9]+)?`)
	publicErrorBearer        = regexp.MustCompile(`(?i)\bBearer\s+[^\s,"';}]+`)
	publicErrorCredential    = regexp.MustCompile(`(?i)\b(authorization|api[_-]?key|access[_-]?token|bearer)([\s:="']+)[^\s,"';}]+`)
	publicErrorURLDetail     = regexp.MustCompile(`(?i)(?:^|[\s,，;；])url\s*[:：=]\s*["']?\[redacted\]`)
	publicErrorPercentEscape = regexp.MustCompile(`%[0-9a-fA-F]{2}`)
	publicErrorUnicodeEscape = regexp.MustCompile(`\\(?:u[0-9a-fA-F]{4}|x[0-9a-fA-F]{2})`)
)

func normalizePublicErrorEncoding(message string) string {
	for i := 0; i < 4; i++ {
		decoded := html.UnescapeString(message)
		decoded = publicErrorUnicodeEscape.ReplaceAllStringFunc(decoded, func(escape string) string {
			value, err := strconv.Unquote(`"` + escape + `"`)
			if err != nil {
				return escape
			}
			return value
		})
		decoded = publicErrorPercentEscape.ReplaceAllStringFunc(decoded, func(escape string) string {
			value, _ := url.PathUnescape(escape)
			return value
		})
		decoded = strings.ReplaceAll(decoded, `\/`, `/`)
		decoded = strings.Map(func(r rune) rune {
			if unicode.Is(unicode.Cf, r) {
				return -1
			}
			return r
		}, decoded)
		if decoded == message {
			return message
		}
		message = decoded
	}
	// Deeply encoded diagnostics are not safe to reflect. Keep work bounded.
	return "Upstream request failed"
}

// SanitizePublicError removes network endpoints from diagnostic text. It must
// only be applied to errors, never model output, prompts or successful media URLs.
// Unlike log masking, the public representation retains no host suffix or port.
func SanitizePublicError(message string) string {
	message = normalizePublicErrorEncoding(message)
	message = publicErrorURL.ReplaceAllString(message, "[redacted]")
	message = publicErrorIPv6.ReplaceAllStringFunc(message, func(candidate string) string {
		host := candidate
		if addr, err := netip.ParseAddrPort(candidate); err == nil {
			host = addr.Addr().String()
		}
		if addr, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil && addr.Is6() {
			return "[redacted]"
		}
		return candidate
	})
	message = publicErrorIP.ReplaceAllString(message, "[redacted]")
	message = publicErrorLocalhost.ReplaceAllString(message, "[redacted]")
	message = publicErrorDomain.ReplaceAllStringFunc(message, func(host string) string {
		name, _, _ := strings.Cut(host, ":")
		asciiName, err := idna.Lookup.ToASCII(name)
		if err != nil {
			return "[redacted]"
		}
		suffix, icann := publicsuffix.PublicSuffix(strings.ToLower(asciiName))
		if icann || suffix == "internal" || suffix == "local" || suffix == "localhost" || suffix == "test" || suffix == "invalid" || suffix == "example" || suffix == "lan" || suffix == "corp" || suffix == "svc" {
			return "[redacted]"
		}
		return host
	})
	message = publicErrorLookup.ReplaceAllString(message, "${1} [redacted]")
	message = publicErrorBearer.ReplaceAllString(message, "Bearer [redacted]")
	message = publicErrorCredential.ReplaceAllString(message, "${1}${2}[redacted]")
	// Gateway diagnostics append URL and tracing details after the actual error.
	// Public messages show only that leading error, without a URL placeholder.
	if detail := publicErrorURLDetail.FindStringIndex(message); detail != nil {
		message = strings.TrimRight(message[:detail[0]], " \t\r\n,，;；:：")
		if message == "" {
			return "Upstream request failed"
		}
	}
	return message
}

// SanitizeErrorJSON rewrites only protocol error fields. RawMessage keeps large
// sequence numbers, usage and successful output byte-exact. Normal frames are
// returned unchanged. Call at the outbound boundary, after internal accounting.
func SanitizeErrorJSON(data []byte) []byte {
	if trimmed := bytes.TrimSpace(data); len(trimmed) > 0 && trimmed[0] == '[' {
		var items []json.RawMessage
		if Unmarshal(data, &items) != nil {
			return data
		}
		changed := false
		for i, raw := range items {
			clean := SanitizeErrorJSON(raw)
			if !bytes.Equal(raw, clean) {
				items[i] = clean
				changed = true
			}
		}
		if changed {
			if result, err := Marshal(items); err == nil {
				return result
			}
		}
		return data
	}
	var fields map[string]json.RawMessage
	if Unmarshal(data, &fields) != nil || fields == nil {
		return data
	}
	changed := false
	var eventType string
	_ = Unmarshal(fields["type"], &eventType)
	var status string
	_ = Unmarshal(fields["status"], &status)
	failed := strings.EqualFold(status, "failed") || strings.EqualFold(status, "failure")
	isErrorEvent := eventType == "error" || strings.HasSuffix(eventType, ".failed") || strings.HasSuffix(eventType, ".error")
	var success bool
	businessFailure := Unmarshal(fields["success"], &success) == nil && bytes.Equal(bytes.TrimSpace(fields["success"]), []byte("false"))
	hasError := false
	for _, key := range []string{"error", "errors"} {
		raw := bytes.TrimSpace(fields[key])
		if len(raw) > 0 && !bytes.Equal(raw, []byte("null")) && !bytes.Equal(raw, []byte(`""`)) && !bytes.Equal(raw, []byte("false")) {
			hasError = true
		}
	}
	var base struct {
		StatusCode int `json:"status_code"`
	}
	providerFailure := Unmarshal(fields["base_resp"], &base) == nil && base.StatusCode != 0
	errorEnvelope := failed || isErrorEvent || businessFailure || hasError || providerFailure
	for key, raw := range fields {
		if errorEnvelope && isPublicErrorEndpointKey(key) {
			delete(fields, key)
			changed = true
			continue
		}
		var replacement []byte
		switch {
		case key == "error" || key == "errors" || key == "error_message" || key == "error_msg" || key == "error_type" || key == "incomplete_details":
			replacement = SanitizeErrorValueJSON(raw)
		case (key == "fail_reason" || key == "failReason") && !strings.EqualFold(status, "success"):
			replacement = SanitizeErrorValueJSON(raw)
		case errorEnvelope && (key == "metadata" || key == "message" || key == "reason" || key == "detail" || key == "description"):
			replacement = SanitizeErrorValueJSON(raw)
		case key == "base_resp":
			var base struct {
				StatusCode int `json:"status_code"`
			}
			if Unmarshal(raw, &base) != nil || base.StatusCode == 0 {
				continue
			}
			replacement = SanitizeErrorValueJSON(raw)
		case key == "data" && (businessFailure || failed || providerFailure || hasError):
			replacement = SanitizeErrorValueJSON(raw)
		case key == "response" || key == "status_details" || key == "data":
			replacement = SanitizeErrorJSON(raw)
		case errorEnvelope && key != "output" && key != "choices" && key != "content" && key != "usage":
			replacement = SanitizeErrorValueJSON(raw)
		default:
			continue
		}
		if !bytes.Equal(raw, replacement) {
			fields[key] = replacement
			changed = true
		}
	}
	if !changed {
		return data
	}
	result, err := Marshal(fields)
	if err != nil {
		return []byte(`{"error":{"message":"Upstream request failed","type":"upstream_error"}}`)
	}
	return result
}

func isPublicErrorEndpointKey(key string) bool {
	key = strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key))
	switch key {
	case "url", "uri", "baseurl", "requesturl", "upstreamurl", "endpoint", "upstreamhost", "host", "address", "resulturl":
		return true
	}
	return false
}

// SanitizeErrorValueJSON handles arbitrary provider extensions inside an error,
// including nested metadata and JSON encoded as a string, without mutating input.
func SanitizeErrorValueJSON(data []byte) []byte {
	var value any
	// Decode into raw fields/arrays below to avoid rounding numeric error codes.
	var fields map[string]json.RawMessage
	if Unmarshal(data, &fields) == nil && fields != nil {
		result := make(map[string]json.RawMessage, len(fields))
		for key, raw := range fields {
			if isPublicErrorEndpointKey(key) {
				continue
			}
			cleanKey := SanitizePublicError(key)
			switch strings.ToLower(key) {
			case "authorization", "api_key", "api-key", "access_token", "token", "secret", "password":
				result[cleanKey] = json.RawMessage(`"[redacted]"`)
			default:
				result[cleanKey] = SanitizeErrorValueJSON(raw)
			}
		}
		encoded, err := Marshal(result)
		if err == nil {
			return encoded
		}
	}
	var items []json.RawMessage
	if Unmarshal(data, &items) == nil && items != nil {
		for i := range items {
			items[i] = SanitizeErrorValueJSON(items[i])
		}
		encoded, err := Marshal(items)
		if err == nil {
			return encoded
		}
	}
	if Unmarshal(data, &value) == nil {
		if message, ok := value.(string); ok {
			encoded, err := Marshal(SanitizePublicError(message))
			if err == nil {
				return encoded
			}
		}
		return data
	}
	return []byte(`"Upstream request failed"`)
}

// SanitizeHTTPErrorBody also covers HTML/plain-text gateway errors. An error
// response is never a successful model payload and can be sanitized in full.
func SanitizeHTTPErrorBody(data []byte) []byte {
	var fields map[string]json.RawMessage
	if Unmarshal(data, &fields) == nil && fields != nil {
		return SanitizeErrorValueJSON(data)
	}
	return []byte(`{"error":{"message":"Upstream request failed","type":"upstream_error"}}`)
}
