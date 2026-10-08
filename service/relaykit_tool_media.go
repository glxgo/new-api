package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// RelayKit preserves Responses tool-output blocks. Normalize those blocks to
// Claude's native schema, resolving remote media through the host's SSRF guard.
func normalizeClaudeToolMedia(c *gin.Context, request *kitdto.ClaudeRequest) error {
	for i := range request.Messages {
		message := &request.Messages[i]
		if message.IsStringContent() {
			continue
		}
		data, err := common.Marshal(message.Content)
		if err != nil {
			return err
		}
		var blocks []map[string]any
		if err := common.Unmarshal(data, &blocks); err != nil {
			return err
		}
		for _, block := range blocks {
			if block["type"] != "tool_result" {
				continue
			}
			items, ok := block["content"].([]any)
			if !ok {
				continue
			}
			for j, item := range items {
				media, ok := item.(map[string]any)
				if !ok {
					return errors.New("invalid tool media block")
				}
				switch media["type"] {
				case "input_text":
					media["type"] = "text"
				case "input_image", "input_file":
					kind, source := "image", ""
					if media["type"] == "input_image" {
						source, _ = media["image_url"].(string)
					} else {
						kind = "document"
						source, _ = media["file_data"].(string)
						if source == "" {
							source, _ = media["file_url"].(string)
						}
					}
					if source == "" {
						return errors.New("tool media requires inline data or a URL")
					}
					data, mime, err := GetBase64Data(c, types.NewFileSourceFromData(source, ""), "tool_output")
					if err != nil {
						return err
					}
					if (kind == "image" && !strings.HasPrefix(mime, "image/")) || (kind == "document" && mime != "application/pdf") {
						return errors.New("unsupported Claude tool media type")
					}
					items[j] = map[string]any{"type": kind, "source": map[string]any{"type": "base64", "media_type": mime, "data": data}}
				}
			}
		}
		message.Content = blocks
	}
	return nil
}
