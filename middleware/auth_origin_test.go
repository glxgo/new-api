package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSessionCookieOriginGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldSecure, oldTrusted := common.SessionCookieSecure, common.SessionCookieTrustedURLs
	t.Cleanup(func() {
		common.SessionCookieSecure = oldSecure
		common.SessionCookieTrustedURLs = oldTrusted
	})
	common.SessionCookieSecure = true
	common.SessionCookieTrustedURLs = []string{"https://panel.example.com"}

	router := gin.New()
	router.GET("/logout", SessionCookieOriginGuard(), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "https://panel.example.com/logout", nil)
	req.Header.Set("Origin", "https://panel.example.com")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNoContent, resp.Code)

	req = httptest.NewRequest(http.MethodGet, "https://panel.example.com/logout", nil)
	req.Header.Set("Origin", "https://evil.example")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusForbidden, resp.Code)
}
