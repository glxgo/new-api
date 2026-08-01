package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestProcessChannelErrorPersistsRedactedResponsesInputIDDiagnostic(t *testing.T) {
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
	ctx.Set("username", "diagnostic-user")
	ctx.Set("token_name", "shared-key")
	ctx.Set("original_model", "gpt-test")
	ctx.Set("token_id", 1)
	ctx.Set("group", "plan")
	ctx.Set("use_channel", []string{"28"})

	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: json.RawMessage(`[{"type":"message","role":"assistant","id":"item_cde38a42cc60f24afb9e1dc2","content":"TOP_SECRET_CONTENT"}]`),
	}
	apiErr := types.WithOpenAIError(types.OpenAIError{
		Message: "Invalid 'input[0].id'",
		Type:    "invalid_request_error",
		Param:   "input[0].id",
		Code:    "invalid_value",
	}, http.StatusBadRequest)
	service.CaptureResponsesInputIDDiagnostic(ctx, req, apiErr)

	processChannelError(ctx, types.ChannelError{
		ChannelId:   28,
		ChannelType: 1,
		ChannelName: "cpa",
		AutoBan:     false,
	}, apiErr, true)

	var logEntry model.Log
	if err := db.Where("type = ?", model.LogTypeError).First(&logEntry).Error; err != nil {
		t.Fatalf("query error log: %v", err)
	}
	if strings.Contains(logEntry.Other, "TOP_SECRET_CONTENT") || strings.Contains(logEntry.Other, "item_cde38a42cc60f24afb9e1dc2") {
		t.Fatalf("admin diagnostic leaked request data: %s", logEntry.Other)
	}
	var other map[string]interface{}
	if err := common.UnmarshalJsonStr(logEntry.Other, &other); err != nil {
		t.Fatalf("unmarshal other: %v", err)
	}
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing admin info: %#v", other)
	}
	diagnostic, ok := adminInfo["responses_input_id_diagnostic"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing responses diagnostic: %#v", adminInfo)
	}
	if diagnostic["id_prefix"] != "item_" || diagnostic["item_type"] != "message" {
		t.Fatalf("unexpected responses diagnostic: %#v", diagnostic)
	}
}
