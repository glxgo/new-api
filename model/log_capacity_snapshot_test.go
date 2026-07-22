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
