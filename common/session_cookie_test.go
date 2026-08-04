package common

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeOriginRejectsBroadCookieTrust(t *testing.T) {
	for _, raw := range []string{"https://*.example.com", "https://example.com/path", "https://user@example.com", "http://example.com"} {
		origin, err := NormalizeOrigin(raw)
		if raw == "http://example.com" {
			require.NoError(t, err)
			require.Equal(t, raw, origin)
			continue
		}
		require.Error(t, err, raw)
	}
}

func TestInitSessionCookieSettingsRequiresHttpsTrust(t *testing.T) {
	originalSecure, originalTrusted := os.Getenv("SESSION_COOKIE_SECURE"), os.Getenv("SESSION_COOKIE_TRUSTED_URL")
	t.Cleanup(func() {
		_ = os.Setenv("SESSION_COOKIE_SECURE", originalSecure)
		_ = os.Setenv("SESSION_COOKIE_TRUSTED_URL", originalTrusted)
		_ = InitSessionCookieSettings()
	})

	_ = os.Setenv("SESSION_COOKIE_SECURE", "true")
	_ = os.Setenv("SESSION_COOKIE_TRUSTED_URL", "http://example.com")
	require.Error(t, InitSessionCookieSettings())
	require.False(t, SessionCookieSecure)

	_ = os.Setenv("SESSION_COOKIE_TRUSTED_URL", "https://example.com, https://admin.example.com:443/")
	require.NoError(t, InitSessionCookieSettings())
	require.True(t, SessionCookieSecure)
	require.Equal(t, []string{"https://example.com", "https://admin.example.com"}, SessionCookieTrustedURLs)
}
