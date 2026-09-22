package controller

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/capabilitytest"
	"github.com/QuantumNous/new-api/relay"
	channeladaptor "github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type capabilityPrepared struct {
	context       *gin.Context
	info          *relaycommon.RelayInfo
	adaptor       channeladaptor.Adaptor
	body          []byte
	writer        *capabilityWriter
	fingerprint   string
	requestHash   string
	channel       *model.Channel
	endpointLabel string
}
type capabilityWriter struct {
	header   http.Header
	body     bytes.Buffer
	status   int
	overflow bool
}

func (w *capabilityWriter) Header() http.Header    { return w.header }
func (w *capabilityWriter) WriteHeader(status int) { w.status = status }
func (w *capabilityWriter) Flush()                 {}
func (w *capabilityWriter) Write(p []byte) (int, error) {
	if w.body.Len()+len(p) > 2<<20 {
		w.overflow = true
		return 0, errors.New("response_too_large")
	}
	return w.body.Write(p)
}

func prepareCapability(ctx context.Context, target service.CapabilityTarget, p model.CapabilityProfile, prompt string, imageData string, ch *model.Channel, key string, slot int, secret []byte) (*capabilityPrepared, error) {
	if key == "" {
		return nil, errors.New("credential_unavailable")
	}
	w := &capabilityWriter{header: make(http.Header), status: 200}
	c, _ := gin.CreateTestContext(w)
	path := "/v1/chat/completions"
	format := types.RelayFormatOpenAI
	stream := false
	if p.Protocol == "responses" {
		path = "/v1/responses"
		format = types.RelayFormatOpenAIResponses
	}
	c.Request = httptest.NewRequest(http.MethodPost, path, nil).WithContext(ctx)
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, target.Group)
	common.SetContextKey(c, constant.ContextKeyUserGroup, target.Group)
	common.SetContextKey(c, constant.ContextKeyTokenGroup, target.Group)
	if e := middleware.SetupContextForSelectedChannelKey(c, ch, target.Model, key, slot); e != nil {
		return nil, errors.New("channel_context_failed")
	}
	// Client-dependent/wildcard headers cannot be reproduced by an internal
	// probe. Fail before sending instead of measuring a different request.
	for k, v := range ch.GetHeaderOverride() {
		if k == "*" || strings.HasPrefix(k, "re:") || strings.HasPrefix(k, "regex:") || strings.Contains(fmt.Sprint(v), "{client_header:") {
			return nil, errors.New("client_context_required")
		}
	}
	var req dto.Request
	if p.Protocol == "responses" {
		input, _ := common.Marshal(prompt)
		if imageData != "" {
			input, _ = common.Marshal([]any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": prompt}, map[string]any{"type": "input_image", "image_url": imageData}}}})
		}
		req = &dto.OpenAIResponsesRequest{Model: target.Model, Input: input, Stream: &stream, MaxOutputTokens: &p.MaxTokens}
	} else {
		var content any = prompt
		if imageData != "" {
			content = []any{map[string]any{"type": "text", "text": prompt}, map[string]any{"type": "image_url", "image_url": map[string]any{"url": imageData}}}
		}
		req = &dto.GeneralOpenAIRequest{Model: target.Model, Messages: []dto.Message{{Role: "user", Content: content}}, Stream: &stream, MaxTokens: &p.MaxTokens}
	}
	info, err := relaycommon.GenRelayInfo(c, format, req, nil)
	if err != nil {
		return nil, errors.New("relay_context_failed")
	}
	info.IsChannelTest = true
	info.InitChannelMeta(c)
	if err = helper.ModelMappedHelper(c, info, req); err != nil {
		return nil, errors.New("model_mapping_failed")
	}
	req.SetModelName(info.UpstreamModelName)
	apiType, _ := common.ChannelType2APIType(ch.Type)
	adaptor := relay.GetAdaptor(apiType)
	if adaptor == nil {
		return nil, errors.New("protocol_unsupported")
	}
	adaptor.Init(info)
	var converted any
	if r, ok := req.(*dto.OpenAIResponsesRequest); ok {
		converted, err = adaptor.ConvertOpenAIResponsesRequest(c, info, *r)
	} else {
		converted, err = adaptor.ConvertOpenAIRequest(c, info, req.(*dto.GeneralOpenAIRequest))
	}
	if err != nil {
		return nil, errors.New("protocol_conversion_failed")
	}
	body, err := common.Marshal(converted)
	if err != nil {
		return nil, err
	}
	if len(info.ParamOverride) > 0 {
		body, err = relaycommon.ApplyParamOverrideWithRelayInfo(body, info)
		if err != nil {
			return nil, errors.New("parameter_override_failed")
		}
	}
	// Bound effective output even when production overrides replace parameters.
	limitFound := false
	for _, name := range []string{"max_tokens", "max_completion_tokens", "max_output_tokens", "generationConfig.maxOutputTokens", "generation_config.max_output_tokens"} {
		v := gjson.GetBytes(body, name)
		if v.Exists() {
			if v.Type != gjson.Number || v.Float() != float64(v.Int()) || v.Int() < 1 || v.Int() > int64(p.MaxTokens) {
				return nil, errors.New("output_limit_outside_profile")
			}
			limitFound = true
		}
	}
	if !limitFound {
		return nil, errors.New("output_limit_not_verifiable")
	}
	if n := gjson.GetBytes(body, "n"); n.Exists() && n.Int() != 1 {
		return nil, errors.New("multiple_completions_not_supported")
	}
	for _, name := range []string{"tools", "functions"} {
		if v := gjson.GetBytes(body, name); v.Exists() && v.Raw != "[]" && v.Raw != "null" {
			return nil, errors.New("tools_not_allowed")
		}
	}
	endpoint, err := adaptor.GetRequestURL(info)
	if err != nil {
		return nil, errors.New("endpoint_invalid")
	}
	headers := make(http.Header)
	if err = adaptor.SetupRequestHeader(c, &headers, info); err != nil {
		return nil, errors.New("headers_invalid")
	}
	// HMAC covers secrets without persisting credentials or endpoints. Include
	// context conservatively when overrides exist; only proven equivalence
	// is eligible for cross-group sharing.
	contextGroup := ""
	if len(info.ParamOverride) > 0 || len(ch.GetHeaderOverride()) > 0 {
		contextGroup = target.Group
	}
	signature, _ := common.Marshal([]any{ch.Id, endpoint, headers, ch.GetHeaderOverride(), body, key, slot, ch.GetSetting(), ch.GetOtherSettings(), contextGroup})
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(signature)
	fingerprint := fmt.Sprintf("%x", mac.Sum(nil))
	sum := sha256.Sum256(body)
	label := ""
	if parsed, e := url.Parse(endpoint); e == nil {
		label = parsed.Scheme + "://" + parsed.Host
	}
	return &capabilityPrepared{context: c, info: info, adaptor: adaptor, body: body, writer: w, fingerprint: fingerprint, requestHash: fmt.Sprintf("%x", sum), channel: ch, endpointLabel: label}, nil
}

type capabilityResponse struct {
	Answer        string
	Usage         string
	UpstreamModel string
	Reason        string
	ReportedModel string
	RequestID     string
}

func (p *capabilityPrepared) execute() capabilityResponse {
	result := capabilityResponse{UpstreamModel: p.info.UpstreamModelName}
	p.context.Request.Body = io.NopCloser(bytes.NewReader(p.body))
	resp, err := p.adaptor.DoRequest(p.context, p.info, bytes.NewReader(p.body))
	if err != nil {
		result.Reason = "upstream_outcome_unknown"
		return result
	}
	httpResp, ok := resp.(*http.Response)
	if !ok || httpResp == nil {
		result.Reason = "response_missing"
		return result
	}
	defer httpResp.Body.Close()
	// Headers/model declarations are upstream claims, not identity attestation.
	for _, name := range []string{"x-request-id", "request-id", "x-amzn-requestid"} {
		if value := httpResp.Header.Get(name); len(value) > 0 && len(value) <= 256 {
			result.RequestID = value
			break
		}
	}
	if httpResp.StatusCode != 200 {
		result.Reason = fmt.Sprintf("upstream_http_%d", httpResp.StatusCode)
		return result
	}
	// Bound the raw response before invoking adapters that may read it all.
	raw, err := io.ReadAll(io.LimitReader(httpResp.Body, 2<<20+1))
	if err != nil {
		result.Reason = "response_interrupted"
		return result
	}
	if len(raw) > 2<<20 {
		result.Reason = "response_too_large"
		return result
	}
	httpResp.Body = io.NopCloser(bytes.NewReader(raw))
	if declared := gjson.GetBytes(raw, "model").String(); len(declared) <= 191 {
		result.ReportedModel = declared
	}
	if strings.Contains(httpResp.Header.Get("Content-Type"), "text/event-stream") {
		p.info.IsStream = true
	}
	_, apiErr := p.adaptor.DoResponse(p.context, httpResp, p.info)
	if apiErr != nil {
		result.Reason = "response_conversion_failed"
		return result
	}
	if p.writer.overflow {
		result.Reason = "response_too_large"
		return result
	}
	answer, usage, err := capabilitytest.ParseResponse(p.writer.body.Bytes(), p.info.RelayFormat == types.RelayFormatOpenAIResponses)
	if err != nil {
		result.Reason = err.Error()
		return result
	}
	result.Answer = answer
	_ = usage
	// Preserve the provider's own usage fields, never adapter-derived estimates.
	result.Usage = capabilitytest.ReportedUsage(raw)
	return result
}
