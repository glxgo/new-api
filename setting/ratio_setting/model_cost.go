package ratio_setting

import "github.com/QuantumNous/new-api/types"

// ModelCostInfo 单个模型的成本价($/1M tokens), 与售价(ModelRatio/ModelPrice/billing_expr)完全分离。
// 成本 = 平台付给上游(如 krill)的买入价, 仅超管可写、管理员只读。用于利润/分红/返利计算。
type ModelCostInfo struct {
	InputCostPerM  float64 `json:"input_cost_per_m"`  // 成本输入 $/1M tokens
	OutputCostPerM float64 `json:"output_cost_per_m"` // 成本输出 $/1M tokens
	CacheCostPerM  float64 `json:"cache_cost_per_m"`  // 可选缓存成本 $/1M tokens
}

// modelCostMap 成本价表, 启动时空(无默认值, 全部由超管后台填写), 由 options 表 key "ModelCost" 加载覆盖。
var modelCostMap = types.NewRWMap[string, ModelCostInfo]()

func GetModelCostMap() map[string]ModelCostInfo {
	return modelCostMap.ReadAll()
}

func ModelCost2JSONString() string {
	return modelCostMap.MarshalJSONString()
}

func UpdateModelCostByJSONString(jsonStr string) error {
	return types.LoadFromJsonStringWithCallback(modelCostMap, jsonStr, InvalidateExposedDataCache)
}

// GetModelCost 返回模型的成本价。第二返回值表示是否配置了该模型的成本(未配置则利润无法计算, 该笔按成本0或跳过)。
func GetModelCost(name string) (ModelCostInfo, bool) {
	name = FormatMatchingModelName(name)
	cost, ok := modelCostMap.Get(name)
	return cost, ok
}
