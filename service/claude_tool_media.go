package service

import (
	"errors"
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"strings"
)

// Keep parallel tool results contiguous; media belongs to the following user
// message because Chat tool messages cannot contain image/file blocks.
func claudeToolResultToChat(c *gin.Context, blocks []dto.ClaudeMediaMessage) (string, []dto.MediaContent, error) {
	var texts []string
	var media []dto.MediaContent
	for _, block := range blocks {
		switch {
		case block.Type == "text" || block.Type == "input_text":
			if text := block.GetText(); text != "" {
				texts = append(texts, text)
			}
		case block.Type == "image" && block.Source != nil:
			url := block.Source.Url
			if url == "" {
				url = fmt.Sprintf("data:%s;base64,%s", block.Source.MediaType, common.Interface2String(block.Source.Data))
			}
			media = append(media, dto.MediaContent{Type: "image_url", ImageUrl: &dto.MessageImageUrl{Url: url}})
		case block.Type == "document" && block.Source != nil && block.Source.Type == "base64":
			data := fmt.Sprintf("data:%s;base64,%s", block.Source.MediaType, common.Interface2String(block.Source.Data))
			media = append(media, dto.MediaContent{Type: "file", File: &dto.MessageFile{FileName: "document", FileData: data}})
		case block.Type == "document" && block.Source != nil && block.Source.Type == "url":
			if c == nil {
				return "", nil, errors.New("document URL requires request context")
			}
			data, mime, err := GetBase64Data(c, types.NewFileSourceFromData(block.Source.Url, ""), "tool_output")
			if err != nil {
				return "", nil, err
			}
			media = append(media, dto.MediaContent{Type: "file", File: &dto.MessageFile{FileName: "document", FileData: "data:" + mime + ";base64," + data}})
		case block.Type == "document" && block.Source != nil && block.Source.Type == "text":
			texts = append(texts, common.Interface2String(block.Source.Data))
		default:
			texts = append(texts, toJSONString(block))
		}
	}
	if len(texts) == 0 && len(media) > 0 {
		return "[tool media attached]", media, nil
	}
	return strings.Join(texts, "\n"), media, nil
}
