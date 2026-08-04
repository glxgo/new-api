package common

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

// NormalizeOrigin canonicalizes a trusted browser origin. Paths, wildcards,
// credentials and query strings are rejected so this setting cannot broaden
// authentication-cookie trust accidentally.
func NormalizeOrigin(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "null") || strings.ContainsAny(raw, "\r\n") {
		return "", fmt.Errorf("origin is empty or invalid")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid origin: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("origin scheme must be http or https")
	}
	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", fmt.Errorf("origin must contain only scheme and host")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" || strings.Contains(host, "*") {
		return "", fmt.Errorf("origin host is empty or contains wildcard")
	}
	normalizedHost := host
	if strings.Contains(host, ":") {
		normalizedHost = "[" + host + "]"
	}
	port := parsed.Port()
	if port == "" || (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443") {
		return parsed.Scheme + "://" + normalizedHost, nil
	}
	return parsed.Scheme + "://" + net.JoinHostPort(host, port), nil
}

// InitSessionCookieSettings loads explicit cookie security settings. Secure
// cookies are opt-in so existing HTTP deployments remain reachable, while a
// trusted URL is mandatory whenever Secure is enabled.
func InitSessionCookieSettings() error {
	secureRaw := strings.TrimSpace(os.Getenv("SESSION_COOKIE_SECURE"))
	trustedRaw := strings.TrimSpace(os.Getenv("SESSION_COOKIE_TRUSTED_URL"))
	SessionCookieSecure = false
	SessionCookieTrustedURLs = nil

	if secureRaw == "" || strings.EqualFold(secureRaw, "false") {
		if trustedRaw != "" {
			return fmt.Errorf("SESSION_COOKIE_TRUSTED_URL requires SESSION_COOKIE_SECURE=true")
		}
		return nil
	}
	if !strings.EqualFold(secureRaw, "true") {
		return fmt.Errorf("SESSION_COOKIE_SECURE must be true or false")
	}
	if trustedRaw == "" {
		return fmt.Errorf("SESSION_COOKIE_SECURE=true requires SESSION_COOKIE_TRUSTED_URL")
	}
	for _, raw := range strings.Split(trustedRaw, ",") {
		origin, err := NormalizeOrigin(raw)
		if err != nil {
			return fmt.Errorf("invalid SESSION_COOKIE_TRUSTED_URL: %w", err)
		}
		if !strings.HasPrefix(origin, "https://") {
			return fmt.Errorf("SESSION_COOKIE_TRUSTED_URL must contain only https URLs with hosts")
		}
		SessionCookieTrustedURLs = append(SessionCookieTrustedURLs, origin)
	}
	SessionCookieSecure = true
	return nil
}
