package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"net/http"
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
	// Username is only a consistency confirmation for a logged-in session. It
	// is never used to look up or authorize a different account.
	Username string `json:"username" form:"username"`
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
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "only the super administrator can manage identity"})
		return
	}
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil || userID <= 0 {
		common.ApiErrorMsg(c, "invalid user id")
		return
	}
	var req identityUpdateRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	canonical, err := model.NormalizeIdentityType(req.IdentityType)
	if err != nil {
		common.ApiErrorMsg(c, "identity_type must be none, enterprise, or education")
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
		return nil, fmt.Errorf("account is not enabled")
	}
	// The username is only a confirmation of the already-authenticated
	// principal. Require it for the public CTA so a browser cannot silently
	// submit a stale/ambiguous form, while never using it for lookup or
	// authorization.
	if strings.TrimSpace(req.Username) == "" || !strings.EqualFold(strings.TrimSpace(req.Username), user.Username) {
		// Do not reveal whether a different username exists.
		return nil, fmt.Errorf("logged-in identity confirmation failed")
	}
	ip := middleware.ClientRealIP(c)
	if ip == nil {
		return nil, fmt.Errorf("unable to determine client IP")
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

func sessionUserID(c *gin.Context) (int, bool) {
	value := sessions.Default(c).Get("id")
	switch id := value.(type) {
	case int:
		return id, id > 0
	case int64:
		return int(id), id > 0
	case float64:
		return int(id), id > 0
	default:
		return 0, false
	}
}

// ApplyMainlandWhitelistFromSession is used by the self-contained CTA form.
// It intentionally requires an existing dashboard session; a username alone
// can never authorize an allowlist entry.
func ApplyMainlandWhitelistFromSession(c *gin.Context) {
	userID, ok := sessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "请先登录后再申请 IP 白名单"})
		return
	}
	var req mainlandWhitelistRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	row, err := addCurrentIPWhitelistForUser(c, userID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if strings.HasPrefix(c.GetHeader("Accept"), "text/html") {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(mainlandWhitelistResultPage("申请成功，当前 IP 已加入白名单。")))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"id": row.ID, "status": row.Status}})
}

func GetMainlandWhitelistPage(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; form-action 'self'")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(mainlandWhitelistApplyPage))
}

func ListMainlandAllowlists(c *gin.Context) {
	userID, _ := strconv.Atoi(c.Query("user_id"))
	rows, err := model.ListMainlandIPAllowlists(userID, c.Query("include_revoked") == "true")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		items = append(items, gin.H{
			"id": row.ID, "user_id": row.UserID, "address_family": row.AddressFamily,
			"prefix_length": row.PrefixLength, "source": row.Source, "status": row.Status,
			"created_at": row.CreatedAt, "last_seen_at": row.LastSeenAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

func RevokeMainlandAllowlist(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid allowlist id")
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

const mainlandWhitelistApplyPage = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>企业/教育身份访问申请</title>
<style>body{margin:0;min-height:100vh;display:grid;place-items:center;background:#10131b;color:#eee;font-family:-apple-system,BlinkMacSystemFont,"PingFang SC","Microsoft YaHei",sans-serif}.card{width:min(560px,calc(100% - 40px));padding:34px;border:1px solid #d7b263;border-radius:20px;background:#171b25;box-shadow:0 20px 60px #0005}h1{margin:0 0 14px;font-size:26px}p{color:#b8bdca;line-height:1.7}label{display:block;margin:24px 0 8px;color:#d7b263}input{width:100%;padding:12px;border-radius:9px;border:1px solid #495266;background:#0e1118;color:#fff;box-sizing:border-box;font-size:16px}button{margin-top:18px;padding:12px 18px;border:0;border-radius:9px;background:#d7b263;color:#17130b;font-weight:700;cursor:pointer}small{display:block;margin-top:16px;color:#858da0}</style></head>
<body><main class="card"><h1>企业/教育身份访问申请</h1><p>请先登录站点。申请只会把服务器解析出的当前 IP 记录到你的账号白名单，不接受手工输入 IP 或仅凭用户名授权。</p><form method="post" action="/api/access-policy/whitelist"><label for="username">登录用户名（用于确认当前会话）</label><input id="username" name="username" autocomplete="username" required><button type="submit">确认加入当前 IP</button></form><small>如果提交后仍无法访问，请联系管理员确认企业/教育身份。</small></main></body></html>`

func mainlandWhitelistResultPage(message string) string {
	return fmt.Sprintf(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>申请结果</title><style>body{margin:0;min-height:100vh;display:grid;place-items:center;background:#10131b;color:#eee;font-family:-apple-system,BlinkMacSystemFont,"PingFang SC","Microsoft YaHei",sans-serif}.card{padding:34px;max-width:560px;border:1px solid #d7b263;border-radius:20px;background:#171b25}a{color:#d7b263}</style></head><body><main class="card"><h1>%s</h1><p>返回 <a href="/">站点首页</a>。</p></main></body></html>`, html.EscapeString(message))
}
