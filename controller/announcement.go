package controller

import (
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/gin-gonic/gin"
)

func normalizeAnnouncementId(item map[string]interface{}, index int) int64 {
	if item != nil {
		switch value := item["id"].(type) {
		case float64:
			if value > 0 {
				return int64(value)
			}
		case int64:
			if value > 0 {
				return value
			}
		case string:
			if id, err := strconv.ParseInt(value, 10, 64); err == nil && id > 0 {
				return id
			}
		}
		if raw, ok := item["publishDate"].(string); ok {
			if publishedAt, err := time.Parse(time.RFC3339, raw); err == nil {
				return publishedAt.UnixMilli() + int64(index)
			}
		}
	}
	return int64(index + 1)
}

func GetUserAnnouncements(c *gin.Context) {
	items := console_setting.GetAnnouncements()
	readIds, err := model.GetReadAnnouncementIds(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	all := make([]map[string]interface{}, 0, len(items))
	unread := make([]map[string]interface{}, 0, len(items))
	for index, source := range items {
		item := make(map[string]interface{}, len(source)+1)
		for key, value := range source {
			item[key] = value
		}
		id := normalizeAnnouncementId(item, index)
		item["id"] = id
		_, isRead := readIds[id]
		item["unread"] = !isRead
		all = append(all, item)
		if !isRead {
			unread = append(unread, item)
		}
	}
	common.ApiSuccess(c, gin.H{
		"enabled":       console_setting.GetConsoleSetting().AnnouncementsEnabled,
		"announcements": all,
		"unread":        unread,
	})
}

func MarkUserAnnouncementsRead(c *gin.Context) {
	var req struct {
		Ids []int64 `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	valid := map[int64]struct{}{}
	for index, item := range console_setting.GetAnnouncements() {
		valid[normalizeAnnouncementId(item, index)] = struct{}{}
	}
	ids := make([]int64, 0, len(req.Ids))
	for _, id := range req.Ids {
		if _, exists := valid[id]; exists {
			ids = append(ids, id)
		}
	}
	if err := model.MarkAnnouncementsRead(c.GetInt("id"), ids); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"read": len(ids)})
}
