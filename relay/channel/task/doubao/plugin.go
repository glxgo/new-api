package doubao

import (
	"context"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	doubaoplugin "github.com/QuantumNous/new-api/plugins/tasks/doubao"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func (a *TaskAdaptor) ValidateEffectiveRequest(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	_, err := a.pluginRequest(c, info)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_doubao_request", http.StatusBadRequest)
	}
	return nil
}

func (a *TaskAdaptor) pluginRequest(c *gin.Context, info *relaycommon.RelayInfo) ([]byte, error) {
	request, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	engine, err := doubaoplugin.Runtime()
	if err != nil {
		return nil, err
	}
	input := map[string]any{"model": info.OriginModelName, "upstreamModel": info.UpstreamModelName, "requestBody": request}
	var output struct {
		Body map[string]any `json:"body"`
	}
	if err := engine.CallInto(c.Request.Context(), &output, "buildSubmitRequest", input); err != nil {
		return nil, err
	}
	input["usagePurpose"] = "billing_ratios"
	var ratios map[string]float64
	if err := engine.CallInto(c.Request.Context(), &ratios, "extractUsage", input); err != nil {
		return nil, err
	}
	c.Set("doubao_plugin_ratios", ratios)
	// URLs and authorization remain host-owned; only the payload is used.
	return common.Marshal(output.Body)
}

func parsePluginTaskResult(data []byte) (*relaycommon.TaskInfo, error) {
	var body map[string]any
	if err := common.Unmarshal(data, &body); err != nil {
		return nil, err
	}
	engine, err := doubaoplugin.Runtime()
	if err != nil {
		return nil, err
	}
	var result struct {
		Status           string `json:"status"`
		Progress         string `json:"progress"`
		URL              string `json:"url"`
		Reason           string `json:"reason"`
		CompletionTokens int    `json:"completionTokens"`
		TotalTokens      int    `json:"totalTokens"`
	}
	if err := engine.CallInto(context.Background(), &result, "parseTaskResult", map[string]any{}, body); err != nil {
		return nil, err
	}
	return &relaycommon.TaskInfo{Status: result.Status, Progress: result.Progress, Url: result.URL, Reason: result.Reason, CompletionTokens: result.CompletionTokens, TotalTokens: result.TotalTokens}, nil
}
