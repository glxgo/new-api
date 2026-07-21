package controller

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
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
	result, err := perfmetrics.QueryGroupSummaryAll(hours, activeGroups)
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
	activeRatios := ratio_setting.GetGroupRatioCopy()
	hidden := hiddenStatusGroups()
	return lo.Filter(groups, func(g perfmetrics.GroupResult, _ int) bool {
		if hidden[g.Group] {
			return false
		}
		_, ok := activeRatios[g.Group]
		return ok || g.Group == "auto"
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

// visibleStatusGroups 返回模型状态页应展示的分组(ratio 分组 - 黑名单 + auto)。
func visibleStatusGroups() []string {
	hidden := hiddenStatusGroups()
	groupSet := map[string]struct{}{}
	for g := range ratio_setting.GetGroupRatioCopy() {
		if !hidden[g] {
			groupSet[g] = struct{}{}
		}
	}
	groupSet["auto"] = struct{}{}
	groups := make([]string, 0, len(groupSet))
	for group := range groupSet {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups
}
