package ratio_setting

import "github.com/QuantumNous/new-api/types"

// ModelPricingSource 模型定价的「官方价×倍率」编辑态来源, 与计费完全分离。
// 仅用于前端 UI 还原(超管填的官方价 + 销售/成本倍率), 计费引擎不读它——
// 计费仍走 ModelRatio/ModelPrice/ModelCost。提交时前端换算后写回:
//   ModelRatio = official_input × sale_multiplier / 2
//   CompletionRatio = official_output / official_input
//   CacheRatio(读) = official_cache_read / official_input
//   CreateCacheRatio(写) = official_cache_write / official_input
//   ModelCost = { input: official_input×cost_multiplier, output: official_output×cost_multiplier, cache: official_cache_read×cost_multiplier }
// 存本 option 是为解决反向还原不唯一(从 ratio 反推不出原始官方价/倍率)。
type ModelPricingSource struct {
	OfficialInput      float64 `json:"official_input"`       // 官方价 输入 $/1M
	OfficialOutput     float64 `json:"official_output"`      // 官方价 输出 $/1M
	OfficialCacheRead  float64 `json:"official_cache_read"`  // 官方价 缓存读 $/1M
	OfficialCacheWrite   float64 `json:"official_cache_write"`   // 官方价 缓存写 $/1M
	OfficialRequestPrice float64 `json:"official_request_price"` // per-request 官方价 $/request
	OfficialExpr         string  `json:"official_expr"`          // tiered_expr 官方分段表达式(售价侧)
	SaleMultiplier       float64 `json:"sale_multiplier"`        // 销售倍率(统一乘各项官方价 = 售价)
	CostMultiplier     float64 `json:"cost_multiplier"`      // 成本倍率(统一乘各项官方价 = 成本)
}

// pricingSourceMap 启动时空(无默认值), 由 options 表 key "ModelPricingSource" 加载覆盖。
var pricingSourceMap = types.NewRWMap[string, ModelPricingSource]()

func GetModelPricingSourceMap() map[string]ModelPricingSource {
	return pricingSourceMap.ReadAll()
}

// PublicPricingSource 是对外公开的视图结构(模型广场 /api/pricing), 绝不含 CostMultiplier。
// 权限红线: 成本倍率是利润核心, 泄露即暴露成本结构, 此结构是唯一安全闸门。
type PublicPricingSource struct {
	OfficialInput        float64 `json:"official_input"`
	OfficialOutput       float64 `json:"official_output"`
	OfficialCacheRead    float64 `json:"official_cache_read"`
	OfficialCacheWrite   float64 `json:"official_cache_write"`
	OfficialRequestPrice float64 `json:"official_request_price"`
	OfficialExpr         string  `json:"official_expr"`
	SaleMultiplier       float64 `json:"sale_multiplier"`
}

// GetPublicModelPricingSourceMap 返回对外公开的视图(白名单, 显式 omit cost_multiplier)。
func GetPublicModelPricingSourceMap() map[string]PublicPricingSource {
	all := pricingSourceMap.ReadAll()
	public := make(map[string]PublicPricingSource, len(all))
	for name, src := range all {
		public[name] = PublicPricingSource{
			OfficialInput:        src.OfficialInput,
			OfficialOutput:       src.OfficialOutput,
			OfficialCacheRead:    src.OfficialCacheRead,
			OfficialCacheWrite:   src.OfficialCacheWrite,
			OfficialRequestPrice: src.OfficialRequestPrice,
			OfficialExpr:         src.OfficialExpr,
			SaleMultiplier:       src.SaleMultiplier,
		}
	}
	return public
}

func ModelPricingSource2JSONString() string {
	return pricingSourceMap.MarshalJSONString()
}

func UpdateModelPricingSourceByJSONString(jsonStr string) error {
	return types.LoadFromJsonStringWithCallback(pricingSourceMap, jsonStr, InvalidateExposedDataCache)
}
