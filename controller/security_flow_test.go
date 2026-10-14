package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSecurityHTTPProofAndCookieRevocation(t *testing.T) {
	db := openTokenControllerTestDB(t)
	require.NoError(t, i18n.Init())
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.DashboardSession{}, &model.SecurityProof{}, &model.TwoFA{}, &model.PasskeyCredential{}, &model.Log{}))
	old := common.PasswordLoginEnabled
	common.PasswordLoginEnabled = true
	t.Cleanup(func() { common.PasswordLoginEnabled = old })
	hash, err := common.HashAccountPassword("test-password")
	require.NoError(t, err)
	user := model.User{Id: 7, Username: "proof-user", Password: hash, Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)
	router := gin.New()
	router.Use(sessions.Sessions("security-test", cookie.NewStore([]byte("test-session-key"))))
	router.GET("/session", func(c *gin.Context) {
		nonce, err := model.SecurityToken()
		require.NoError(t, err)
		require.NoError(t, model.CreateDashboardSession(&user, nonce))
		s := sessions.Default(c)
		s.Set("id", user.Id)
		s.Set("username", user.Username)
		s.Set("role", user.Role)
		s.Set("status", user.Status)
		s.Set(service.SecuritySessionKey, nonce)
		require.NoError(t, s.Save())
		c.Status(200)
	})
	router.POST("/verify", middleware.UserAuth(), UniversalVerify)
	router.GET("/logout", middleware.UserAuth(), Logout)
	writes := 0
	router.PUT("/target/:id", middleware.UserAuth(), service.SecurityProofRequired("account.test"), func(c *gin.Context) { writes++; c.JSON(200, gin.H{"success": true}) })
	request := func(method, path, body, proof string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("New-Api-User", "7")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if proof != "" {
			r.Header.Set("X-Security-Proof", proof)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	first := request("GET", "/session", "", "", nil).Result().Cookies()[0]
	second := request("GET", "/session", "", "", nil).Result().Cookies()[0]
	challenge := request("PUT", "/target/8", `{"role":10}`, "", first)
	require.Equal(t, 403, challenge.Code)
	token := gjson.GetBytes(challenge.Body.Bytes(), "verification.challenge").String()
	require.NotEmpty(t, token)
	body := `{"method":"password","password":"test-password","challenge":"` + token + `"}`
	require.False(t, gjson.GetBytes(request("POST", "/verify", body, "", second).Body.Bytes(), "success").Bool())
	require.True(t, gjson.GetBytes(request("POST", "/verify", body, "", first).Body.Bytes(), "success").Bool())
	for _, attempt := range [][2]string{{"/target/9", `{"role":10}`}, {"/target/8", `{"role":100}`}} {
		require.Equal(t, 403, request("PUT", attempt[0], attempt[1], token, first).Code)
	}
	require.Equal(t, 200, request("PUT", "/target/8", `{"role":10}`, token, first).Code)
	require.Equal(t, 403, request("PUT", "/target/8", `{"role":10}`, token, first).Code)
	require.Equal(t, 1, writes)
	require.True(t, gjson.GetBytes(request("GET", "/logout", "", "", first).Body.Bytes(), "success").Bool())
	require.Equal(t, 401, request("PUT", "/target/8", `{}`, "", first).Code)
	require.NoError(t, db.Model(&user).Update("status", common.UserStatusDisabled).Error)
	require.Equal(t, 401, request("PUT", "/target/8", `{}`, "", second).Code)
}
