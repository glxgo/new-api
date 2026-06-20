package ratio_setting

import (
	"encoding/json"
	"sync/atomic"
)

// 分组级模型定价(渠道分组维度), 覆盖全局 ModelRatio/ModelPrice/ModelCost。
// 业务: 同模型在不同渠道分组走不同上游 → 成本/售价不同; miss 回退全局。
// 数据: 嵌套 map[group]map[model]value, 用 atomic.Value 存不可变快照(读无锁, Update 整体替换)。
// 详见 plan mellow-growing-waterfall.md 需求2。

var (
	groupModelRatioVal atomic.Value // map[string]map[string]float64 (group→model→售价ratio)
	groupModelPriceVal atomic.Value // map[string]map[string]float64 (group→model→$/request)
	groupModelCostVal  atomic.Value // map[string]map[string]ModelCostInfo
)

func loadGroupRatioMap() map[string]map[string]float64 {
	if v := groupModelRatioVal.Load(); v != nil {
		if m, ok := v.(map[string]map[string]float64); ok {
			return m
		}
	}
	return nil
}
func loadGroupPriceMap() map[string]map[string]float64 {
	if v := groupModelPriceVal.Load(); v != nil {
		if m, ok := v.(map[string]map[string]float64); ok {
			return m
		}
	}
	return nil
}
func loadGroupCostMap() map[string]map[string]ModelCostInfo {
	if v := groupModelCostVal.Load(); v != nil {
		if m, ok := v.(map[string]map[string]ModelCostInfo); ok {
			return m
		}
	}
	return nil
}

// GetModelRatioForGroup 分组级售价 ratio, miss 回退全局 GetModelRatio。
func GetModelRatioForGroup(name, group string) (float64, bool, string) {
	name = FormatMatchingModelName(name)
	if group != "" {
		if m := loadGroupRatioMap(); m != nil {
			if gm, ok := m[group]; ok {
				if r, ok2 := gm[name]; ok2 {
					return r, true, name
				}
			}
		}
	}
	return GetModelRatio(name)
}

// GetModelPriceForGroup 分组级按次售价 $/request, miss 回退全局 GetModelPrice。
func GetModelPriceForGroup(name, group string, printErr bool) (float64, bool) {
	name = FormatMatchingModelName(name)
	if group != "" {
		if m := loadGroupPriceMap(); m != nil {
			if gm, ok := m[group]; ok {
				if p, ok2 := gm[name]; ok2 {
					return p, true
				}
			}
		}
	}
	return GetModelPrice(name, printErr)
}

// GetModelCostForGroup 分组级成本, miss 回退全局 GetModelCost。
func GetModelCostForGroup(name, group string) (ModelCostInfo, bool) {
	name = FormatMatchingModelName(name)
	if group != "" {
		if m := loadGroupCostMap(); m != nil {
			if gm, ok := m[group]; ok {
				if c, ok2 := gm[name]; ok2 {
					return c, true
				}
			}
		}
	}
	return GetModelCost(name)
}

// --- Update(嵌套 JSON) / 2JSON ---

func UpdateGroupModelRatioByJSONString(jsonStr string) error {
	var m map[string]map[string]float64
	if jsonStr == "" {
		groupModelRatioVal.Store(map[string]map[string]float64{})
		InvalidateExposedDataCache()
		return nil
	}
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		return err
	}
	groupModelRatioVal.Store(m)
	InvalidateExposedDataCache()
	return nil
}

func UpdateGroupModelPriceByJSONString(jsonStr string) error {
	var m map[string]map[string]float64
	if jsonStr == "" {
		groupModelPriceVal.Store(map[string]map[string]float64{})
		InvalidateExposedDataCache()
		return nil
	}
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		return err
	}
	groupModelPriceVal.Store(m)
	InvalidateExposedDataCache()
	return nil
}

func UpdateGroupModelCostByJSONString(jsonStr string) error {
	var m map[string]map[string]ModelCostInfo
	if jsonStr == "" {
		groupModelCostVal.Store(map[string]map[string]ModelCostInfo{})
		InvalidateExposedDataCache()
		return nil
	}
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		return err
	}
	groupModelCostVal.Store(m)
	InvalidateExposedDataCache()
	return nil
}

func GroupModelRatio2JSONString() string {
	if m := loadGroupRatioMap(); m != nil {
		b, _ := json.Marshal(m)
		return string(b)
	}
	return "{}"
}

func GroupModelPrice2JSONString() string {
	if m := loadGroupPriceMap(); m != nil {
		b, _ := json.Marshal(m)
		return string(b)
	}
	return "{}"
}

func GroupModelCost2JSONString() string {
	if m := loadGroupCostMap(); m != nil {
		b, _ := json.Marshal(m)
		return string(b)
	}
	return "{}"
}

// 只读快照(管理/审计用)。
func GetGroupModelRatioMap() map[string]map[string]float64         { return loadGroupRatioMap() }
func GetGroupModelPriceMap() map[string]map[string]float64         { return loadGroupPriceMap() }
func GetGroupModelCostMap() map[string]map[string]ModelCostInfo    { return loadGroupCostMap() }
