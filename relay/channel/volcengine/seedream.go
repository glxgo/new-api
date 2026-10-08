package volcengine

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func seedreamResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	fail := func(err error) (any, *types.NewAPIError) {
		return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway, types.ErrOptionWithSkipRetry())
	}
	if resp == nil || resp.Body == nil {
		return fail(errors.New("empty image response"))
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil {
		return fail(err)
	}
	if len(data) > 2<<20 {
		return fail(errors.New("image metadata response too large"))
	}
	var body map[string]any
	if err := common.Unmarshal(data, &body); err != nil {
		return fail(err)
	}
	if body["error"] != nil {
		return fail(errors.New("image generation failed"))
	}
	entries, ok := body["data"].([]any)
	if !ok || len(entries) == 0 || len(entries) > 17 {
		return fail(errors.New("invalid image output count"))
	}
	decoded := helper.SeedreamRequest(c)
	if decoded == nil {
		return fail(errors.New("missing image request state"))
	}
	facts, err := decoded.ActualUsage(c.Request.Context(), info.UpstreamModelName, body)
	if err != nil {
		// Invalid vendor usage must not refund a delivered image. Keep the frozen
		// estimate for expression settlement; per-call billing counts payloads below.
		common.SysError(fmt.Sprintf("Seedream usage unavailable; retaining reservation: %v", err))
		facts = nil
	}
	count := 0
	requestedFormat := c.GetString(helper.SeedreamResponseFormatKey)
	for _, entry := range entries {
		item, ok := entry.(map[string]any)
		if !ok {
			return fail(errors.New("invalid image output"))
		}
		url, _ := item["url"].(string)
		if url == "" {
			continue
		}
		count++
		if requestedFormat == "b64_json" {
			encoded, _, err := service.GetBase64Data(c, types.NewFileSourceFromData(url, ""), "Seedream image output")
			if err != nil {
				return fail(err)
			}
			item["b64_json"] = encoded
			delete(item, "url")
		}
	}
	if count == 0 {
		return fail(errors.New("image generation returned no images"))
	}
	helper.SetSeedreamActualUsage(info, facts)
	info.PriceData.AddOtherRatio("n", float64(count))
	data, err = common.Marshal(body)
	if err != nil {
		return fail(err)
	}
	copy := *resp
	copy.Body = io.NopCloser(bytes.NewReader(data))
	copy.ContentLength = int64(len(data))
	return openai.OpenaiImageHandler(c, info, &copy)
}
