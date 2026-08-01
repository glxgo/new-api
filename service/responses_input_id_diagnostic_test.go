package service

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func TestCaptureResponsesInputIDDiagnosticRedactsRequestContent(t *testing.T) {
	ctx, _ := gin.CreateTestContext(nil)
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: json.RawMessage(`[
			{"type":"message","role":"user","id":"msg_good","content":[{"type":"input_text","text":"TOP_SECRET_USER_TEXT"}]},
			{"type":"message","role":"assistant","id":"item_cde38a42cc60f24afb9e1dc2","content":[{"type":"output_text","text":"TOP_SECRET_ASSISTANT_TEXT"}]}
		]`),
		PreviousResponseID: "resp_sensitive_value",
		PromptCacheKey:     json.RawMessage(`"private-session-key"`),
	}
	apiErr := types.WithOpenAIError(types.OpenAIError{
		Message: "Invalid 'input[1].id'",
		Type:    "invalid_request_error",
		Param:   "input[1].id",
		Code:    "invalid_value",
	}, http.StatusBadRequest)

	CaptureResponsesInputIDDiagnostic(ctx, req, apiErr)

	adminInfo := map[string]interface{}{}
	AppendResponsesInputIDDiagnosticAdminInfo(ctx, adminInfo)
	diagnostic, ok := adminInfo["responses_input_id_diagnostic"].(ResponsesInputIDDiagnostic)
	if !ok {
		t.Fatalf("expected typed diagnostic, got %#v", adminInfo["responses_input_id_diagnostic"])
	}
	if diagnostic.InputIndex != 1 || diagnostic.InputCount != 2 {
		t.Fatalf("unexpected input location: %#v", diagnostic)
	}
	if diagnostic.ItemType != "message" || diagnostic.ItemRole != "assistant" {
		t.Fatalf("unexpected item metadata: %#v", diagnostic)
	}
	if diagnostic.IDPrefix != "item_" || diagnostic.IDLength != len("item_cde38a42cc60f24afb9e1dc2") {
		t.Fatalf("unexpected id metadata: %#v", diagnostic)
	}
	if diagnostic.PreviousResponseIDPrefix != "resp_" || diagnostic.PreviousResponseIDLength != len("resp_sensitive_value") {
		t.Fatalf("unexpected previous response metadata: %#v", diagnostic)
	}
	if len(diagnostic.PromptCacheKeyHash) != 12 {
		t.Fatalf("expected bounded prompt cache key hash, got %q", diagnostic.PromptCacheKeyHash)
	}

	encoded, err := common.Marshal(diagnostic)
	if err != nil {
		t.Fatalf("marshal diagnostic: %v", err)
	}
	formatted := FormatResponsesInputIDDiagnostic(ctx)
	for _, secret := range []string{
		"TOP_SECRET_USER_TEXT",
		"TOP_SECRET_ASSISTANT_TEXT",
		"item_cde38a42cc60f24afb9e1dc2",
		"resp_sensitive_value",
		"private-session-key",
	} {
		if strings.Contains(string(encoded), secret) || strings.Contains(formatted, secret) {
			t.Fatalf("diagnostic leaked %q: json=%s formatted=%s", secret, encoded, formatted)
		}
	}
}

func TestCaptureResponsesInputIDDiagnosticRequiresMatchingUpstreamParam(t *testing.T) {
	ctx, _ := gin.CreateTestContext(nil)
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: json.RawMessage(`[{"type":"message","role":"assistant","id":"item_bad","content":"secret"}]`),
	}

	CaptureResponsesInputIDDiagnostic(ctx, req, types.WithOpenAIError(types.OpenAIError{
		Message: "other bad request",
		Type:    "invalid_request_error",
		Param:   "input[0].content",
		Code:    "invalid_value",
	}, http.StatusBadRequest))
	adminInfo := map[string]interface{}{}
	AppendResponsesInputIDDiagnosticAdminInfo(ctx, adminInfo)
	if _, exists := adminInfo["responses_input_id_diagnostic"]; exists {
		t.Fatalf("unexpected diagnostic for unrelated param: %#v", adminInfo)
	}
}

func TestRecordChannelRetryAttemptIncludesResponsesInputIDDiagnostic(t *testing.T) {
	ctx, _ := gin.CreateTestContext(nil)
	req := &dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: json.RawMessage(`[{"type":"message","role":"assistant","id":"item_bad","content":"secret"}]`),
	}
	apiErr := types.WithOpenAIError(types.OpenAIError{
		Message: "Invalid 'input[0].id'",
		Type:    "invalid_request_error",
		Param:   "input[0].id",
		Code:    "invalid_value",
	}, http.StatusBadRequest)
	CaptureResponsesInputIDDiagnostic(ctx, req, apiErr)

	RecordChannelRetryAttempt(ctx, types.ChannelError{ChannelId: 28, ChannelName: "cpa", ChannelType: 1}, apiErr)
	adminInfo := map[string]interface{}{}
	AppendChannelRetryAttemptsAdminInfo(ctx, adminInfo)
	attempts, ok := adminInfo["retry_attempts"].([]map[string]interface{})
	if !ok || len(attempts) != 1 {
		t.Fatalf("unexpected retry attempts: %#v", adminInfo["retry_attempts"])
	}
	if _, ok := attempts[0]["responses_input_id_diagnostic"].(ResponsesInputIDDiagnostic); !ok {
		t.Fatalf("retry attempt missing typed diagnostic: %#v", attempts[0])
	}
}
