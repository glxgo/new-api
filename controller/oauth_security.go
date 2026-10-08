package controller

import (
	"errors"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
	"slices"
)

type oauthFlow struct {
	UserID      int
	SessionHash string
	Provider    string
	Challenge   string
}

func issueOAuthState(c *gin.Context) (string, error) {
	flow := oauthFlow{Provider: c.Query("provider"), Challenge: c.Query("challenge")}
	provider := oauth.GetProvider(flow.Provider)
	if provider == nil || !provider.IsEnabled() {
		return "", model.ErrSecurityProof
	}
	if sessions.Default(c).Get("id") != nil {
		user, err := service.DashboardUser(c)
		if err != nil {
			return "", err
		}
		c.Set("id", user.Id)
		flow.UserID = user.Id
		flow.SessionHash, err = service.SecuritySessionHash(c)
		if err != nil {
			return "", err
		}
		if flow.Challenge != "" {
			proof, err := model.GetSecurityProof(flow.Challenge, user.Id, flow.SessionHash)
			if err != nil {
				return "", err
			}
			methods, err := service.AvailableSecurityMethods(user.Id, proof.Action)
			if err != nil {
				return "", err
			}
			if !service.SecurityMethodAllowed(methods, "oauth") || !slices.Contains(methods.OAuthProviders, flow.Provider) {
				return "", model.ErrSecurityProof
			}
		} else if !service.RequireSecurityProof(c, "account.bind.oauth") {
			return "", model.ErrSecurityProof
		}
	} else if flow.Challenge != "" {
		return "", model.ErrSecurityProof
	}
	payload, err := common.Marshal(flow)
	if err != nil {
		return "", err
	}
	return model.SaveSecurityCeremony(string(payload))
}

func consumeOAuthState(c *gin.Context) (*oauthFlow, error) {
	session := sessions.Default(c)
	state, _ := session.Get("oauth_state").(string)
	if state == "" || state != c.Query("state") {
		return nil, model.ErrSecurityProof
	}
	session.Delete("oauth_state")
	if err := session.Save(); err != nil {
		return nil, err
	}
	payload, err := model.PopSecurityCeremony(state)
	if err != nil {
		return nil, err
	}
	var flow oauthFlow
	if err := common.UnmarshalJsonStr(payload, &flow); err != nil {
		return nil, err
	}
	if flow.Provider != "" && flow.Provider != c.Param("provider") {
		return nil, model.ErrSecurityProof
	}
	if flow.UserID != 0 {
		user, err := service.DashboardUser(c)
		if err != nil || user.Id != flow.UserID {
			return nil, model.ErrSecurityProof
		}
		c.Set("id", user.Id)
		hash, err := service.SecuritySessionHash(c)
		if err != nil || hash != flow.SessionHash {
			return nil, model.ErrSecurityProof
		}
	}
	return &flow, nil
}

func finishOAuthProof(c *gin.Context, provider oauth.Provider, flow *oauthFlow) {
	if !provider.IsEnabled() {
		common.ApiError(c, model.ErrSecurityProof)
		return
	}
	token, err := provider.ExchangeToken(c.Request.Context(), c.Query("code"), c)
	if err != nil {
		handleOAuthError(c, err)
		return
	}
	external, err := provider.GetUserInfo(c.Request.Context(), token)
	if err != nil {
		handleOAuthError(c, err)
		return
	}
	var linked model.User
	if err := provider.FillUserByProviderID(&linked, external.ProviderUserID); err != nil || linked.Id != flow.UserID {
		common.ApiErrorMsg(c, "Please verify with the account already linked to this user")
		return
	}
	proof, err := model.GetSecurityProof(flow.Challenge, flow.UserID, flow.SessionHash)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	methods, err := service.AvailableSecurityMethods(flow.UserID, proof.Action)
	if err != nil || !slices.Contains(methods.OAuthProviders, flow.Provider) {
		common.ApiError(c, model.ErrSecurityProof)
		return
	}
	if err := model.VerifySecurityProof(flow.Challenge, flow.UserID, flow.SessionHash, "oauth"); err != nil {
		common.ApiError(c, err)
		return
	}
	recordUserSecurityAudit(c, flow.UserID, "user.oauth_verify", map[string]interface{}{"provider": flow.Provider})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"action": "verify"}})
}

func BeginOAuthProof(c *gin.Context) {
	state, err := issueOAuthState(c)
	if err != nil {
		if !c.IsAborted() {
			common.ApiError(c, err)
		}
		return
	}
	session := sessions.Default(c)
	session.Set("oauth_state", state)
	if err := session.Save(); err != nil {
		common.ApiError(c, err)
		return
	}
	target, err := oauthProofURL(c.Query("provider"), state)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"url": target}})
}

func oauthProofURL(slug, state string) (string, error) {
	provider := oauth.GetProvider(slug)
	if provider == nil || !provider.IsEnabled() {
		return "", model.ErrSecurityProof
	}
	var endpoint, client, scope string
	switch slug {
	case "github":
		endpoint, client, scope = "https://github.com/login/oauth/authorize", common.GitHubClientId, "user:email"
	case "discord":
		endpoint, client, scope = "https://discord.com/oauth2/authorize", system_setting.GetDiscordSettings().ClientId, "identify openid"
	case "oidc":
		cfg := system_setting.GetOIDCSettings()
		endpoint, client, scope = cfg.AuthorizationEndpoint, cfg.ClientId, "openid profile email"
	case "linuxdo":
		endpoint, client, scope = "https://connect.linux.do/oauth2/authorize", common.LinuxDOClientId, ""
	default:
		custom, ok := provider.(*oauth.GenericOAuthProvider)
		if !ok {
			return "", model.ErrSecurityProof
		}
		cfg := custom.GetConfig()
		endpoint, client, scope = cfg.AuthorizationEndpoint, cfg.ClientId, cfg.Scopes
	}
	target, err := url.Parse(endpoint)
	if err != nil || target.Host == "" || (target.Scheme != "https" && target.Scheme != "http") {
		return "", errors.New("Invalid OAuth authorization endpoint")
	}
	query := target.Query()
	query.Set("client_id", client)
	query.Set("state", state)
	query.Set("response_type", "code")
	query.Set("scope", scope)
	query.Set("redirect_uri", system_setting.ServerAddress+"/oauth/"+slug)
	// Request active reauthentication where supported. The callback must still match the linked identity.
	if slug == "oidc" {
		query.Set("prompt", "login")
		query.Set("max_age", "0")
	}
	target.RawQuery = query.Encode()
	return target.String(), nil
}

func OAuthProofStatus(c *gin.Context) {
	hash, err := service.SecuritySessionHash(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	proof, err := model.GetSecurityProof(c.GetHeader("X-Security-Challenge"), c.GetInt("id"), hash)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"verified": proof.Verified && proof.Method == "oauth"}})
}
