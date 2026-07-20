package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func TestGetSelfQuotaForResponseCCSwitchIncludesGiftQuota(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self", nil)
	ctx.Request.Header.Set("User-Agent", "cc-switch/1.0")
	user := &model.User{Quota: 498054, GiftQuota: 1946}

	if got := getSelfQuotaForResponse(ctx, user); got != 500000 {
		t.Fatalf("CC Switch quota = %d, want total spendable quota 500000", got)
	}
}

func TestGetSelfQuotaForResponseBrowserKeepsPrincipalQuota(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self", nil)
	ctx.Request.Header.Set("User-Agent", "Mozilla/5.0")
	user := &model.User{Quota: 498054, GiftQuota: 1946}

	if got := getSelfQuotaForResponse(ctx, user); got != 498054 {
		t.Fatalf("browser quota = %d, want principal quota 498054", got)
	}
}
