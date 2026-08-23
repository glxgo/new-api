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

// Some Responses-compatible providers build an assistant message ID from the
// response ID (resp_<opaque>_msg) instead of using the required msg_ prefix.
// Keep this deliberately narrow: only the observed alphanumeric form is
// eligible, and only when the input item is known to be a message.
func isSyntheticResponsesMessageID(id string) bool {
	const responsePrefix = "resp_"
	const messageSuffix = "_msg"
	if !strings.HasPrefix(id, responsePrefix) || !strings.HasSuffix(id, messageSuffix) {
		return false
	}
	opaque := id[len(responsePrefix) : len(id)-len(messageSuffix)]
	if opaque == "" {
		return false
	}
	for _, char := range opaque {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') {
			return false
		}
	}
	return true
}

// ResponsesInputItemIDNormalizationReport contains only aggregate metadata;
// it never exposes complete response item identifiers or request content.
type ResponsesInputItemIDNormalizationReport struct {
	Reasoning          int `json:"reasoning"`
	Message            int `json:"message"`
	FunctionCall       int `json:"function_call"`
	CustomToolCall     int `json:"custom_tool_call"`
	FunctionCallOutput int `json:"function_call_output"`
	CustomToolOutput   int `json:"custom_tool_call_output"`
}

func (r ResponsesInputItemIDNormalizationReport) Count() int {
	return r.Reasoning + r.Message + r.FunctionCall + r.CustomToolCall + r.FunctionCallOutput + r.CustomToolOutput
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

// NormalizeResponsesInputItemIDs repairs narrowly identified compatibility
// defects emitted by some Responses-compatible providers: output items are
// assigned a generic item_ ID, a response-scoped resp_*_msg ID, or a function
// call ID is reused for a custom tool call. Keep the opaque suffix and every
// other byte of the item; only known type/prefix mismatches are normalized.
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
		itemType := strings.TrimSpace(item.Get("type").String())
		if itemType == "" && strings.TrimSpace(item.Get("role").String()) != "" {
			itemType = "message"
		}

		var normalizedID string
		switch {
		case itemType == "message" && isSyntheticResponsesMessageID(id):
			// Replace only the invalid leading response prefix. Retaining the
			// opaque suffix avoids collapsing two provider-generated IDs.
			normalizedID = "msg_" + strings.TrimPrefix(id, "resp_")
		case strings.HasPrefix(id, responsesGenericItemIDPrefix) && len(id) > len(responsesGenericItemIDPrefix) && itemType == "reasoning":
			normalizedID = "rs_" + strings.TrimPrefix(id, responsesGenericItemIDPrefix)
		case strings.HasPrefix(id, responsesGenericItemIDPrefix) && len(id) > len(responsesGenericItemIDPrefix) && itemType == "message":
			normalizedID = "msg_" + strings.TrimPrefix(id, responsesGenericItemIDPrefix)
		case itemType == "custom_tool_call" && strings.HasPrefix(id, "fc_"):
			// OpenAI requires custom_tool_call IDs to begin with ctc_. Some
			// providers incorrectly reuse the fc_ prefix used by function_call.
			// Prefix the complete opaque ID instead of replacing its bytes, which
			// keeps this repair collision-resistant and makes the source defect
			// observable in the diagnostic report without leaking the ID.
			normalizedID = "ctc_" + id
		case itemType == "custom_tool_call_output" && strings.HasPrefix(id, "fco_"):
			normalizedID = "ctco_" + id
		case itemType == "function_call" && strings.HasPrefix(id, "ctc_"):
			normalizedID = "fc_" + id
		case itemType == "function_call_output" && strings.HasPrefix(id, "ctco_"):
			normalizedID = "fco_" + id
		default:
			continue
		}
		replacements = append(replacements, replacement{
			path: fmt.Sprintf("input.%d.id", index),
			id:   normalizedID,
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
		case "function_call":
			report.FunctionCall++
		case "custom_tool_call":
			report.CustomToolCall++
		case "function_call_output":
			report.FunctionCallOutput++
		case "custom_tool_call_output":
			report.CustomToolOutput++
		}
	}
	return result, report, nil
}
