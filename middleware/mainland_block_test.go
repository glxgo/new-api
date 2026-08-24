package middleware

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/access_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func resetGeoBlockCountersForTest() {
	geoBlockTotal.Store(0)
	geoUnknownTotal.Store(0)
	geoLookupErrorTotal.Store(0)
	geoDecisionTotal.Store(0)
}

func resetAccessPolicyForTest() {
	policy := access_setting.GetAccessPolicy()
	policy.BlockMainlandWebAccess = false
	policy.IncludeHkMoTW = false
	policy.GeoIPUnknownPolicy = access_setting.GeoIPUnknownPolicyAllow
}

func newMainlandTestRouter(lookup countryLookupFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("mainland-test-secret"))))
	// Test-only role injection models the role written by authHelper after a
	// successful login, without requiring a database-backed login flow.
	r.Use(func(c *gin.Context) {
		if c.GetHeader("X-Test-Role") == "admin" {
			sessions.Default(c).Set("role", common.RoleAdminUser)
		}
		c.Next()
	})
	r.Use(mainlandWebAccessBlockWithLookup(lookup))
	ok := func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	}
	r.GET("/", ok)
	r.GET("/system-settings", ok)
	r.GET("/console", ok)
	r.GET("/assets/app.js", ok)
	r.GET("/api/test", ok)
	return r
}

func requestMainland(r *gin.Engine, path string) *httptest.ResponseRecorder {
	return requestMainlandWithHeaders(r, path, "203.0.113.10:12345", nil)
}

func requestMainlandWithHeaders(
	r *gin.Engine,
	path string,
	remoteAddr string,
	headers map[string]string,
) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = remoteAddr
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func setTrustedProxiesForTest(raw string) {
	trustedProxyCIDRs = parseProxyCIDRs(raw)
	trustedProxyOnce = sync.Once{}
	trustedProxyOnce.Do(func() {})
}

func TestMainlandWebAccessBlock(t *testing.T) {
	resetAccessPolicyForTest()
	resetGeoBlockCountersForTest()
	lookup := func(ip net.IP) (string, bool) {
		if ip.String() == "203.0.113.10" {
			return "CN", true
		}
		return "US", true
	}

	t.Run("switch off allows all", func(t *testing.T) {
		resetAccessPolicyForTest()
		r := newMainlandTestRouter(lookup)
		if w := requestMainland(r, "/"); w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("CN page blocked with 451 and copy", func(t *testing.T) {
		resetAccessPolicyForTest()
		resetGeoBlockCountersForTest()
		access_setting.GetAccessPolicy().BlockMainlandWebAccess = true
		r := newMainlandTestRouter(lookup)
		w := requestMainland(r, "/")
		if w.Code != http.StatusUnavailableForLegalReasons {
			t.Fatalf("expected 451, got %d", w.Code)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("expected no-store cache header")
		}
		if w.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
			t.Fatalf("expected noindex robots header")
		}
		body := w.Body.String()
		if !strings.Contains(body, "451") || !strings.Contains(body, "生成式人工智能服务管理暂行办法") {
			t.Fatalf("451 page copy missing")
		}
		if !strings.Contains(body, "width: min(800px, 100%)") ||
			!strings.Contains(body, `href="https://www.cac.gov.cn/2023-07/13/c_1690898327029107.htm"`) {
			t.Fatalf("451 page size or official legal link missing")
		}
		if geoBlockTotal.Load() != 1 {
			t.Fatalf("expected block counter 1, got %d", geoBlockTotal.Load())
		}
	})

	t.Run("non-CN page allowed", func(t *testing.T) {
		resetAccessPolicyForTest()
		access_setting.GetAccessPolicy().BlockMainlandWebAccess = true
		r := newMainlandTestRouter(func(ip net.IP) (string, bool) { return "US", true })
		if w := requestMainland(r, "/"); w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("HK not blocked", func(t *testing.T) {
		resetAccessPolicyForTest()
		access_setting.GetAccessPolicy().BlockMainlandWebAccess = true
		r := newMainlandTestRouter(func(ip net.IP) (string, bool) { return "HK", true })
		if w := requestMainland(r, "/"); w.Code != http.StatusOK {
			t.Fatalf("expected 200 for HK, got %d", w.Code)
		}
	})

	t.Run("unknown IP fail-open", func(t *testing.T) {
		resetAccessPolicyForTest()
		resetGeoBlockCountersForTest()
		access_setting.GetAccessPolicy().BlockMainlandWebAccess = true
		r := newMainlandTestRouter(func(ip net.IP) (string, bool) { return "", false })
		if w := requestMainland(r, "/"); w.Code != http.StatusOK {
			t.Fatalf("expected 200 for unknown IP, got %d", w.Code)
		}
		if geoUnknownTotal.Load() != 1 {
			t.Fatalf("expected unknown counter 1, got %d", geoUnknownTotal.Load())
		}
	})

	t.Run("admin page requires an admin session", func(t *testing.T) {
		resetAccessPolicyForTest()
		access_setting.GetAccessPolicy().BlockMainlandWebAccess = true
		r := newMainlandTestRouter(lookup)
		if w := requestMainland(r, "/system-settings"); w.Code != http.StatusUnavailableForLegalReasons {
			t.Fatalf("expected 451 for unauthenticated admin page, got %d", w.Code)
		}
		w := requestMainlandWithHeaders(r, "/system-settings", "203.0.113.10:12345", map[string]string{"X-Test-Role": "admin"})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for authenticated admin page, got %d", w.Code)
		}
	})

	t.Run("user console is not an admin exemption", func(t *testing.T) {
		resetAccessPolicyForTest()
		access_setting.GetAccessPolicy().BlockMainlandWebAccess = true
		r := newMainlandTestRouter(lookup)
		if w := requestMainland(r, "/console"); w.Code != http.StatusUnavailableForLegalReasons {
			t.Fatalf("expected 451 for user console, got %d", w.Code)
		}
	})

	t.Run("static asset exempt", func(t *testing.T) {
		resetAccessPolicyForTest()
		access_setting.GetAccessPolicy().BlockMainlandWebAccess = true
		r := newMainlandTestRouter(lookup)
		if w := requestMainland(r, "/assets/app.js"); w.Code != http.StatusOK {
			t.Fatalf("expected 200 for static asset, got %d", w.Code)
		}
	})

	t.Run("api path exempt", func(t *testing.T) {
		resetAccessPolicyForTest()
		access_setting.GetAccessPolicy().BlockMainlandWebAccess = true
		r := newMainlandTestRouter(lookup)
		if w := requestMainland(r, "/api/test"); w.Code != http.StatusOK {
			t.Fatalf("expected 200 for api path, got %d", w.Code)
		}
	})

	t.Run("unknown IP deny policy blocks", func(t *testing.T) {
		resetAccessPolicyForTest()
		access_setting.GetAccessPolicy().BlockMainlandWebAccess = true
		access_setting.GetAccessPolicy().GeoIPUnknownPolicy = access_setting.GeoIPUnknownPolicyDeny
		r := newMainlandTestRouter(func(ip net.IP) (string, bool) { return "", false })
		if w := requestMainland(r, "/"); w.Code != http.StatusUnavailableForLegalReasons {
			t.Fatalf("expected 451 for deny policy, got %d", w.Code)
		}
	})

	t.Run("include hk mo tw guard allows CN-listed hk", func(t *testing.T) {
		resetAccessPolicyForTest()
		access_setting.GetAccessPolicy().BlockMainlandWebAccess = true
		access_setting.GetAccessPolicy().IncludeHkMoTW = true
		r := newMainlandTestRouter(lookup)
		if w := requestMainland(r, "/"); w.Code != http.StatusOK {
			t.Fatalf("expected 200 when hk/mo/tw included, got %d", w.Code)
		}
	})

	t.Run("spoofed XFF from untrusted peer ignored", func(t *testing.T) {
		resetAccessPolicyForTest()
		setTrustedProxiesForTest("")
		access_setting.GetAccessPolicy().BlockMainlandWebAccess = true
		r := newMainlandTestRouter(lookup)
		w := requestMainlandWithHeaders(r, "/", "203.0.113.10:12345", map[string]string{
			"X-Forwarded-For": "198.51.100.7",
		})
		if w.Code != http.StatusUnavailableForLegalReasons {
			t.Fatalf("expected 451 using peer IP, got %d", w.Code)
		}
	})

	t.Run("trusted proxy header honored", func(t *testing.T) {
		resetAccessPolicyForTest()
		setTrustedProxiesForTest("127.0.0.1/32")
		access_setting.GetAccessPolicy().BlockMainlandWebAccess = true
		proxyLookup := func(ip net.IP) (string, bool) {
			switch ip.String() {
			case "198.51.100.7":
				return "US", true
			case "203.0.113.10":
				return "CN", true
			}
			return "US", true
		}
		r := newMainlandTestRouter(proxyLookup)
		w := requestMainlandWithHeaders(r, "/", "127.0.0.1:12345", map[string]string{
			"X-Forwarded-For": "198.51.100.7",
		})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for trusted-proxy US header, got %d", w.Code)
		}
		w2 := requestMainlandWithHeaders(r, "/", "127.0.0.1:12345", map[string]string{
			"X-Forwarded-For": "203.0.113.10",
		})
		if w2.Code != http.StatusUnavailableForLegalReasons {
			t.Fatalf("expected 451 for trusted-proxy CN header, got %d", w2.Code)
		}
	})

	resetAccessPolicyForTest()
	setTrustedProxiesForTest("")
}
