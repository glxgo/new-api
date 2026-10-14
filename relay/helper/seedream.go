package helper

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/pkg/seedream"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
)

const seedreamContext = "seedream_request"
const SeedreamResponseFormatKey = "seedream_response_format"

func IsSeedreamImage(c *gin.Context, info *relaycommon.RelayInfo) bool {
	_, image := info.Request.(*dto.ImageRequest)
	return image && common.GetContextKeyInt(c, constant.ContextKeyChannelType) == constant.ChannelTypeVolcEngine
}

// PrepareSeedreamImage validates the effective provider request before reserving
// quota. The plugin has no network capabilities; transport stays in the adapter.
func PrepareSeedreamImage(c *gin.Context, info *relaycommon.RelayInfo) (map[string]any, error) {
	operation := "generate"
	if info.RelayMode == relayconstant.RelayModeImagesEdits {
		operation = "edit"
	}
	var input map[string]any
	if form := c.Request.MultipartForm; form != nil {
		fields := form.Value
		files := []map[string]any{}
		uploads := map[string]string{}
		for field, headers := range form.File {
			for _, header := range headers {
				if len(files) >= 14 || header.Size > 30<<20 {
					return nil, errors.New("too many or oversized reference images")
				}
				file, err := header.Open()
				if err != nil {
					return nil, err
				}
				data, err := io.ReadAll(io.LimitReader(file, (30<<20)+1))
				file.Close()
				if err != nil {
					return nil, err
				}
				if len(data) > 30<<20 {
					return nil, errors.New("reference image exceeds 30 MiB")
				}
				mime := strings.ToLower(header.Header.Get("Content-Type"))
				if !strings.HasPrefix(mime, "image/") {
					return nil, errors.New("reference must be an image")
				}
				ref := fmt.Sprint(len(files))
				uploads[ref] = "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
				files = append(files, map[string]any{"field": field, "ref": ref, "mimeType": mime})
			}
		}
		decoded, err := seedream.Decode(c.Request.Context(), info.UpstreamModelName, operation, map[string]any{"kind": "multipart", "fields": fields, "files": files})
		if err != nil {
			return nil, err
		}
		if images, ok := decoded.Body["image"].([]any); ok {
			for i, item := range images {
				if ref, ok := item.(map[string]any); ok {
					images[i] = uploads[fmt.Sprint(ref["__fileRef"])]
				}
			}
		}
		input = decoded.Body
		if values := fields["response_format"]; len(values) > 0 {
			input["response_format"] = values[0]
		}
	} else {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return nil, err
		}
		data, err := storage.Bytes()
		if err != nil {
			return nil, err
		}
		if err := common.Unmarshal(data, &input); err != nil {
			return nil, err
		}
	}
	data, err := common.Marshal(input)
	if err != nil {
		return nil, err
	}
	if len(info.ParamOverride) > 0 {
		data, err = relaycommon.ApplyParamOverrideWithRelayInfo(data, info)
		if err != nil {
			return nil, err
		}
		if err := common.Unmarshal(data, &input); err != nil {
			return nil, err
		}
	}
	c.Set(SeedreamResponseFormatKey, input["response_format"])
	decoded, err := seedream.Decode(c.Request.Context(), info.UpstreamModelName, operation, map[string]any{"kind": "json", "value": input})
	if err != nil {
		return nil, err
	}
	body, facts, err := decoded.Build(c.Request.Context(), info.UpstreamModelName)
	if err != nil {
		return nil, err
	}
	requestInput, err := ResolveIncomingBillingExprRequestInput(c, info)
	if err != nil {
		return nil, err
	}
	requestInput.Usage = facts
	info.BillingRequestInput = &requestInput
	c.Set(seedreamContext, decoded)
	return body, nil
}

func SeedreamRequest(c *gin.Context) *seedream.Request {
	value, _ := c.Get(seedreamContext)
	request, _ := value.(*seedream.Request)
	return request
}

func SeedreamReservationCount(info *relaycommon.RelayInfo) float64 {
	if info.BillingRequestInput == nil {
		return 1
	}
	usage := info.BillingRequestInput.Usage
	count := usage["images_up_to_1_5k"] + usage["images_above_1_5k"]
	if count < 1 {
		return 1
	}
	return count
}

func SetSeedreamActualUsage(info *relaycommon.RelayInfo, facts map[string]float64) {
	if info.BillingRequestInput == nil {
		info.BillingRequestInput = &billingexpr.RequestInput{}
	}
	if info.BillingRequestInput.Usage == nil {
		info.BillingRequestInput.Usage = map[string]float64{}
	}
	for key, value := range facts {
		info.BillingRequestInput.Usage[key] = value
	}
}
