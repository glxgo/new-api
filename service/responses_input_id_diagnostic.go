package service

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const ginKeyResponsesInputIDDiagnostic = "responses_input_id_diagnostic"

var responsesInputIDParamPattern = regexp.MustCompile(`^input\[(\d+)\]\.id$`)

// ResponsesInputIDDiagnostic contains only bounded structural metadata. It is
// safe to attach to administrator logs because it never stores request content
// or complete item/session identifiers.
type ResponsesInputIDDiagnostic struct {
	Param                    string `json:"param"`
	InputIndex               int    `json:"input_index"`
	InputCount               int    `json:"input_count"`
	ItemType                 string `json:"item_type,omitempty"`
	ItemRole                 string `json:"item_role,omitempty"`
	IDPrefix                 string `json:"id_prefix,omitempty"`
	IDLength                 int    `json:"id_length"`
	PreviousResponseIDPrefix string `json:"previous_response_id_prefix,omitempty"`
	PreviousResponseIDLength int    `json:"previous_response_id_length,omitempty"`
	PromptCacheKeyHash       string `json:"prompt_cache_key_hash,omitempty"`
}

// CaptureResponsesInputIDDiagnostic inspects only the single input item named
// by a structured upstream 400 parameter such as input[264].id. Non-matching
// errors clear any prior per-attempt diagnostic from the request context.
func CaptureResponsesInputIDDiagnostic(c *gin.Context, request dto.Request, apiErr *types.NewAPIError) {
	if c == nil {
		return
	}
	c.Set(ginKeyResponsesInputIDDiagnostic, nil)
	if apiErr == nil || apiErr.StatusCode != 400 {
		return
	}

	param := responsesInputIDErrorParam(apiErr)
	matches := responsesInputIDParamPattern.FindStringSubmatch(param)
	if len(matches) != 2 {
		return
	}
	inputIndex, err := strconv.Atoi(matches[1])
	if err != nil || inputIndex < 0 {
		return
	}

	input, previousResponseID, promptCacheKey, ok := responsesRequestDiagnosticFields(request)
	if !ok || common.GetJsonType(input) != "array" {
		return
	}
	var items []json.RawMessage
	if err := common.Unmarshal(input, &items); err != nil || inputIndex >= len(items) {
		return
	}
	var item struct {
		Type string `json:"type"`
		Role string `json:"role"`
		ID   string `json:"id"`
	}
	if err := common.Unmarshal(items[inputIndex], &item); err != nil {
		return
	}
	itemType := strings.TrimSpace(item.Type)
	if itemType == "" && strings.TrimSpace(item.Role) != "" {
		itemType = "message"
	}
	diagnostic := ResponsesInputIDDiagnostic{
		Param:                    param,
		InputIndex:               inputIndex,
		InputCount:               len(items),
		ItemType:                 itemType,
		ItemRole:                 strings.TrimSpace(item.Role),
		IDPrefix:                 safeResponsesIDPrefix(item.ID),
		IDLength:                 len(item.ID),
		PreviousResponseIDPrefix: safeResponsesIDPrefix(previousResponseID),
		PreviousResponseIDLength: len(previousResponseID),
		PromptCacheKeyHash:       hashPromptCacheKey(promptCacheKey),
	}
	c.Set(ginKeyResponsesInputIDDiagnostic, diagnostic)
}

func responsesInputIDErrorParam(apiErr *types.NewAPIError) string {
	if apiErr == nil {
		return ""
	}
	switch relayErr := apiErr.RelayError.(type) {
	case types.OpenAIError:
		return strings.TrimSpace(relayErr.Param)
	case *types.OpenAIError:
		if relayErr != nil {
			return strings.TrimSpace(relayErr.Param)
		}
	}
	return ""
}

func responsesRequestDiagnosticFields(request dto.Request) (json.RawMessage, string, json.RawMessage, bool) {
	switch req := request.(type) {
	case *dto.OpenAIResponsesRequest:
		if req == nil {
			return nil, "", nil, false
		}
		return req.Input, req.PreviousResponseID, req.PromptCacheKey, true
	case *dto.OpenAIResponsesCompactionRequest:
		if req == nil {
			return nil, "", nil, false
		}
		return req.Input, req.PreviousResponseID, nil, true
	default:
		return nil, "", nil, false
	}
}

func safeResponsesIDPrefix(id string) string {
	id = strings.TrimSpace(id)
	underscore := strings.IndexByte(id, '_')
	if underscore <= 0 || underscore > 16 {
		if id == "" {
			return ""
		}
		return "unrecognized"
	}
	for _, char := range id[:underscore] {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' {
			return "unrecognized"
		}
	}
	return id[:underscore+1]
}

func hashPromptCacheKey(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var key string
	if err := common.Unmarshal(raw, &key); err != nil || key == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", hash[:6])
}

func GetResponsesInputIDDiagnostic(c *gin.Context) (ResponsesInputIDDiagnostic, bool) {
	if c == nil {
		return ResponsesInputIDDiagnostic{}, false
	}
	value, exists := c.Get(ginKeyResponsesInputIDDiagnostic)
	if !exists {
		return ResponsesInputIDDiagnostic{}, false
	}
	diagnostic, ok := value.(ResponsesInputIDDiagnostic)
	return diagnostic, ok
}

func AppendResponsesInputIDDiagnosticAdminInfo(c *gin.Context, adminInfo map[string]interface{}) {
	if adminInfo == nil {
		return
	}
	if diagnostic, ok := GetResponsesInputIDDiagnostic(c); ok {
		adminInfo["responses_input_id_diagnostic"] = diagnostic
	}
}

func FormatResponsesInputIDDiagnostic(c *gin.Context) string {
	diagnostic, ok := GetResponsesInputIDDiagnostic(c)
	if !ok {
		return ""
	}
	return fmt.Sprintf(
		"param=%s input_index=%d input_count=%d item_type=%s item_role=%s id_prefix=%s id_length=%d previous_response_id_prefix=%s previous_response_id_length=%d prompt_cache_key_hash=%s",
		diagnostic.Param,
		diagnostic.InputIndex,
		diagnostic.InputCount,
		diagnostic.ItemType,
		diagnostic.ItemRole,
		diagnostic.IDPrefix,
		diagnostic.IDLength,
		diagnostic.PreviousResponseIDPrefix,
		diagnostic.PreviousResponseIDLength,
		diagnostic.PromptCacheKeyHash,
	)
}
