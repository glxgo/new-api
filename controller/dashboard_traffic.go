package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const maxDashboardTrafficRange = 31 * 24 * time.Hour

type dashboardTrafficQuery struct {
	startTime int64
	endTime   int64
	location  *time.Location
}

func parseDashboardTrafficQuery(c *gin.Context) (dashboardTrafficQuery, bool) {
	now := time.Now().Unix()
	endTime := now
	if raw := c.Query("end_timestamp"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "end_timestamp is invalid"})
			return dashboardTrafficQuery{}, false
		}
		endTime = parsed
	}
	startTime := endTime - int64(7*24*time.Hour/time.Second)
	if raw := c.Query("start_timestamp"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "start_timestamp is invalid"})
			return dashboardTrafficQuery{}, false
		}
		startTime = parsed
	}
	if startTime >= endTime {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "time range is invalid"})
		return dashboardTrafficQuery{}, false
	}
	if time.Duration(endTime-startTime)*time.Second > maxDashboardTrafficRange {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "time range cannot exceed 31 days"})
		return dashboardTrafficQuery{}, false
	}

	timezoneOffset := 0
	if raw := c.Query("timezone_offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < -840 || parsed > 840 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "timezone_offset is invalid"})
			return dashboardTrafficQuery{}, false
		}
		timezoneOffset = parsed
	}
	// JavaScript getTimezoneOffset is UTC - local; FixedZone expects local - UTC.
	location := time.FixedZone("dashboard", -timezoneOffset*60)
	return dashboardTrafficQuery{startTime: startTime, endTime: endTime, location: location}, true
}

func getDashboardTraffic(c *gin.Context, userId int, includeChannels bool) {
	query, ok := parseDashboardTrafficQuery(c)
	if !ok {
		return
	}
	records, err := model.GetDashboardTrafficRecords(userId, query.startTime, query.endTime)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	channelNames := map[int]string{}
	if includeChannels {
		channelSet := make(map[int]struct{})
		for _, record := range records {
			if record.ChannelId > 0 {
				channelSet[record.ChannelId] = struct{}{}
			}
		}
		channelIds := make([]int, 0, len(channelSet))
		for channelId := range channelSet {
			channelIds = append(channelIds, channelId)
		}
		channelNames, err = model.GetDashboardChannelNames(channelIds)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}

	result := service.BuildDashboardTraffic(
		records,
		channelNames,
		query.startTime,
		query.endTime,
		query.location,
		includeChannels,
	)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func GetDashboardTrafficSelf(c *gin.Context) {
	getDashboardTraffic(c, common.GetContextKeyInt(c, constant.ContextKeyUserId), false)
}

func GetDashboardTraffic(c *gin.Context) {
	getDashboardTraffic(c, 0, true)
}
