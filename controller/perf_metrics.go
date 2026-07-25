package controller

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

func GetPerfMetricsSummary(c *gin.Context) {
	hours := 24
	if rawHours := c.Query("hours"); rawHours != "" {
		if parsed, err := strconv.Atoi(rawHours); err == nil {
			hours = parsed
		}
	}

	activeGroups := visibleStatusGroups()
	result, err := perfmetrics.QuerySummaryAll(hours, activeGroups)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

func GetPerfMetricsGroupSummary(c *gin.Context) {
	hours := 24
	if rawHours := c.Query("hours"); rawHours != "" {
		if parsed, err := strconv.Atoi(rawHours); err == nil {
			hours = parsed
		}
	}

	activeGroups := visibleStatusGroups()
	scopes, err := loadPerfMetricChannelScopes(activeGroups)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	result, err := perfmetrics.QueryGroupSummaryByChannels(hours, scopes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	probeSummaries, err := buildGroupProbeSummaries(hours, activeGroups)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	groupIndexes := make(map[string]int, len(result.Groups))
	for index := range result.Groups {
		groupIndexes[result.Groups[index].Group] = index
	}
	for group, probe := range probeSummaries {
		if index, ok := groupIndexes[group]; ok {
			result.Groups[index].Probe = probe
			continue
		}
		result.Groups = append(result.Groups, perfmetrics.GroupCacheSummary{
			Group: group,
			Probe: probe,
		})
	}
	sort.Slice(result.Groups, func(i, j int) bool { return result.Groups[i].Group < result.Groups[j].Group })

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

func loadPerfMetricChannelScopes(activeGroups []string) ([]perfmetrics.GroupChannelScope, error) {
	channels, err := model.GetAllChannelsWithoutKey()
	if err != nil {
		return nil, err
	}
	abilities, err := model.GetAllEnableAbilitiesWithError()
	if err != nil {
		return nil, err
	}
	return buildPerfMetricChannelScopes(
		activeGroups,
		channels,
		abilities,
		operation_setting.GetMonitorSetting(),
	), nil
}

func buildPerfMetricChannelScopes(
	activeGroups []string,
	channels []*model.Channel,
	abilities []model.Ability,
	settings *operation_setting.MonitorSetting,
) []perfmetrics.GroupChannelScope {
	activeGroupSet := make(map[string]struct{}, len(activeGroups))
	channelSets := make(map[string]map[string]map[int]struct{}, len(activeGroups))
	for _, group := range activeGroups {
		if group == "" {
			continue
		}
		activeGroupSet[group] = struct{}{}
		channelSets[group] = make(map[string]map[int]struct{})
	}

	selectedIds := []int(nil)
	if settings != nil {
		selectedIds = settings.ChannelCanaryChannelIds
	}
	eligibleChannels := make(map[int]struct{})
	for _, channel := range channels {
		if channel == nil || channel.Status != common.ChannelStatusEnabled {
			continue
		}
		if len(selectedIds) > 0 && !lo.Contains(selectedIds, channel.Id) {
			continue
		}
		eligibleChannels[channel.Id] = struct{}{}
	}

	add := func(group string, ability model.Ability) {
		if _, ok := channelSets[group][ability.Model]; !ok {
			channelSets[group][ability.Model] = make(map[int]struct{})
		}
		channelSets[group][ability.Model][ability.ChannelId] = struct{}{}
	}
	for _, ability := range abilities {
		if !ability.Enabled || ability.Model == "" {
			continue
		}
		if _, ok := eligibleChannels[ability.ChannelId]; !ok {
			continue
		}
		if _, ok := activeGroupSet[ability.Group]; !ok {
			continue
		}
		add(ability.Group, ability)
	}

	scopes := make([]perfmetrics.GroupChannelScope, 0, len(channelSets))
	for group, models := range channelSets {
		modelChannels := make(map[string][]int, len(models))
		for modelName, channelSet := range models {
			channelIds := make([]int, 0, len(channelSet))
			for channelId := range channelSet {
				channelIds = append(channelIds, channelId)
			}
			sort.Ints(channelIds)
			modelChannels[modelName] = channelIds
		}
		scopes = append(scopes, perfmetrics.GroupChannelScope{
			Group:         group,
			ModelChannels: modelChannels,
		})
	}
	sort.Slice(scopes, func(i, j int) bool { return scopes[i].Group < scopes[j].Group })
	return scopes
}

func GetPerfMetrics(c *gin.Context) {
	modelName := c.Query("model")
	if modelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "model is required",
		})
		return
	}

	hours := 24
	if rawHours := c.Query("hours"); rawHours != "" {
		if parsed, err := strconv.Atoi(rawHours); err == nil {
			hours = parsed
		}
	}

	result, err := perfmetrics.Query(perfmetrics.QueryParams{
		Model: modelName,
		Group: c.Query("group"),
		Hours: hours,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	result.Groups = filterActiveGroups(result.Groups)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

func filterActiveGroups(groups []perfmetrics.GroupResult) []perfmetrics.GroupResult {
	visible := lo.SliceToMap(visibleStatusGroups(), func(group string) (string, struct{}) {
		return group, struct{}{}
	})
	return lo.Filter(groups, func(g perfmetrics.GroupResult, _ int) bool {
		_, ok := visible[g.Group]
		return ok
	})
}

// hiddenStatusGroups 读取 option 'HiddenStatusGroups'(逗号分隔), 返回模型状态页不展示的分组集合。
// 默认黑名单: "gpt 企业专属,自用分组"(内部分组, 用户选不到)。后台可增删。
func hiddenStatusGroups() map[string]bool {
	hidden := map[string]bool{}
	var value string
	if err := model.DB.Model(&model.Option{}).Where("`key` = ?", "HiddenStatusGroups").
		Limit(1).Pluck("value", &value).Error; err != nil || strings.TrimSpace(value) == "" {
		value = "gpt 企业专属,自用分组"
	}
	for _, g := range strings.Split(value, ",") {
		if g = strings.TrimSpace(g); g != "" {
			hidden[g] = true
		}
	}
	return hidden
}

// visibleStatusGroups 返回模型状态页应展示的分组（ratio 分组 - 黑名单）。
// auto 仅用于运行时自动路由，不作为独立、可观测的服务分组展示。
func visibleStatusGroups() []string {
	hidden := hiddenStatusGroups()
	groupSet := map[string]struct{}{}
	for g := range ratio_setting.GetGroupRatioCopy() {
		if !hidden[g] {
			groupSet[g] = struct{}{}
		}
	}
	groups := make([]string, 0, len(groupSet))
	for group := range groupSet {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	channels, err := model.GetAllChannelsWithoutKey()
	if err != nil {
		return groups
	}
	return filterStatusGroupsByCanarySelection(groups, channels, operation_setting.GetMonitorSetting())
}
