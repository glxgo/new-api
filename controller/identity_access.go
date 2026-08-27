package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type identityUpdateRequest struct {
	IdentityType string `json:"identity_type"`
}

type mainlandWhitelistRequest struct {
	// Username identifies the operator-granted enterprise/education account
	// for the unauthenticated 451 recovery form.
	Username string `json:"username" form:"username"`
	ReturnTo string `json:"return_to" form:"return_to"`
}

func identityResponse(identityType string) gin.H {
	canonical, _ := model.NormalizeIdentityType(identityType)
	return gin.H{
		"identity_type":  canonical,
		"identity_label": model.IdentityLabel(canonical),
		"is_enterprise":  canonical == model.IdentityTypeEnterprise,
		"is_education":   canonical == model.IdentityTypeEducation,
	}
}

// GetSelfIdentityAccess returns the server-authoritative identity and the
// user's active exception rows. It is intentionally behind UserAuth.
func GetSelfIdentityAccess(c *gin.Context) {
	userID := c.GetInt("id")
	identity, err := model.GetUserIdentity(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	rows, err := model.ListMainlandIPAllowlists(userID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		items = append(items, gin.H{
			"id":             row.ID,
			"address_family": row.AddressFamily,
			"prefix_length":  row.PrefixLength,
			"status":         row.Status,
			"source":         row.Source,
			"created_at":     row.CreatedAt,
			"last_seen_at":   row.LastSeenAt,
		})
	}
	data := identityResponse(identity.IdentityType)
	data["allowlists"] = items
	data["can_request"] = identity.IdentityType != model.IdentityTypeNone
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// UpdateUserIdentity is deliberately root-only: identity is an operator
// grant that controls access to a legal-region exception.
func UpdateUserIdentity(c *gin.Context) {
	if c.GetInt("role") != common.RoleRootUser {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "只有超级管理员可以管理企业/教育身份"})
		return
	}
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil || userID <= 0 {
		common.ApiErrorMsg(c, "用户 ID 无效")
		return
	}
	var req identityUpdateRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiErrorMsg(c, "请求内容无效")
		return
	}
	canonical, err := model.NormalizeIdentityType(req.IdentityType)
	if err != nil {
		common.ApiErrorMsg(c, "身份类型必须是无、企业或教育")
		return
	}
	if _, err := model.GetUserById(userID, false); err != nil {
		common.ApiError(c, err)
		return
	}
	previous, next, err := model.SetUserIdentity(userID, c.GetInt("id"), canonical)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, userID, "user.identity_update", map[string]interface{}{
		"id":       userID,
		"from":     previous,
		"to":       next,
		"identity": next,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": identityResponse(next)})
}

func addCurrentIPWhitelistForUser(c *gin.Context, userID int, req mainlandWhitelistRequest) (*model.MainlandIPAllowlist, error) {
	user, err := model.GetUserById(userID, false)
	if err != nil {
		return nil, err
	}
	if user.Status != common.UserStatusEnabled {
		return nil, fmt.Errorf("账号未启用")
	}
	// Keep the username check on the shared helper so authenticated and public
	// callers follow the same identity-confirmation rule.
	if strings.TrimSpace(req.Username) == "" || !strings.EqualFold(strings.TrimSpace(req.Username), user.Username) {
		// Do not reveal whether a different username exists.
		return nil, fmt.Errorf("用户名与身份信息不匹配")
	}
	ip := middleware.ClientRealIP(c)
	if ip == nil {
		return nil, fmt.Errorf("无法确定当前 IP 地址")
	}
	row, err := model.AddMainlandIPWhitelist(userID, userID, ip, model.MainlandIPAllowlistSourceSelf)
	if err != nil {
		return nil, err
	}
	recordUserSecurityAudit(c, userID, "mainland_ip_whitelist.self_add", map[string]interface{}{
		"address_family": row.AddressFamily,
		"ip_hash":        maskIPForAudit(ip),
	})
	return row, nil
}

// ApplyMainlandWhitelistFromBrowserSession restores the mainland exception
// for a dashboard browser session without requiring the user to submit the
// public username form again. UserAuth marks bearer access-token requests with
// use_access_token; those requests are intentionally excluded because this
// exception is only for the site's session cookie, never for API-key traffic.
//
// This helper is best-effort: a failed allowlist write must not make /self
// fail, otherwise an enterprise/education user could be trapped on the 451
// page because of a transient database or per-user capacity error.
func applyMainlandWhitelistFromBrowserSession(c *gin.Context, userID int) (string, error) {
	if c == nil || c.GetBool("use_access_token") || strings.TrimSpace(c.GetHeader("Authorization")) != "" || userID <= 0 {
		return "not_eligible", nil
	}
	identity, err := model.GetUserIdentity(userID)
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to load browser identity for mainland whitelist user %d: %v", userID, err))
		return "failed", err
	}
	canonical, err := model.NormalizeIdentityType(identity.IdentityType)
	if err != nil || (canonical != model.IdentityTypeEnterprise && canonical != model.IdentityTypeEducation) {
		return "not_eligible", nil
	}
	ip := middleware.ClientRealIP(c)
	if ip == nil {
		common.SysLog(fmt.Sprintf("failed to resolve browser IP for mainland whitelist user %d", userID))
		return "failed", fmt.Errorf("unable to resolve client IP")
	}
	_, lookupErr := model.GetMainlandIPAllowlistForUser(userID, ip)
	row, err := model.AddMainlandIPWhitelist(userID, userID, ip, model.MainlandIPAllowlistSourceBrowserSession)
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to auto-add mainland browser whitelist user %d: %v", userID, err))
		return "failed", err
	}
	if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		recordUserSecurityAudit(c, userID, "mainland_ip_whitelist.browser_session", map[string]interface{}{
			"address_family": row.AddressFamily,
			"ip_hash":        maskIPForAudit(ip),
		})
		return "added", nil
	}
	return "already_present", nil
}

// ApplyMainlandWhitelistFromBrowserSession is the best-effort hook used by
// the normal /api/user/self response after a dashboard session is restored.
func ApplyMainlandWhitelistFromBrowserSession(c *gin.Context, userID int) {
	_, _ = applyMainlandWhitelistFromBrowserSession(c, userID)
}

// AutoWhitelistFromBrowserSession is intentionally session-cookie-only. It
// has no UserAuth middleware because a mainland-blocked browser cannot be
// expected to provide the dashboard's New-Api-User header; API keys and bearer
// access tokens are never accepted here.
func AutoWhitelistFromBrowserSession(c *gin.Context) {
	if strings.TrimSpace(c.GetHeader("Authorization")) != "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "data": gin.H{"status": "not_eligible"}})
		return
	}
	session := sessions.Default(c)
	if session.Get("username") == nil || session.Get("id") == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "data": gin.H{"status": "not_eligible"}})
		return
	}
	status, statusOK := sessionInt(session.Get("status"))
	if !statusOK || status != common.UserStatusEnabled {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "data": gin.H{"status": "not_eligible"}})
		return
	}
	userID, ok := sessionInt(session.Get("id"))
	if !ok || userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "data": gin.H{"status": "not_eligible"}})
		return
	}
	c.Set("id", userID)
	whitelistStatus, err := applyMainlandWhitelistFromBrowserSession(c, userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "data": gin.H{"status": whitelistStatus}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"status": whitelistStatus}})
}

func sessionInt(value interface{}) (int, bool) {
	switch number := value.(type) {
	case int:
		return number, true
	case int8:
		return int(number), true
	case int16:
		return int(number), true
	case int32:
		return int(number), true
	case int64:
		return int(number), true
	case uint:
		return int(number), true
	case uint8:
		return int(number), true
	case uint16:
		return int(number), true
	case uint32:
		return int(number), true
	case uint64:
		return int(number), true
	case float64:
		return int(number), true
	default:
		return 0, false
	}
}

// ApplyMainlandWhitelist is the normal authenticated API endpoint used by the
// SPA. UserAuth guarantees that the caller cannot select another user.
func ApplyMainlandWhitelist(c *gin.Context) {
	var req mainlandWhitelistRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	row, err := addCurrentIPWhitelistForUser(c, c.GetInt("id"), req)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":             row.ID,
			"address_family": row.AddressFamily,
			"prefix_length":  row.PrefixLength,
			"status":         row.Status,
		},
	})
}

// ApplyMainlandWhitelistFromSession is used by the self-contained CTA form.
// It deliberately does not require a dashboard session: the browser reaches
// this page because mainland access was blocked, so login may be impossible.
// The submitted username is resolved server-side and must belong to an enabled
// enterprise/education account before the current IP is added.
func ApplyMainlandWhitelistFromSession(c *gin.Context) {
	var req mainlandWhitelistRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiErrorMsg(c, "请求内容无效")
		return
	}
	user, err := model.GetUserByUsername(req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = fmt.Errorf("用户名不存在或未配置企业/教育身份")
		}
		if strings.HasPrefix(c.GetHeader("Accept"), "text/html") {
			c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(mainlandWhitelistResultPage(err.Error(), req.ReturnTo)))
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	row, err := addCurrentIPWhitelistForUser(c, user.Id, req)
	if err != nil {
		if strings.HasPrefix(c.GetHeader("Accept"), "text/html") {
			c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(mainlandWhitelistResultPage(err.Error(), req.ReturnTo)))
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if strings.HasPrefix(c.GetHeader("Accept"), "text/html") {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(mainlandWhitelistResultPage("申请成功，当前 IP 已加入白名单。", req.ReturnTo)))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"id": row.ID, "status": row.Status}})
}

func GetMainlandWhitelistPage(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; form-action 'self'")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(mainlandWhitelistApplyPage(c.Query("return_to"))))
}

func ListMainlandAllowlists(c *gin.Context) {
	if c.GetInt("role") != common.RoleRootUser {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "只有超级管理员可以管理 IP 白名单"})
		return
	}
	userID, _ := strconv.Atoi(c.Query("user_id"))
	rows, err := model.ListMainlandIPAllowlists(userID, c.Query("include_revoked") == "true")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		item := gin.H{
			"id": row.ID, "user_id": row.UserID, "ip": row.IP,
			"address_family": row.AddressFamily, "prefix_length": row.PrefixLength,
			"identity_type": row.IdentityTypeSnapshot, "source": row.Source,
			"status": row.Status, "created_at": row.CreatedAt,
			"last_seen_at": row.LastSeenAt, "expires_at": row.ExpiresAt,
			"revoked_at": row.RevokedAt, "revoke_reason": row.RevokeReason,
		}
		if user, userErr := model.GetUserById(row.UserID, false); userErr == nil {
			item["username"] = user.Username
			item["email"] = user.Email
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

func RevokeMainlandAllowlist(c *gin.Context) {
	if c.GetInt("role") != common.RoleRootUser {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "只有超级管理员可以管理 IP 白名单"})
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "白名单 ID 无效")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = common.UnmarshalBodyReusable(c, &req)
	if err := model.RevokeMainlandIPAllowlist(id, c.GetInt("id"), req.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "mainland_ip_whitelist.revoke", map[string]interface{}{"id": id, "reason": strings.TrimSpace(req.Reason)})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func maskIPForAudit(ip interface{ String() string }) string {
	if ip == nil {
		return "unknown"
	}
	sum := sha256.Sum256([]byte(ip.String()))
	return hex.EncodeToString(sum[:])[:12]
}

func sanitizeMainlandReturnPath(raw string) string {
	decoded, err := url.QueryUnescape(strings.TrimSpace(raw))
	if err != nil || decoded == "" {
		return "/"
	}
	parsed, err := url.Parse(decoded)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(decoded, "//") {
		return "/"
	}
	return decoded
}

func mainlandWhitelistApplyPage(returnTo string) string {
	returnTo = sanitizeMainlandReturnPath(returnTo)
	return strings.Replace(`<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>企业/教育身份访问申请</title>
<style>body{margin:0;min-height:100vh;display:grid;place-items:center;background:#10131b;color:#eee;font-family:-apple-system,BlinkMacSystemFont,"PingFang SC","Microsoft YaHei",sans-serif}.card{width:min(560px,calc(100% - 40px));padding:34px;border:1px solid #d7b263;border-radius:20px;background:#171b25;box-shadow:0 20px 60px #0005}h1{margin:0 0 14px;font-size:26px}p{color:#b8bdca;line-height:1.7}label{display:block;margin:24px 0 8px;color:#d7b263}input{width:100%;padding:12px;border-radius:9px;border:1px solid #495266;background:#0e1118;color:#fff;box-sizing:border-box;font-size:16px}button{margin-top:18px;padding:12px 18px;border:0;border-radius:9px;background:#d7b263;color:#17130b;font-weight:700;cursor:pointer}small{display:block;margin-top:16px;color:#858da0}</style></head>
<body><main class="card"><h1>企业/教育身份访问申请</h1><p>无需登录。请输入管理员已标记为企业用户或教育用户的用户名，系统会自动将当前 IP 加入该用户的访问白名单。</p><form method="post" action="/api/access-policy/whitelist"><input type="hidden" name="return_to" value="__RETURN_TO__"><label for="username">用户名</label><input id="username" name="username" autocomplete="username" required><button type="submit">确认加入当前 IP</button></form><small>仅企业/教育身份用户可以申请；如果提交后仍无法访问，请联系管理员。</small></main></body></html>`, "__RETURN_TO__", html.EscapeString(returnTo), 1)
}

func mainlandWhitelistResultPage(message, returnTo string) string {
	returnTo = sanitizeMainlandReturnPath(returnTo)
	escapedReturnTo := html.EscapeString(returnTo)
	messageHTML := html.EscapeString(message)
	if strings.HasPrefix(message, "申请成功") {
		return fmt.Sprintf(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="refresh" content="0.8;url=%s"><title>申请结果</title><style>body{margin:0;min-height:100vh;display:grid;place-items:center;background:#10131b;color:#eee;font-family:-apple-system,BlinkMacSystemFont,"PingFang SC","Microsoft YaHei",sans-serif}.card{padding:34px;max-width:560px;border:1px solid #d7b263;border-radius:20px;background:#171b25}a{color:#d7b263}</style></head><body><main class="card"><h1>%s</h1><p>正在返回原页面并刷新… 如未自动跳转，请点击 <a href="%s">这里</a>。</p></main></body></html>`, escapedReturnTo, messageHTML, escapedReturnTo)
	}
	return fmt.Sprintf(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>申请结果</title><style>body{margin:0;min-height:100vh;display:grid;place-items:center;background:#10131b;color:#eee;font-family:-apple-system,BlinkMacSystemFont,"PingFang SC","Microsoft YaHei",sans-serif}.card{padding:34px;max-width:560px;border:1px solid #d7b263;border-radius:20px;background:#171b25}a{color:#d7b263}</style></head><body><main class="card"><h1>%s</h1><p>请返回 <a href="/api/access-policy/whitelist">申请页面</a> 重试。</p></main></body></html>`, messageHTML)
}
