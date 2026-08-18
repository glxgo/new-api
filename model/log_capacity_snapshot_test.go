package model

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRecordConsumeLogPersistsUserCapacitySnapshot(t *testing.T) {
	truncateTables(t)
	user := &User{Username: "capacity-log-user", ConcurrencyLimit: 8}
	require.NoError(t, DB.Create(user).Error)

	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set("username", user.Username)
	common.SetContextKey(context, constant.ContextKeyUserConcurrency, 3)
	common.SetContextKey(context, constant.ContextKeyUserConcurrencyLimit, 8)
	common.SetContextKey(context, constant.ContextKeyUserRPM, 7)
	common.SetContextKey(context, constant.ContextKeyUserRPMLimit, 12)

	RecordConsumeLog(context, user.Id, RecordConsumeLogParams{
		ModelName: "capacity-test-model",
		Content:   "capacity snapshot test",
	})

	var log Log
	require.NoError(t, LOG_DB.Where("user_id = ?", user.Id).First(&log).Error)
	require.Equal(t, 3, log.UserConcurrency)
	require.Equal(t, 8, log.UserConcurrencyLimit)
	require.Equal(t, 7, log.UserRPM)
	require.Equal(t, 12, log.UserRPMLimit)
}

func TestFormatUserLogsKeepsUserCapacitySnapshot(t *testing.T) {
	logs := []*Log{{
		Id:                   99,
		ChannelName:          "admin-only-channel",
		UserConcurrency:      3,
		UserConcurrencyLimit: 8,
		UserRPM:              7,
		UserRPMLimit:         12,
		Other:                `{"admin_info":{"channel":8},"audit_info":{"operator":1},"stream_status":"completed","request_path":"/v1/responses","request_conversion":["OpenAI Responses","Claude Messages"],"is_model_mapped":true,"upstream_model_name":"gpt-5.6-terra","safe":"visible"}`,
	}}

	formatUserLogs(logs, 10)

	require.Equal(t, 11, logs[0].Id)
	require.Empty(t, logs[0].ChannelName)
	require.Equal(t, 3, logs[0].UserConcurrency)
	require.Equal(t, 8, logs[0].UserConcurrencyLimit)
	require.Equal(t, 7, logs[0].UserRPM)
	require.Equal(t, 12, logs[0].UserRPMLimit)
	other, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.Equal(t, "visible", other["safe"])
	require.Equal(t, "/v1/responses", other["request_path"])
	require.Equal(t, []interface{}{"OpenAI Responses", "Claude Messages"}, other["request_conversion"])
	require.Equal(t, true, other["is_model_mapped"])
	require.Equal(t, "gpt-5.6-terra", other["upstream_model_name"])
	require.NotContains(t, other, "admin_info")
	require.NotContains(t, other, "audit_info")
	require.NotContains(t, other, "stream_status")
}
