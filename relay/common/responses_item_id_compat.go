package common

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const responsesGenericItemIDPrefix = "item_"
const ginKeyResponsesInputItemIDNormalization = "responses_input_item_id_normalization"

// ResponsesInputItemIDNormalizationReport contains only aggregate metadata;
// it never exposes complete response item identifiers or request content.
type ResponsesInputItemIDNormalizationReport struct {
	Reasoning int `json:"reasoning"`
	Message   int `json:"message"`
}

func (r ResponsesInputItemIDNormalizationReport) Count() int {
	return r.Reasoning + r.Message
}

func RecordResponsesInputItemIDNormalization(c *gin.Context, report ResponsesInputItemIDNormalizationReport) {
	if c == nil || report.Count() == 0 {
		return
	}
	c.Set(ginKeyResponsesInputItemIDNormalization, report)
}

func AppendResponsesInputItemIDNormalizationAdminInfo(c *gin.Context, adminInfo map[string]interface{}) {
	if c == nil || adminInfo == nil {
		return
	}
	value, ok := c.Get(ginKeyResponsesInputItemIDNormalization)
	if !ok {
		return
	}
	report, ok := value.(ResponsesInputItemIDNormalizationReport)
	if !ok || report.Count() == 0 {
		return
	}
	adminInfo["responses_input_item_id_normalization"] = report
}

// NormalizeResponsesInputItemIDs repairs a compatibility defect emitted by
// some Responses-compatible providers: output items are assigned a generic
// item_ ID even though the item type requires a typed prefix when the item is
// replayed as input. Keep the opaque suffix and every other byte of the item;
// only the proven reasoning and message cases are normalized.
func NormalizeResponsesInputItemIDs(body []byte) ([]byte, ResponsesInputItemIDNormalizationReport, error) {
	var report ResponsesInputItemIDNormalizationReport
	input := gjson.GetBytes(body, "input")
	if !input.Exists() || !input.IsArray() {
		return body, report, nil
	}

	type replacement struct {
		path string
		id   string
		kind string
	}
	replacements := make([]replacement, 0)
	for index, item := range input.Array() {
		id := strings.TrimSpace(item.Get("id").String())
		if !strings.HasPrefix(id, responsesGenericItemIDPrefix) || len(id) == len(responsesGenericItemIDPrefix) {
			continue
		}

		itemType := strings.TrimSpace(item.Get("type").String())
		if itemType == "" && strings.TrimSpace(item.Get("role").String()) != "" {
			itemType = "message"
		}

		var typedPrefix string
		switch itemType {
		case "reasoning":
			typedPrefix = "rs_"
		case "message":
			typedPrefix = "msg_"
		default:
			continue
		}
		replacements = append(replacements, replacement{
			path: fmt.Sprintf("input.%d.id", index),
			id:   typedPrefix + strings.TrimPrefix(id, responsesGenericItemIDPrefix),
			kind: itemType,
		})
	}

	if len(replacements) == 0 {
		return body, report, nil
	}

	result := body
	for _, patch := range replacements {
		next, err := sjson.SetBytes(result, patch.path, patch.id)
		if err != nil {
			return body, ResponsesInputItemIDNormalizationReport{}, err
		}
		result = next
		switch patch.kind {
		case "reasoning":
			report.Reasoning++
		case "message":
			report.Message++
		}
	}
	return result, report, nil
}
