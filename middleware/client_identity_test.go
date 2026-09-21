package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestClientDenialDoesNotBanOrChargeUser(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:client-denial-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ClientIdentity{}, &model.ClientReview{}, &model.ClientGroupPolicy{}, &model.ClientGroupPolicyReview{}, &model.User{}, &model.Token{}, &model.Log{}))
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldRedis := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() { common.RedisEnabled = oldRedis })
	model.DB, model.LOG_DB = db, db
	t.Cleanup(func() { model.DB, model.LOG_DB = oldDB, oldLogDB; sqlDB, _ := db.DB(); _ = sqlDB.Close() })
	user := model.User{Id: 81, Username: "client-denial-test", Status: common.UserStatusEnabled, Quota: 10000}
	require.NoError(t, db.Create(&user).Error)
	token := model.Token{Id: 82, UserId: user.Id, Key: "local-only-client-test", Status: common.TokenStatusEnabled, RemainQuota: 10000}
	require.NoError(t, db.Create(&token).Error)
	var reached bool
	r := gin.New()
	r.Use(CaptureRequestClient(), func(c *gin.Context) {
		c.Set("id", user.Id)
		c.Set("token_id", token.Id)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "Coding")
		c.Request.Header.Set("User-Agent", "codex_cli_rs/1.0")
		c.Next()
	}, Distribute())
	r.POST("/v1/responses", func(c *gin.Context) { reached = true; c.Status(204) })
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "unknown-client/1.0")
	req.Header.Set("Authorization", "Bearer must-not-appear-in-log")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, 403, w.Code)
	require.Contains(t, w.Body.String(), "client_not_allowed")
	require.False(t, reached)
	var after model.User
	require.NoError(t, db.First(&after, user.Id).Error)
	require.Equal(t, common.UserStatusEnabled, after.Status)
	require.Equal(t, user.Quota, after.Quota)
	var afterToken model.Token
	require.NoError(t, db.First(&afterToken, token.Id).Error)
	require.Equal(t, token.Status, afterToken.Status)
	require.Equal(t, token.RemainQuota, afterToken.RemainQuota)
	var identities []model.ClientIdentity
	require.NoError(t, db.Find(&identities).Error)
	require.Len(t, identities, 1)
	require.Equal(t, "unknown", identities[0].Family)
	_, err = model.CheckClientGroupAccess(context.Background(), common.IdentifyClient("unknown-client/1.0"), "default")
	require.NoError(t, err)
	var logs []model.Log
	require.NoError(t, db.Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Contains(t, logs[0].Other, "client_access_denied")
	for _, log := range logs {
		require.Zero(t, log.Quota)
		require.NotContains(t, log.Other, "must-not-appear-in-log")
		require.NotContains(t, log.Other, "codex_cli_rs")
	}
}

func TestClientAdminDataRequiresAdministrator(t *testing.T) {
	for _, authenticated := range []bool{false, true} {
		response := performHeaderNavRequest(t, AdminAuth(), authenticated)
		require.NotContains(t, response.Body.String(), `"success":true`)
	}
}
