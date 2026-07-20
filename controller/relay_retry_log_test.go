package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestProcessChannelErrorSuppressesRetryAttemptUserLog(t *testing.T) {
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalErrorLogEnabled := constant.ErrorLogEnabled
	originalRedisEnabled := common.RedisEnabled
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		constant.ErrorLogEnabled = originalErrorLogEnabled
		common.RedisEnabled = originalRedisEnabled
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Log{}); err != nil {
		t.Fatalf("migrate log tables: %v", err)
	}
	model.DB = db
	model.LOG_DB = db
	constant.ErrorLogEnabled = true
	common.RedisEnabled = false

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	ctx.Set("id", 1)
	ctx.Set("username", "retry-user")
	ctx.Set("token_name", "shared-key")
	ctx.Set("original_model", "gpt-test")
	ctx.Set("token_id", 1)
	ctx.Set("group", "plan")
	ctx.Set("channel_id", 8)
	ctx.Set("channel_name", "krill")
	ctx.Set("channel_type", 1)

	apiErr := types.NewErrorWithStatusCode(errors.New("temporary upstream failure"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway)
	channelErr := types.ChannelError{ChannelId: 8, ChannelType: 1, ChannelName: "krill", AutoBan: false}

	processChannelError(ctx, channelErr, apiErr, false)
	var count int64
	if err := db.Model(&model.Log{}).Where("type = ?", model.LogTypeError).Count(&count).Error; err != nil {
		t.Fatalf("count retry logs: %v", err)
	}
	if count != 0 {
		t.Fatalf("intermediate retry failure must be admin-only, got %d user error logs", count)
	}
	adminInfo := map[string]interface{}{}
	service.AppendChannelRetryAttemptsAdminInfo(ctx, adminInfo)
	attempts, ok := adminInfo["retry_attempts"].([]map[string]interface{})
	if !ok || len(attempts) != 1 {
		t.Fatalf("expected one admin-only retry attempt, got %#v", adminInfo["retry_attempts"])
	}

	processChannelError(ctx, channelErr, apiErr, true)
	if err := db.Model(&model.Log{}).Where("type = ?", model.LogTypeError).Count(&count).Error; err != nil {
		t.Fatalf("count final logs: %v", err)
	}
	if count != 1 {
		t.Fatalf("final failure must create one user error log, got %d", count)
	}
}
