package channel

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestClientRevocationStopsNextOutboundAttempt(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:outbound-client-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ClientIdentity{}, &model.ClientReview{}, &model.ClientGroupPolicy{}, &model.ClientGroupPolicyReview{}))
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB; sqlDB, _ := db.DB(); _ = sqlDB.Close() })
	ctx := context.Background()
	s := common.IdentifyClient("codex_cli_rs/1.0")
	require.NoError(t, model.ObserveClient(ctx, s))
	require.NoError(t, model.ReviewClient(ctx, s.ClientKey, "approved", "test approval", 1, 0))
	require.NoError(t, model.SaveClientGroupPolicy(ctx, model.ClientGroupPolicy{GroupName: "Coding", IsCoding: true, SupportedClients: []string{s.ClientKey}}, "test support", 1))
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer upstream.Close()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("User-Agent", s.UserAgent)
	common.CaptureClientSnapshot(c)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "Coding")
	c.Request.Header.Set("User-Agent", "Go-http-client/1.1")
	req, err := http.NewRequest(http.MethodPost, upstream.URL, nil)
	require.NoError(t, err)
	req.Body = http.NoBody
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	resp, err := DoRequest(c, req, info)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.EqualValues(t, 1, calls.Load())
	require.NoError(t, model.ReviewClient(ctx, s.ClientKey, "pending", "revoke before retry", 1, 1))
	resp, err = DoRequest(c, req, info)
	require.Nil(t, resp)
	var denied *types.NewAPIError
	require.ErrorAs(t, err, &denied)
	require.Equal(t, types.ErrorCode("client_not_allowed"), denied.GetErrorCode())
	require.Equal(t, http.StatusForbidden, denied.StatusCode)
	require.True(t, types.IsSkipRetryError(denied))
	require.EqualValues(t, 1, calls.Load(), "revoked retry must not contact upstream")
	taskErr := service.TaskErrorWrapper(fmt.Errorf("wrapped: %w", err), "do_request_failed", 500)
	require.Equal(t, "client_not_allowed", taskErr.Code)
	require.True(t, taskErr.LocalError)
	require.Equal(t, 403, taskErr.StatusCode)
	oldAuto := setting.AutoGroups2JsonString()
	t.Cleanup(func() { _ = setting.UpdateAutoGroupsByJsonString(oldAuto) })
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["Coding"]`))
	common.SetContextKey(c, constant.ContextKeyUserGroup, "Coding")
	_, _, autoErr := service.CacheGetRandomSatisfiedChannel(&service.RetryParam{Ctx: c, TokenGroup: "auto", Retry: common.GetPointer(0)})
	require.ErrorAs(t, autoErr, &denied)
	require.Equal(t, types.ErrorCode("client_not_allowed"), denied.GetErrorCode())
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	resp, err = DoRequest(c, req, info)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.EqualValues(t, 2, calls.Load(), "ordinary group remains usable")
}
