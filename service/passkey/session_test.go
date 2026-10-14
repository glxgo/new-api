package passkey

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSecurityPasskeyCeremonyBindingAndReplay(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "passkey.db")), &gorm.Config{})
	require.NoError(t, err)
	old := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = old; sql, _ := db.DB(); sql.Close() })
	require.NoError(t, db.AutoMigrate(&model.SecurityCeremony{}))
	r := gin.New()
	r.Use(sessions.Sessions("ceremony", cookie.NewStore([]byte("test-cookie-key"))))
	r.GET("/begin", func(c *gin.Context) {
		s := sessions.Default(c)
		s.Set("security_session_id", "original")
		s.Set("passkey_operation_challenge", "proof-one")
		require.NoError(t, SaveSessionData(c, VerifySessionKey, &webauthn.SessionData{Challenge: "challenge-one"}))
		c.Status(200)
	})
	r.GET("/finish", func(c *gin.Context) {
		if value := c.GetHeader("Test-Session"); value != "" {
			sessions.Default(c).Set("security_session_id", value)
		}
		sessions.Default(c).Set("passkey_operation_challenge", "different-proof")
		data, err := PopSessionData(c, VerifySessionKey)
		if err != nil {
			c.Status(403)
			return
		}
		require.Equal(t, "proof-one", c.GetString("passkey_operation_challenge"))
		require.Equal(t, "challenge-one", data.Challenge)
		c.Status(200)
	})
	for _, crossSession := range []bool{false, true} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/begin", nil))
		original := w.Result().Cookies()[0]
		for attempt := 0; attempt < 2; attempt++ {
			req := httptest.NewRequest("GET", "/finish", nil)
			req.AddCookie(original)
			if crossSession {
				req.Header.Set("Test-Session", "other")
			}
			w = httptest.NewRecorder()
			r.ServeHTTP(w, req)
			want := 403
			if !crossSession && attempt == 0 {
				want = 200
			}
			require.Equal(t, want, w.Code)
		}
	}
}
