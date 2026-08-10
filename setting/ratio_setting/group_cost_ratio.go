package ratio_setting

import (
	"github.com/QuantumNous/new-api/types"
)

// 分组成本倍率(渠道分组维度), 与售价倍率 GroupRatio 同构 map[group]float64。
// 业务(2026-06-22 重构, plan mellow-growing-waterfall.md):
//   平台成本 = 官方基础扣费 × GroupCostRatio[group]
// 未配置 GroupCostRatio[group] 时【继承】该分组的售价倍率 GroupRatio[group],
// 均未配置则默认 1。这样运营只需维护「一套官方售价 + 分组售价倍率 + 分组成本倍率」,
// 用于运营侧对照实际售价与平台成本，不参与充值分润。

var groupCostRatioMap = types.NewRWMap[string, float64]()

// 成本倍率来源(供后台预览展示「单独配置 / 继承售价倍率 / 默认」)。
const (
	CostRatioSourceExplicit  = "explicit"  // 显式配置了 GroupCostRatio[group]
	CostRatioSourceInherited = "inherited" // 未配, 继承 GroupRatio[group]
	CostRatioSourceDefault   = "default"   // 均未配, 默认 1
)

func GetGroupCostRatioCopy() map[string]float64 {
	return groupCostRatioMap.ReadAll()
}

func ContainsGroupCostRatio(name string) bool {
	_, ok := groupCostRatioMap.Get(name)
	return ok
}

func GroupCostRatio2JSONString() string {
	return groupCostRatioMap.MarshalJSONString()
}

func UpdateGroupCostRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupCostRatioMap, jsonStr)
}

// GetGroupCostRatioWithSource 返回分组的成本倍率及其来源。
//   - 显式配置了 GroupCostRatio[group] → (值, explicit)
//   - 否则继承 GroupRatio[group]          → (值, inherited)
//   - 均未配置                            → (1, default)
func GetGroupCostRatioWithSource(group string) (ratio float64, source string) {
	if r, ok := groupCostRatioMap.Get(group); ok {
		return r, CostRatioSourceExplicit
	}
	if r, ok := groupRatioMap.Get(group); ok {
		return r, CostRatioSourceInherited
	}
	return 1, CostRatioSourceDefault
}

// GetGroupCostRatio 返回分组的成本倍率(未配置则继承售价倍率, 再未配置则 1)。
func GetGroupCostRatio(group string) float64 {
	ratio, _ := GetGroupCostRatioWithSource(group)
	return ratio
}

// CheckGroupCostRatio 校验成本倍率 JSON: 与 GroupRatio 同构(map[string]float64, 值 ≥ 0)。
func CheckGroupCostRatio(jsonStr string) error {
	return CheckGroupRatio(jsonStr)
}
