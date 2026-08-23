package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/access_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/oschwald/maxminddb-golang"
)

// GeoBlockStats 中国大陆网页访问限制的可观测计数（进程内累计）。
type GeoBlockStats struct {
	BlockTotal       uint64 `json:"block_total"`
	UnknownTotal     uint64 `json:"unknown_total"`
	LookupErrorTotal uint64 `json:"lookup_error_total"`
	DecisionTotal    uint64 `json:"decision_total"`
}

var (
	geoBlockTotal       atomic.Uint64
	geoUnknownTotal     atomic.Uint64
	geoLookupErrorTotal atomic.Uint64
	geoDecisionTotal    atomic.Uint64
)

// countryLookupFunc 注入式国家查询，便于单元测试。
type countryLookupFunc func(ip net.IP) (country string, ok bool)

var (
	countryDBMu     sync.RWMutex
	countryDBReader *maxminddb.Reader
	countryDBPath   string
)

var (
	trustedProxyOnce  sync.Once
	trustedProxyCIDRs []*net.IPNet
)

// trustedProxies 返回可信代理 CIDR 列表。默认信任本机回环（生产 Nginx 与 new-api
// 同机）；可通过 GEOIP_TRUSTED_PROXIES 环境变量覆盖（逗号分隔 CIDR）。
func trustedProxies() []*net.IPNet {
	trustedProxyOnce.Do(func() {
		raw := os.Getenv("GEOIP_TRUSTED_PROXIES")
		if strings.TrimSpace(raw) == "" {
			raw = "127.0.0.1/32,::1/128"
		}
		trustedProxyCIDRs = parseProxyCIDRs(raw)
	})
	return trustedProxyCIDRs
}

func parseProxyCIDRs(raw string) []*net.IPNet {
	var cidrs []*net.IPNet
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, cidr, err := net.ParseCIDR(part); err == nil {
			cidrs = append(cidrs, cidr)
		}
	}
	return cidrs
}

func isTrustedProxy(ip net.IP) bool {
	for _, cidr := range trustedProxies() {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// clientRealIP 解析真实客户端 IP：仅当直连对端属于可信代理时才采信
// X-Forwarded-For / X-Real-IP（取第一个合法 IP，与既有 ClientIP 语义一致），
// 否则一律使用直连 IP，防止伪造 Header 绕过或误伤（PRD v0.4 防伪造要求）。
func clientRealIP(c *gin.Context) net.IP {
	peer := net.ParseIP(hostOnly(c.Request.RemoteAddr))
	if peer == nil {
		return nil
	}
	if !isTrustedProxy(peer) {
		return peer
	}
	for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
		for _, part := range strings.Split(c.GetHeader(header), ",") {
			if ip := net.ParseIP(strings.TrimSpace(part)); ip != nil {
				return ip
			}
		}
	}
	return peer
}

func hostOnly(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return strings.TrimSpace(remoteAddr)
}

// adminPagePrefixes 后台管理页面路径前缀（仅已认证管理员会话豁免，PRD v0.4）。
// 注意：/keys、/models、/dashboard、/wallet、/chat、/playground 等用户页面仍受网页限制约束。
var adminPagePrefixes = []string{
	"/system-settings",
	"/users",
	"/channels",
	"/redemption-codes",
	"/topup-coupons",
	"/withdraw-review",
	"/lucky-wheel-admin",
	"/api-ingress",
	"/virtual-memberships",
	"/profit",
	"/usage-statistics",
	"/admin",
}

// authPagePrefixes stay reachable from the self-contained 451 page so a
// mainland user can sign in before requesting an identity-based exception.
// They do not grant access to authenticated application pages themselves.
var authPagePrefixes = []string{
	"/login",
	"/sign-in",
	"/register",
	"/sign-up",
	"/forgot-password",
	"/reset-password",
	"/access-policy",
}

func isAdminPagePath(path string) bool {
	for _, prefix := range adminPagePrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

// hasAdminSession keeps the web-page exemption tied to an authenticated
// administrator.  The middleware runs before SPA routing, so a pathname such
// as /usage-statistics or /system-settings must not become a mainland bypass
// for an ordinary user (or an unauthenticated browser).  The global session
// middleware is present in production; the defensive context lookup also
// keeps standalone middleware tests and alternate router setups fail-closed.
func hasAdminSession(c *gin.Context) bool {
	if c == nil {
		return false
	}
	if role, exists := c.Get("role"); exists {
		if roleValue, ok := sessionRoleInt(role); ok && roleValue >= common.RoleAdminUser {
			return true
		}
	}
	value, exists := c.Get(sessions.DefaultKey)
	if !exists {
		return false
	}
	session, ok := value.(sessions.Session)
	if !ok || session == nil {
		return false
	}
	role, ok := sessionRoleInt(session.Get("role"))
	return ok && role >= common.RoleAdminUser
}

func sessionRoleInt(value interface{}) (int, bool) {
	switch role := value.(type) {
	case int:
		return role, true
	case int8:
		return int(role), true
	case int16:
		return int(role), true
	case int32:
		return int(role), true
	case int64:
		return int(role), true
	case uint:
		return int(role), true
	case uint8:
		return int(role), true
	case uint16:
		return int(role), true
	case uint32:
		return int(role), true
	case uint64:
		return int(role), true
	case float64:
		return int(role), true
	default:
		return 0, false
	}
}

func isAuthPagePath(path string) bool {
	for _, prefix := range authPagePrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

// isStaticAssetRequestPath 静态资源默认放行（PRD v0.4：静态资源默认 allow，待评审）。
func isStaticAssetRequestPath(path string) bool {
	if path == "/favicon.ico" {
		return true
	}
	if strings.HasPrefix(path, "/assets/") || strings.HasPrefix(path, "/uploads/") || strings.HasPrefix(path, "/static/") {
		return true
	}
	lastSegment := path[strings.LastIndex(path, "/")+1:]
	if dot := strings.LastIndex(lastSegment, "."); dot > 0 && dot < len(lastSegment)-1 {
		return true
	}
	return false
}

// MainlandWebAccessBlock 网页请求地区限制中间件。
// 仅挂载在 Web 路由（非 /api、/v1），按 PRD v0.4 执行：
// 开关开启 + 网页请求 + GeoIP=CN → 451；API / 管理页 / 静态资源 / 未知 IP 放行。
func MainlandWebAccessBlock() gin.HandlerFunc {
	return mainlandWebAccessBlockWithLookup(nil)
}

// ClientRealIP exposes the same trusted-proxy resolution used by the web
// restriction.  Controllers must not parse X-Forwarded-For independently.
func ClientRealIP(c *gin.Context) net.IP {
	return clientRealIP(c)
}

func mainlandWebAccessBlockWithLookup(lookup countryLookupFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !access_setting.IsBlockMainlandWebAccess() {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/v1/") || path == "/api" || path == "/v1" {
			c.Next()
			return
		}
		if (isAdminPagePath(path) && hasAdminSession(c)) || isAuthPagePath(path) || isStaticAssetRequestPath(path) {
			c.Next()
			return
		}
		ip := clientRealIP(c)
		if ip == nil {
			geoUnknownTotal.Add(1)
			geoDecisionTotal.Add(1)
			recordGeoUnknown("unparsable_ip")
			if !access_setting.UnknownPolicyAllows() {
				serveMainlandBlocked(c, net.IP{})
				return
			}
			c.Next()
			return
		}
		// Identity exceptions are checked before GeoIP.  The table stores only
		// exact server-resolved addresses and cannot be populated by a caller's
		// submitted XFF value.
		if model.IsMainlandIPWhitelisted(ip) {
			c.Next()
			return
		}
		country, ok := lookupCountry(ip, lookup)
		geoDecisionTotal.Add(1)
		if !ok || country == "" {
			geoUnknownTotal.Add(1)
			recordGeoUnknown("lookup_failed")
			if !access_setting.UnknownPolicyAllows() {
				serveMainlandBlocked(c, ip)
				return
			}
			c.Next()
			return
		}
		policy := access_setting.GetAccessPolicy()
		if country == "CN" && !policy.IncludeHkMoTW {
			geoBlockTotal.Add(1)
			serveMainlandBlocked(c, ip)
			return
		}
		c.Next()
	}
}

func recordGeoUnknown(reason string) {
	common.SysLog(fmt.Sprintf("mainland web access unknown ip reason=%s", reason))
}

func lookupCountry(ip net.IP, lookup countryLookupFunc) (string, bool) {
	if lookup != nil {
		return lookup(ip)
	}
	return defaultCountryLookup(ip)
}

func defaultCountryLookup(ip net.IP) (string, bool) {
	reader := loadCountryDBReader()
	if reader == nil {
		geoLookupErrorTotal.Add(1)
		return "", false
	}
	var record struct {
		Country struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
	}
	if err := reader.Lookup(ip, &record); err != nil {
		geoLookupErrorTotal.Add(1)
		return "", false
	}
	if record.Country.ISOCode == "" {
		return "", false
	}
	return record.Country.ISOCode, true
}

func loadCountryDBReader() *maxminddb.Reader {
	path := access_setting.GetGeoIPDBPath()
	countryDBMu.RLock()
	reader := countryDBReader
	loadedPath := countryDBPath
	countryDBMu.RUnlock()
	if reader != nil && loadedPath == path {
		return reader
	}
	countryDBMu.Lock()
	defer countryDBMu.Unlock()
	if countryDBReader != nil && countryDBPath == path {
		return countryDBReader
	}
	if countryDBReader != nil {
		_ = countryDBReader.Close()
		countryDBReader = nil
	}
	r, err := maxminddb.Open(path)
	if err != nil {
		countryDBPath = path
		common.SysLog("geoip country database unavailable path=" + path + " error=" + err.Error())
		return nil
	}
	countryDBReader = r
	countryDBPath = path
	common.SysLog("geoip country database loaded path=" + path)
	return r
}

// GeoIPDBLoaded 是否已成功加载 GeoIP 数据库。
func GeoIPDBLoaded() bool {
	countryDBMu.RLock()
	defer countryDBMu.RUnlock()
	return countryDBReader != nil
}

// GeoBlockStatsSnapshot 返回进程内累计计数。
func GeoBlockStatsSnapshot() GeoBlockStats {
	return GeoBlockStats{
		BlockTotal:       geoBlockTotal.Load(),
		UnknownTotal:     geoUnknownTotal.Load(),
		LookupErrorTotal: geoLookupErrorTotal.Load(),
		DecisionTotal:    geoDecisionTotal.Load(),
	}
}

func serveMainlandBlocked(c *gin.Context, ip net.IP) {
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Language", "zh-CN")
	c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Robots-Tag", "noindex, nofollow")
	c.Header("Content-Type", "text/html; charset=utf-8")
	body := []byte(mainlandBlockedPage)
	c.Header("Content-Length", strconv.Itoa(len(body)))
	common.SysLog(fmt.Sprintf("mainland web access blocked path_category=web country=CN masked_ip=%s", maskIP(ip)))
	c.Data(http.StatusUnavailableForLegalReasons, "text/html; charset=utf-8", body)
	c.Abort()
}

// maskIP 只记录 IP 哈希前缀，不落完整 IP。
func maskIP(ip net.IP) string {
	if ip == nil || len(ip) == 0 {
		return "unknown"
	}
	sum := sha256.Sum256([]byte(ip.String()))
	return hex.EncodeToString(sum[:])[:12]
}
