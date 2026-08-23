package controller

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type usageStatisticsPeriod struct {
	duration      time.Duration
	bucketSeconds int64
}

var usageStatisticsPeriods = map[string]usageStatisticsPeriod{
	"24h": {duration: 24 * time.Hour, bucketSeconds: int64(time.Hour / time.Second)},
	"7d":  {duration: 7 * 24 * time.Hour, bucketSeconds: int64(6 * time.Hour / time.Second)},
	"30d": {duration: 30 * 24 * time.Hour, bucketSeconds: int64(24 * time.Hour / time.Second)},
}

func GetUsageStatisticsSelf(c *gin.Context) {
	getUsageStatistics(c, c.GetInt("id"))
}

// GetUsageStatisticsAdmin lets the root administrator inspect one user's
// bounded usage report without changing the user-facing /self contract.
func GetUsageStatisticsAdmin(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("user"))
	if keyword == "" {
		keyword = strings.TrimSpace(c.Query("user_id"))
	}
	if keyword == "" {
		common.ApiErrorMsg(c, "请输入用户名或用户ID")
		return
	}

	var user model.User
	query := model.DB.Omit("password")
	if userID, err := strconv.Atoi(keyword); err == nil && userID > 0 {
		// Prefer an exact numeric ID, but fall back to an exact username so
		// installations that allow all-numeric usernames remain searchable.
		if err := query.Where("id = ?", userID).First(&user).Error; err != nil {
			if err := model.DB.Omit("password").Where("username = ?", keyword).First(&user).Error; err != nil {
				common.ApiErrorMsg(c, "未找到该用户")
				return
			}
		}
	} else if err := query.Where("username = ?", keyword).First(&user).Error; err != nil {
		common.ApiErrorMsg(c, "未找到该用户")
		return
	}
	getUsageStatistics(c, user.Id)
}

func getUsageStatistics(c *gin.Context, userID int) {
	periodName := c.DefaultQuery("range", "7d")
	period, ok := usageStatisticsPeriods[periodName]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "range must be one of 24h, 7d, or 30d",
		})
		return
	}

	endTime := common.GetTimestamp()
	startTime := endTime - int64(period.duration/time.Second)
	stats, err := model.GetUserUsageStatistics(userID, startTime, endTime, period.bucketSeconds)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	response := gin.H{
		"range":           periodName,
		"start_timestamp": startTime,
		"end_timestamp":   endTime,
		"bucket_seconds":  period.bucketSeconds,
		"summary":         stats.Summary,
		"series":          stats.Series,
		"models":          stats.Models,
		"subscriptions":   stats.Subscriptions,
	}
	if userID != c.GetInt("id") {
		var user struct {
			Id          int    `json:"id"`
			Username    string `json:"username"`
			DisplayName string `json:"display_name"`
			Email       string `json:"email"`
		}
		if err := model.DB.Model(&model.User{}).Select("id, username, display_name, email").Where("id = ?", userID).First(&user).Error; err == nil {
			response["user"] = user
		}
	}
	common.ApiSuccess(c, response)
}
