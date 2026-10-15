package service

import (
	"bytes"
	"errors"
	"io"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const SecuritySessionKey = "security_session_id"

// DashboardUser validates the database-backed cookie on every authentication path.
func DashboardUser(c *gin.Context) (*model.User, error) {
	value, exists := c.Get(sessions.DefaultKey)
	if !exists {
		return nil, model.ErrSecurityProof
	}
	session, ok := value.(sessions.Session)
	if !ok || session == nil {
		return nil, model.ErrSecurityProof
	}
	id, _ := session.Get("id").(int)
	nonce, _ := session.Get(SecuritySessionKey).(string)
	return model.ValidateDashboardSession(id, nonce)
}

func SecuritySessionHash(c *gin.Context) (string, error) {
	if c.GetBool("use_access_token") {
		return common.GenerateHMAC("access-token-session:" + c.GetHeader("Authorization")), nil
	}
	session := sessions.Default(c)
	if session.Get("id") != c.GetInt("id") {
		return "", errors.New("Authentication required")
	}
	nonce, _ := session.Get(SecuritySessionKey).(string)
	if nonce == "" {
		return "", model.ErrSecurityProof
	}
	if _, err := DashboardUser(c); err != nil {
		return "", err
	}
	return model.SecurityDigest(nonce), nil
}

type SecurityMethods struct {
	Methods        []string `json:"methods"`
	OAuthProviders []string `json:"oauth_providers"`
}

func AvailableSecurityMethods(userID int, action string) (*SecurityMethods, error) {
	user, err := model.GetUserById(userID, true)
	if err != nil {
		return nil, err
	}
	if user.Status != common.UserStatusEnabled {
		return nil, errors.New("Account is disabled")
	}
	result := &SecurityMethods{Methods: []string{}, OAuthProviders: []string{}}
	twoFA, err := model.GetTwoFAByUserId(userID)
	if err != nil {
		return nil, err
	}
	if twoFA != nil && twoFA.IsEnabled {
		result.Methods = append(result.Methods, "2fa")
	}
	_, err = model.GetPasskeyByUserID(userID)
	if err != nil && !errors.Is(err, model.ErrPasskeyNotFound) {
		return nil, err
	}
	if err == nil {
		// An enrolled but unavailable factor must never enable password fallback.
		result.Methods = append(result.Methods, "passkey")
	}
	if len(result.Methods) != 0 || action == "channel.key.read" {
		return result, nil
	}
	if user.Password != "" && common.PasswordLoginEnabled {
		result.Methods = append(result.Methods, "password")
		return result, nil
	}
	bindings, err := model.GetUserOAuthBindingsByUserId(userID)
	if err != nil {
		return nil, err
	}
	for slug, provider := range oauth.GetAllProviders() {
		if !provider.IsEnabled() {
			continue
		}
		linked := false
		if custom, ok := provider.(*oauth.GenericOAuthProvider); ok {
			for _, binding := range bindings {
				if binding.ProviderId == custom.GetProviderId() {
					linked = true
				}
			}
		} else {
			switch slug {
			case "github":
				linked = user.GitHubId != ""
			case "discord":
				linked = user.DiscordId != ""
			case "oidc":
				linked = user.OidcId != ""
			case "linuxdo":
				linked = user.LinuxDOId != ""
			}
		}
		if linked {
			result.OAuthProviders = append(result.OAuthProviders, slug)
		}
	}
	if len(result.OAuthProviders) > 0 {
		result.Methods = append(result.Methods, "oauth")
	}
	return result, nil
}

func SecurityMethodAllowed(methods *SecurityMethods, method string) bool {
	for _, allowed := range methods.Methods {
		if method == allowed {
			return true
		}
	}
	return false
}

// Bind the complete request, including target, query and body. Hash with a
// server key so low-entropy passwords cannot be guessed from database digests.
func SecurityRequestHash(c *gin.Context) (string, error) {
	if hash, ok := c.Get("security_request_hash"); ok {
		return hash.(string), nil
	}
	var body []byte
	var err error
	if c.Request.Body != nil {
		body, err = io.ReadAll(io.LimitReader(c.Request.Body, 1<<20+1))
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		if len(body) > 1<<20 {
			return "", errors.New("Request body too large")
		}
	}
	hash := common.GenerateHMAC(c.Request.Method + "\n" + c.Request.URL.EscapedPath() + "\n" + c.Request.URL.Query().Encode() + "\n" + string(body))
	c.Set("security_request_hash", hash)
	return hash, nil
}
